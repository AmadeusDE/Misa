//go:build !unix && !windows

package builtins

import (
	"errors"
	"os"
	"time"
)

// touchChtimes follows symlinks (os.Chtimes) and cannot stamp standard output.
func touchChtimes(ctx *Context, name, path string, atime, mtime time.Time, noFollow bool, specifyTimes bool) error {
	if name == "-" || noFollow {
		return errors.ErrUnsupported
	}
	return os.Chtimes(path, atime, mtime)
}
