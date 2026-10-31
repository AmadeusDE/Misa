//go:build windows

package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsReservedRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := TryLock(f); err != nil {
		t.Fatal(err)
	}
	defer Unlock(f)
	g, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	// The ordinary start of the file is not part of the protocol lock.
	var start windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(g.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &start); err != nil {
		t.Fatal(err)
	}
	if err := windows.UnlockFileEx(windows.Handle(g.Fd()), 0, 1, 0, &start); err != nil {
		t.Fatal(err)
	}
	reserved := windows.Overlapped{OffsetHigh: 1 << 30}
	err = windows.LockFileEx(windows.Handle(g.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &reserved)
	if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		if err == nil {
			_ = windows.UnlockFileEx(windows.Handle(g.Fd()), 0, 1, 0, &reserved)
		}
		t.Fatalf("reserved range: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsOpenNonInheritable(t *testing.T) {
	f, err := Open(filepath.Join(t.TempDir(), "lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var flags uint32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation")
	ok, _, callErr := proc.Call(f.Fd(), uintptr(unsafe.Pointer(&flags)))
	if ok == 0 {
		t.Fatal(callErr)
	}
	if flags&windows.HANDLE_FLAG_INHERIT != 0 {
		t.Fatalf("inheritable handle: %#x", flags)
	}
}

func TestWindowsRejectsOpenedReparsePoint(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "target"), filepath.Join(dir, "link")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	_ = windows.CloseHandle(h)
	if err != nil {
		t.Fatal(err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		t.Fatal("native handle did not open reparse point")
	}
	f, err := Open(link)
	if err == nil {
		f.Close()
		t.Fatal("Open accepted reparse point")
	}
}
