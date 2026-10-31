//go:build !(linux || aix || solaris)

package builtins

import "os"

// fdatasync falls back to a full fsync: Go exposes fdatasync(2) only on Linux,
// AIX and Solaris, so darwin, the BSDs and Windows sync metadata too. Toybox's
// `fsync -d` calls fdatasync(2) wherever libc provides it.
func fdatasync(file *os.File) error {
	return file.Sync()
}
