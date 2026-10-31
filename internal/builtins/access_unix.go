//go:build unix

package builtins

import "golang.org/x/sys/unix"

// canSearch reports whether the process may traverse the directory path.
func canSearch(path string) bool { return unix.Access(path, unix.X_OK) == nil }

// canWrite reports whether the process may write path.
func canWrite(path string) bool { return unix.Access(path, unix.W_OK) == nil }
