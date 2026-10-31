//go:build unix && !linux

package native

import "golang.org/x/sys/unix"

// targetNice returns pid's nice value, or -1 when it cannot be read.
func targetNice(pid int) int {
	n, err := unix.Getpriority(unix.PRIO_PROCESS, pid)
	if err != nil {
		return -1
	}
	return n
}
