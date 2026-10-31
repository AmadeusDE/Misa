//go:build unix && !linux

package proceffect

import "syscall"

func niceness() (int, error) { return syscall.Getpriority(syscall.PRIO_PROCESS, 0) }
func umask() (int, error)    { return umaskChild() }
