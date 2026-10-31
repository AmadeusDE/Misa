//go:build linux || darwin || freebsd || dragonfly || openbsd || netbsd || solaris

// Changing the flags of an already open standard stream, which dd needs for
// iflag=/oflag= given with input or output redirected from the shell.
package builtins

import (
	"os"
	"syscall"
)

// ddFcntlSetFlags adds bits to the status flags of an open descriptor.
func ddFcntlSetFlags(file *os.File, bits int) error {
	fd := file.Fd()
	old, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 {
		return errno
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFL, old|uintptr(bits)); errno != 0 {
		return errno
	}
	return nil
}

// ddClearStreamFlag removes bits from the status flags of an open file and
// returns a function that adds them back.
func ddClearStreamFlag(file *os.File, bits int) (func(), bool) {
	fd := file.Fd()
	old, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 {
		return nil, false
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFL, old&^uintptr(bits)); errno != 0 {
		return nil, false
	}
	return func() {
		syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFL, old)
	}, true
}
