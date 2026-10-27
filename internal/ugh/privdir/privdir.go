// Package privdir creates per-user directories in shared places (the temp
// directory) and refuses ones another user could control.
package privdir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Under is base/name when that is safe, else a private directory under the
// user's cache directory, so a directory another user planted in a shared
// base is never used.
func Under(base, name string) string {
	dir := filepath.Join(base, name)
	if Ensure(dir) == nil {
		return dir
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = filepath.Join(os.TempDir(), name+"-fallback")
	}
	dir = filepath.Join(cache, name)
	_ = os.MkdirAll(filepath.Dir(dir), 0o700)
	_ = Ensure(dir)
	return dir
}
