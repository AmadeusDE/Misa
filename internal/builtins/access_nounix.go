//go:build !unix

package builtins

import "os"

// canSearch: Windows has no search bit; a directory that can be read can be
// traversed.
func canSearch(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// canWrite reports whether path exists without the read-only attribute, the
// only write protection Go can see on Windows.
func canWrite(path string) bool {
	info, err := os.Stat(path)
	return err == nil && (info.IsDir() || info.Mode().Perm()&0200 != 0)
}
