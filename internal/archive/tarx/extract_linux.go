//go:build linux

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"golang.org/x/sys/unix"
	"os"
	"time"
)

func makeSpecial(p *os.Root, n string, h *tar.Header) error {
	d, e := p.Open(".")
	if e != nil {
		return e
	}
	defer d.Close()
	mode := uint32(0600)
	switch h.Typeflag {
	case tar.TypeFifo:
		mode |= unix.S_IFIFO
	case tar.TypeChar:
		mode |= unix.S_IFCHR
	case tar.TypeBlock:
		mode |= unix.S_IFBLK
	}
	return unix.Mknodat(int(d.Fd()), n, mode, int(unix.Mkdev(uint32(h.Devmajor), uint32(h.Devminor))))
}

func (x *extractor) specialMetadata(p *os.Root, n string, h *tar.Header) error {
	d, err := p.Open(".")
	if err != nil {
		return err
	}
	defer d.Close()
	fd := int(d.Fd())
	if !x.opt.NoSameOwner && os.Geteuid() == 0 {
		u, g := x.ids(h)
		if err := unix.Fchownat(fd, n, u, g, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return err
		}
	}
	mode := uint32(h.Mode) & 0777
	if x.opt.SamePermissions {
		mode = uint32(h.Mode) & 07777
	} else {
		mode &^= uint32(x.opt.Umask)
	}
	if err := unix.Fchmodat(fd, n, mode, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if !x.opt.Touch && !h.ModTime.IsZero() {
		at := h.AccessTime
		if at.IsZero() {
			at = time.Now()
		}
		ts := []unix.Timespec{unix.NsecToTimespec(at.UnixNano()), unix.NsecToTimespec(h.ModTime.UnixNano())}
		return unix.UtimesNanoAt(fd, n, ts, unix.AT_SYMLINK_NOFOLLOW)
	}
	return nil
}
