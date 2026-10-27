//go:build freebsd || darwin || netbsd

package lscore

import (
	"syscall"
	"time"
)

func statTimes(sys *syscall.Stat_t) (access, change time.Time) {
	return time.Unix(sys.Atimespec.Unix()), time.Unix(sys.Ctimespec.Unix())
}
