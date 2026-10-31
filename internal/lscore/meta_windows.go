//go:build windows

package lscore

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// readMeta reads what Windows has: link count, volume serial and file index
// from an open handle, access and creation times from the directory entry.
// Owner, group, block count and change time stay unavailable.
func readMeta(path string, info os.FileInfo, lstat bool) sysMeta {
	m := sysMeta{Missing: FieldOwner | FieldGroup | FieldBlocks | FieldChangeTime}
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		m.Access = time.Unix(0, d.LastAccessTime.Nanoseconds())
		m.Birth = time.Unix(0, d.CreationTime.Nanoseconds())
	} else {
		m.Missing |= FieldAccessTime | FieldBirthTime
	}
	id, ok := fileID(path, !lstat)
	if !ok {
		m.Missing |= FieldNlink | FieldInode | FieldDev
		return m
	}
	m.Nlink, m.Inode, m.Dev = id.nlink, id.index, id.volume
	return m
}

type winID struct {
	volume, index, nlink uint64
}

func fileID(path string, follow bool) (winID, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return winID{}, false
	}
	flags := uint32(windows.FILE_FLAG_BACKUP_SEMANTICS)
	if !follow {
		flags |= windows.FILE_FLAG_OPEN_REPARSE_POINT
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	h, err := windows.CreateFile(p, 0, share, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return winID{}, false
	}
	defer windows.CloseHandle(h)
	var bhfi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &bhfi); err != nil {
		return winID{}, false
	}
	return winID{
		volume: uint64(bhfi.VolumeSerialNumber),
		index:  uint64(bhfi.FileIndexHigh)<<32 | uint64(bhfi.FileIndexLow),
		nlink:  uint64(bhfi.NumberOfLinks),
	}, true
}

// Identify returns the (volume serial, file index) pair of path, dereferencing links.
func Identify(path string) (DirIdent, bool) {
	id, ok := fileID(path, true)
	if !ok {
		return DirIdent{}, false
	}
	return DirIdent{Dev: id.volume, Ino: id.index}, true
}
