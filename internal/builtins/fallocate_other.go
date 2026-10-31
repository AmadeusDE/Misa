//go:build !linux

package builtins

import "os"

// fallocateSupported is false here so the command fails before creating a file:
// Misa never fakes allocation by writing zeros. Darwin's F_PREALLOCATE and the
// BSDs' posix_fallocate(2) have no Go binding.
const fallocateSupported = false

func fallocateFile(file *os.File, offset, length int64) error {
	return errFallocateUnsupported
}
