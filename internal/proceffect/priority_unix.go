//go:build unix && !linux

package proceffect

import "syscall"

func niceness() (int, error) { return priority(syscall.PRIO_PROCESS, 0) }
func priority(which, id int) (int, error) {
	return syscall.Getpriority(which, id)
}
func umask() (int, error) { return umaskChild() }
