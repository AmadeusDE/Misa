//go:build !darwin && !windows

package builtins

import "golang.org/x/sys/unix"

const utimeNow = unix.UTIME_NOW
