//go:build !linux

package builtins

import "syscall"

func renameExchange(src, dst string) error {
	return syscall.ENOSYS
}
