//go:build unix

package builtins

import (
	"os"
	"syscall"
)

// readFSMeta reads the stat fields behind info; path is only needed by
// providers that must reopen the file.
func readFSMeta(path string, info os.FileInfo) (*fsMeta, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, false
	}
	a, m, c := rawAtim(st), rawMtim(st), rawCtim(st)
	return &fsMeta{
		Dev: uint64(st.Dev), Ino: st.Ino, Nlink: uint64(st.Nlink),
		Blocks: st.Blocks, Blksize: int64(st.Blksize),
		Uid: st.Uid, Gid: st.Gid, Mode: uint32(st.Mode),
		Atime: fsTimespec{int64(a.Sec), int64(a.Nsec)},
		Mtime: fsTimespec{int64(m.Sec), int64(m.Nsec)},
		Ctime: fsTimespec{int64(c.Sec), int64(c.Nsec)},
	}, true
}

// ownerOf reports the owner recorded in info.
func ownerOf(info os.FileInfo) (uid, gid int, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return int(st.Uid), int(st.Gid), true
}

// sameFileInfo reports whether a and b are the same inode.
func sameFileInfo(a, b os.FileInfo) bool {
	sa, oka := a.Sys().(*syscall.Stat_t)
	sb, okb := b.Sys().(*syscall.Stat_t)
	return oka && okb && sa.Ino != 0 && sa.Ino == sb.Ino
}

func makeFifo(path string, mode uint32) error { return syscall.Mkfifo(path, mode) }
