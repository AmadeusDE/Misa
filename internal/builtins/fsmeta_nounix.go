//go:build !unix

package builtins

import (
	"errors"
	"os"
)

// makeFifo: Windows has no named-pipe nodes in the file system.
func makeFifo(path string, mode uint32) error {
	return &os.PathError{Op: "mkfifo", Path: path, Err: errors.ErrUnsupported}
}
