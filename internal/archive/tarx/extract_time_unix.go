//go:build unix

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"golang.org/x/sys/unix"
	"os"
	"time"
)

func fileTimes(f *os.File, at, mt time.Time) error {
	return unix.Futimes(int(f.Fd()), []unix.Timeval{unix.NsecToTimeval(at.UnixNano()), unix.NsecToTimeval(mt.UnixNano())})
}

func symlinkTimes(p *os.Root, name string, h *tar.Header) error {
	d, err := p.Open(".")
	if err != nil {
		return err
	}
	defer d.Close()
	at := h.AccessTime
	if at.IsZero() {
		at = time.Now()
	}
	ts := []unix.Timespec{unix.NsecToTimespec(at.UnixNano()), unix.NsecToTimespec(h.ModTime.UnixNano())}
	return unix.UtimesNanoAt(int(d.Fd()), name, ts, unix.AT_SYMLINK_NOFOLLOW)
}
