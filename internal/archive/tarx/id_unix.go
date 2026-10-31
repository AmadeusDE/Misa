//go:build unix

package tarx

import (
	"io/fs"
	"syscall"
)

type fileID struct{ dev, ino uint64 }

func idOf(info fs.FileInfo) (fileID, bool) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Nlink < 2 {
		return fileID{}, false
	}
	return fileID{uint64(s.Dev), uint64(s.Ino)}, true
}
