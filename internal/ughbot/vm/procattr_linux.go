package vm

import "syscall"

// qemuProcAttr puts qemu in its own group and kills it if the daemon dies.
func qemuProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
