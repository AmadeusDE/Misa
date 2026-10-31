//go:build unix && !aix

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// Open opens or creates path without following its final component.
func Open(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

// TryLock attempts an exclusive lock without waiting.
func TryLock(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return ErrLocked
	}
	return err
}

// Unlock releases the lock. Repeated calls are safe; the caller closes f.
func Unlock(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
