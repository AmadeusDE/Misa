//go:build !unix && !windows

package builtins

import (
	"os"

	"github.com/AmadeusDE/Misa/internal/lscore"
)

func readFSMeta(path string, info os.FileInfo) (*fsMeta, bool) {
	mt := info.ModTime()
	return &fsMeta{
		Mode: posixMode(info.Mode()), Mtime: fsTimespec{mt.Unix(), int64(mt.Nanosecond())},
		Missing: lscore.FieldOwner | lscore.FieldGroup | lscore.FieldNlink | lscore.FieldInode | lscore.FieldDev |
			lscore.FieldBlocks | lscore.FieldAccessTime | lscore.FieldChangeTime | lscore.FieldBirthTime,
	}, true
}

func ownerOf(info os.FileInfo) (uid, gid int, ok bool) { return 0, 0, false }

func sameFileInfo(a, b os.FileInfo) bool { return os.SameFile(a, b) }
