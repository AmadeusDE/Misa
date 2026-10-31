//go:build unix

package builtins

import (
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func touchChtimes(ctx *Context, name, path string, atime, mtime time.Time, noFollow bool, specifyTimes bool) error {
	var ts []unix.Timespec
	if specifyTimes {
		ts = []unix.Timespec{
			unix.NsecToTimespec(atime.UnixNano()),
			unix.NsecToTimespec(mtime.UnixNano()),
		}
	} else {
		ts = []unix.Timespec{
			{Sec: 0, Nsec: utimeNow},
			{Sec: 0, Nsec: utimeNow},
		}
	}

	if name == "-" {
		return unix.UtimesNanoAt(1, "", ts, 0)
	}

	flags := 0
	if noFollow {
		flags = unix.AT_SYMLINK_NOFOLLOW
	}

	err := unix.UtimesNanoAt(unix.AT_FDCWD, path, ts, flags)
	if err == nil {
		return nil
	}

	if (err == syscall.EACCES || err == syscall.EPERM) && !specifyTimes {
		fd, errOpen := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
		if errOpen == nil {
			defer unix.Close(fd)
			errFutimens := unix.UtimesNanoAt(fd, "", ts, 0)
			if errFutimens == nil {
				return nil
			}
		}
	}
	return err
}
