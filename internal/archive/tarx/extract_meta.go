// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"context"
	"os"
	"os/user"
	"sort"
	"strconv"
	"time"
)

func (x *extractor) ids(h *tar.Header) (int, int) {
	uid, gid := h.Uid, h.Gid
	if !x.opt.NumericOwner {
		if h.Uname != "" {
			if u, e := user.Lookup(h.Uname); e == nil {
				if n, e := strconv.Atoi(u.Uid); e == nil {
					uid = n
				}
			}
		}
		if h.Gname != "" {
			if g, e := user.LookupGroup(h.Gname); e == nil {
				if n, e := strconv.Atoi(g.Gid); e == nil {
					gid = n
				}
			}
		}
	}
	if x.opt.Owner != nil {
		uid = *x.opt.Owner
	}
	if x.opt.Group != nil {
		gid = *x.opt.Group
	}
	return uid, gid
}
func (x *extractor) symlinkOwner(p *os.Root, n string, h *tar.Header) error {
	if x.opt.NoSameOwner || os.Geteuid() != 0 {
		return nil
	}
	u, g := x.ids(h)
	return p.Lchown(n, u, g)
}
func (x *extractor) metadata(f *os.File, h *tar.Header) error {
	if !x.opt.NoSameOwner && os.Geteuid() == 0 {
		u, g := x.ids(h)
		if err := f.Chown(u, g); err != nil {
			return err
		}
	}
	mode := x.mode(h)
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if !x.opt.Touch && !h.ModTime.IsZero() {
		at := h.AccessTime
		if at.IsZero() {
			at = time.Now()
		}
		return fileTimes(f, at, h.ModTime)
	}
	return nil
}
func (x *extractor) mode(h *tar.Header) os.FileMode {
	mode := os.FileMode(h.Mode) & 0777
	if x.opt.SamePermissions {
		if h.Mode&04000 != 0 {
			mode |= os.ModeSetuid
		}
		if h.Mode&02000 != 0 {
			mode |= os.ModeSetgid
		}
		if h.Mode&01000 != 0 {
			mode |= os.ModeSticky
		}
	} else {
		mode &^= x.opt.Umask
	}
	return mode
}
func (x *extractor) finishDirs(ctx context.Context) {
	sort.SliceStable(x.dirs, func(i, j int) bool { return x.dirs[i].depth > x.dirs[j].depth })
	for _, d := range x.dirs {
		if ctx.Err() != nil {
			return
		}
		x.finishDir(d)
	}
}

// Reopen each directory without following symlink parents. The identity check
// prevents an archive entry or concurrent rename from redirecting metadata.
func (x *extractor) finishDir(d dirMetadata) {
	p, base, err := x.parent(d.path, false)
	if err != nil {
		if !os.IsNotExist(err) {
			x.report(d.name, err)
		}
		return
	}
	defer p.Close()
	info, err := p.Lstat(base)
	if err != nil {
		if !os.IsNotExist(err) {
			x.report(d.name, err)
		}
		return
	}
	if !info.IsDir() || !os.SameFile(info, d.info) {
		return
	}
	root, err := p.OpenRoot(base)
	if err != nil {
		x.report(d.name, err)
		return
	}
	defer root.Close()
	f, err := root.Open(".")
	if err != nil {
		x.report(d.name, err)
		return
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		x.report(d.name, err)
		return
	}
	if !os.SameFile(opened, d.info) {
		return
	}
	if err := x.metadata(f, &d.h); err != nil {
		x.report(d.name, err)
	}
}
