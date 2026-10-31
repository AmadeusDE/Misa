//go:build unix

package builtins

import (
	"io"
	"syscall"
)

func statFileSystem(w io.Writer, name, path string) error {
	var stfs syscall.Statfs_t
	if err := syscall.Statfs(path, &stfs); err != nil {
		return err
	}
	printFSInfo(w, name, &stfs)
	return nil
}
