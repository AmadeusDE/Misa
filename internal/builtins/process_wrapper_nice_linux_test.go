//go:build linux

package builtins

import "golang.org/x/sys/unix"

// niceOf returns pid's nice value, or -1 when it cannot be read. Linux reports
// 20-nice through the raw getpriority syscall.
func niceOf(pid int) int {
	n, err := unix.Getpriority(unix.PRIO_PROCESS, pid)
	if err != nil {
		return -1
	}
	return 20 - n
}
