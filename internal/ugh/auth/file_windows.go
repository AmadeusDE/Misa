//go:build windows

package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	"github.com/AmadeusDE/Misa/internal/ugh/privdir"
)

// On Windows the credentials live in a protected file inside a directory
// created with a user-only protected DACL (see internal/ugh/privdir).
// Credential Manager is not used: its 2560-byte blob cap is smaller than the
// document (docs/plans/windows.md, A02).

// checkDir creates missing ancestors normally, then creates the application
// directory with a private descriptor (or validates the existing one).
func checkDir(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("auth: absolute clean store path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("auth: create directory failed")
	}
	if err := privdir.Ensure(path); err != nil {
		return errors.New("auth: application directory must be private (user-only protected ACL) and owned by current user")
	}
	return nil
}

// checkLoadDir refuses an existing credentials directory that is not private.
// A missing directory is fine: the store is simply empty.
func checkLoadDir(dir string) error {
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return errors.New("auth: credentials directory cannot be inspected")
	}
	if err := privdir.Verify(dir, true); err != nil {
		return errors.New("auth: credentials directory must be private (user-only protected ACL) and owned by current user")
	}
	return nil
}

// openCredentials opens the file for reading: os.ErrNotExist when absent,
// otherwise only a regular, private, owned file of acceptable size that is not
// a reparse point.
func openCredentials(path string) (*os.File, error) {
	file, err := privdir.OpenVerified(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, errors.New("auth: credentials file is unsafe (must be regular, owned, user-only protected ACL)")
	}
	if st, e := file.Stat(); e != nil || st.Size() > maxFile {
		_ = file.Close()
		return nil, errors.New("auth: credentials file is unsafe (must be regular, owned, user-only protected ACL)")
	}
	return file, nil
}

// stageFile creates the temporary file the new document is written to, with
// a private descriptor from the start.
func stageFile(dir string) (*os.File, error) {
	for range 8 {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, err
		}
		f, err := privdir.CreatePrivateFile(filepath.Join(dir, ".credentials-"+hex.EncodeToString(b[:])))
		if err == nil || !errors.Is(err, os.ErrExist) {
			return f, err
		}
	}
	return nil, os.ErrExist
}

// replaceFile atomically replaces dst with tmp and waits for the move to reach
// the disk (MoveFileEx replaces in one step and the data was synced before).
func replaceFile(tmp, dst string) error {
	from, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return errors.New("auth: replacement failed")
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return errors.New("auth: replacement failed")
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return errors.New("auth: replacement failed")
	}
	return nil
}
