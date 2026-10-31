//go:build unix

package walk

import (
	"io/fs"
	"syscall"
)

type fileID struct{ dev, ino uint64 }

func idOf(info fs.FileInfo) (fileID, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, false
	}
	return fileID{uint64(st.Dev), uint64(st.Ino)}, true
}

func sameDevice(a, b fileID) bool { return a.dev == b.dev }
