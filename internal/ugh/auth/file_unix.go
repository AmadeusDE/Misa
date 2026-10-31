//go:build unix

package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// checkDir creates missing path components without following symlinks. The
// application directory must be owned by this user and exactly 0700.
func checkDir(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("auth: absolute clean store path required")
	}
	cur := string(os.PathSeparator)
	for _, part := range strings.Split(strings.TrimPrefix(path, cur), cur) {
		cur = filepath.Join(cur, part)
		st, e := os.Lstat(cur)
		if errors.Is(e, os.ErrNotExist) {
			if e = os.Mkdir(cur, 0700); e != nil && !errors.Is(e, os.ErrExist) {
				return errors.New("auth: create directory failed")
			}
			st, e = os.Lstat(cur)
		}
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("auth: unsafe directory component")
		}
	}
	st, e := os.Lstat(path)
	if e != nil || st.Mode().Perm() != 0700 || !owned(st) {
		return errors.New("auth: application directory must be private (0700) and owned by current user")
	}
	return nil
}

func owned(st os.FileInfo) bool {
	sys, ok := st.Sys().(*syscall.Stat_t)
	return ok && int(sys.Uid) == os.Getuid()
}

// checkLoadDir refuses an existing credentials directory that is not private.
// A missing directory is fine: the store is simply empty.
func checkLoadDir(dir string) error {
	st, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.New("auth: credentials directory cannot be inspected")
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm() != 0700 || !owned(st) {
		return errors.New("auth: credentials directory must be private (0700) and owned by current user")
	}
	return nil
}

// openCredentials opens the file for reading: os.ErrNotExist when absent,
// otherwise only a regular, private, owned file of acceptable size.
func openCredentials(path string) (*os.File, error) {
	// O_NOFOLLOW refuses symlink aliases; O_NONBLOCK avoids FIFO hangs.
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if e == syscall.ENOENT {
		return nil, os.ErrNotExist
	}
	if e != nil {
		return nil, errors.New("auth: credentials cannot be opened safely")
	}
	file := os.NewFile(uintptr(fd), path)
	st, e := file.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0o077 != 0 || !owned(st) || st.Size() > maxFile {
		_ = file.Close()
		return nil, errors.New("auth: credentials file is unsafe (must be 0600, regular, owned, ungrouped/unworlded)")
	}
	return file, nil
}

// stageFile creates the temporary file the new document is written to.
func stageFile(dir string) (*os.File, error) {
	tmp, e := os.CreateTemp(dir, ".credentials-*")
	if e != nil {
		return nil, e
	}
	if e = tmp.Chmod(0600); e != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, e
	}
	return tmp, nil
}

// replaceFile atomically renames tmp over dst and syncs the directory.
func replaceFile(tmp, dst string) error {
	if e := os.Rename(tmp, dst); e != nil {
		return errors.New("auth: replacement failed")
	}
	df, e := os.Open(filepath.Dir(dst))
	if e != nil {
		return errors.New("auth: directory sync failed; replacement outcome uncertain")
	}
	e = df.Sync()
	_ = df.Close()
	if e != nil {
		return errors.New("auth: directory sync failed; replacement outcome uncertain")
	}
	return nil
}
