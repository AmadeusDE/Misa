//go:build linux

package builtins

import "golang.org/x/sys/unix"

func renameExchange(src, dst string) error {
	return unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_EXCHANGE)
}
