//go:build linux

package builtins

import (
	"os"

	"golang.org/x/sys/unix"
)

// fallocateSupported reports that fallocate(2) is available for this build.
const fallocateSupported = true

// fallocateFile reserves space with fallocate(2), which grows the file.
func fallocateFile(file *os.File, offset, length int64) error {
	return unix.Fallocate(int(file.Fd()), 0, offset, length)
}
