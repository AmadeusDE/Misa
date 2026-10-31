//go:build linux || aix || solaris

package builtins

import (
	"os"

	"golang.org/x/sys/unix"
)

// fdatasync flushes file contents without forcing a metadata update. Go only
// binds fdatasync(2) on these platforms; see fsync_nodatasync.go.
func fdatasync(file *os.File) error {
	return unix.Fdatasync(int(file.Fd()))
}
