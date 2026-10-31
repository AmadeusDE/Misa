//go:build !windows

package privdir

import (
	"errors"
	"fmt"
	"os"
)

// Ensure makes dir with mode 0700 unless it exists, then checks that it is a
// real directory (not a symlink), owned by this user and closed to others.
func Ensure(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s: not a directory", dir)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s: open to other users (mode %o)", dir, fi.Mode().Perm())
	}
	if !ownedByMe(fi) {
		return fmt.Errorf("%s: owned by another user", dir)
	}
	return nil
}
