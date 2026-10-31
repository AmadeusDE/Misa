//go:build windows

package builtins

import (
	"os"

	"github.com/AmadeusDE/Misa/internal/lscore"
)

// readFSMeta reads what Windows has through lscore: link count, volume serial
// and file index, access and creation time. Owner, group, block counts and
// change time stay missing.
func readFSMeta(path string, info os.FileInfo) (*fsMeta, bool) {
	fi := lscore.StatPath(path, info.Mode()&os.ModeSymlink == 0)
	if fi.Err != nil {
		return nil, false
	}
	mt := info.ModTime()
	return &fsMeta{
		Dev: fi.Dev, Ino: fi.Inode, Nlink: fi.Nlink,
		Mode:    posixMode(info.Mode()),
		Atime:   fsTimespec{fi.AccessTime.Unix(), int64(fi.AccessTime.Nanosecond())},
		Mtime:   fsTimespec{mt.Unix(), int64(mt.Nanosecond())},
		Missing: fi.Missing | lscore.FieldOwner | lscore.FieldGroup | lscore.FieldBlocks | lscore.FieldChangeTime,
	}, true
}

// ownerOf reports no owner: Windows has no uid or gid.
func ownerOf(info os.FileInfo) (uid, gid int, ok bool) { return 0, 0, false }

func sameFileInfo(a, b os.FileInfo) bool { return os.SameFile(a, b) }
