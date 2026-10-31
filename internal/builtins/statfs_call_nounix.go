//go:build !unix

package builtins

import (
	"errors"
	"io"
)

// statFileSystem: stat -f prints statfs(2) fields (type, id, name length,
// inode counts) that Windows cannot report; see support.go.
func statFileSystem(w io.Writer, name, path string) error {
	if l, ok := platformLimit("stat", "-f"); ok {
		return errors.New(l.Reason)
	}
	return errors.ErrUnsupported
}
