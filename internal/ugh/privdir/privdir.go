// Package privdir creates per-user directories in shared places (the temp
// directory) and refuses ones another user could control. On Unix a private
// directory is mode 0700 and owned by the user; on Windows it is created with
// a protected DACL that grants only the user's SID, and validated by handle
// (owner, ACEs, no reparse point).
package privdir

import (
	"os"
	"path/filepath"
)

// Under is base/name when that is safe, else a private directory under the
// user's cache directory, so a directory another user planted in a shared
// base is never used. It returns an error, never an unvalidated path, when
// neither location can be made private.
func Under(base, name string) (string, error) {
	dir := filepath.Join(base, name)
	if Ensure(dir) == nil {
		return dir, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = filepath.Join(os.TempDir(), name+"-fallback")
	}
	dir = filepath.Join(cache, name)
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return "", err
	}
	if err := Ensure(dir); err != nil {
		return "", err
	}
	return dir, nil
}
