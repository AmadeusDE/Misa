//go:build windows

package builtins

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// touchChtimes sets both times through SetFileTime on a handle, which keeps
// the 100 ns FILETIME precision. noFollow opens the reparse point itself
// (FILE_FLAG_OPEN_REPARSE_POINT) so touch -h stamps a symlink, not its target.
// The caller already chose both times (now, a stamp, or for an unselected one
// the file's current value), so neither is clobbered; specifyTimes only
// matters to the Unix UTIME_NOW fallback.
func touchChtimes(ctx *Context, name, path string, atime, mtime time.Time, noFollow bool, specifyTimes bool) error {
	var h windows.Handle
	if name == "-" {
		h = windows.Handle(os.Stdout.Fd())
	} else {
		p, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		flags := uint32(windows.FILE_FLAG_BACKUP_SEMANTICS)
		if noFollow {
			flags |= windows.FILE_FLAG_OPEN_REPARSE_POINT
		}
		share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
		h, err = windows.CreateFile(p, windows.FILE_WRITE_ATTRIBUTES, share, nil, windows.OPEN_EXISTING, flags, 0)
		if err != nil {
			return &os.PathError{Op: "touch", Path: path, Err: err}
		}
		defer windows.CloseHandle(h)
	}
	a := windows.NsecToFiletime(atime.UnixNano())
	m := windows.NsecToFiletime(mtime.UnixNano())
	if err := windows.SetFileTime(h, nil, &a, &m); err != nil {
		return &os.PathError{Op: "touch", Path: path, Err: err}
	}
	return nil
}
