//go:build windows

package filelock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// Open opens or creates path, rejecting a final-component reparse point.
// See https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew
// and https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfileinformationbyhandle.
func Open(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		err = windows.ERROR_CANT_ACCESS_FILE
	}
	if err != nil {
		windows.CloseHandle(h)
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}

// The single reserved byte is far beyond ordinary EOF so reads and writes of
// diagnostic content at the start of the file remain unaffected by the lock.
// See https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
// and https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex.
func lockOffset() windows.Overlapped { return windows.Overlapped{OffsetHigh: 1 << 30} }

// TryLock attempts an exclusive lock without waiting.
func TryLock(f *os.File) error {
	offset := lockOffset()
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &offset)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrLocked
	}
	return err
}

// Unlock releases the lock. Repeated calls are safe; the caller closes f.
func Unlock(f *os.File) error {
	offset := lockOffset()
	err := windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &offset)
	if errors.Is(err, windows.ERROR_NOT_LOCKED) {
		return nil
	}
	return err
}
