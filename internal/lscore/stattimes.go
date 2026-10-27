//go:build linux || openbsd

package lscore

import (
	"syscall"
	"time"
)

func statTimes(sys *syscall.Stat_t) (access, change time.Time) {
	return time.Unix(sys.Atim.Unix()), time.Unix(sys.Ctim.Unix())
}
