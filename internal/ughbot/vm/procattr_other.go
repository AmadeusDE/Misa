//go:build unix && !linux

package vm

import "syscall"

// qemuProcAttr puts qemu in its own group; only Linux can tie its life to the daemon's.
func qemuProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
