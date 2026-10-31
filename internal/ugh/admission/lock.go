package admission

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AmadeusDE/Misa/internal/filelock"
)

// openLockFile opens a non-inheritable lock file without following symlinks.
func openLockFile(path string) (*os.File, error) {
	f, err := filelock.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open lock file %s: %v", ErrStorageUnavailable, filepath.Base(path), err)
	}
	return f, nil
}

func tryLockEx(f *os.File) error   { return filelock.TryLock(f) }
func unlock(f *os.File) error      { return filelock.Unlock(f) }
func closeFd(f *os.File) error     { return f.Close() }
func isLockedError(err error) bool { return errors.Is(err, filelock.ErrLocked) }

// lockAdmin acquires the brief serialization lock for an account.
func lockAdmin(ctx context.Context, accountDir string) (func(), error) {
	f, err := openLockFile(filepath.Join(accountDir, "admin.lock"))
	if err != nil {
		return nil, err
	}
	if err := filelock.Lock(ctx, f); err != nil {
		_ = f.Close()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: admin lock failed: %v", ErrStorageUnavailable, err)
	}
	return func() { _ = unlock(f); _ = f.Close() }, nil
}
