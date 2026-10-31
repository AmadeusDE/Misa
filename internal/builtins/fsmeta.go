package builtins

import (
	"os"

	"github.com/AmadeusDE/Misa/internal/lscore"
)

type fsTimespec struct{ Sec, Nsec int64 }

// fsMeta is the file metadata the builtins read beyond os.FileInfo. Missing
// names what the platform cannot report (the lscore convention): those fields
// hold zero and callers print "?" instead of the zero. Blksize shares
// lscore.FieldBlocks.
type fsMeta struct {
	Dev, Ino, Nlink uint64
	Blocks, Blksize int64
	Uid, Gid        uint32
	Mode            uint32
	Atime, Mtime    fsTimespec
	Ctime           fsTimespec
	Missing         lscore.Fields
}

func (m *fsMeta) lacks(f lscore.Fields) bool { return m.Missing&f != 0 }

func atim(m *fsMeta) fsTimespec { return m.Atime }
func mtim(m *fsMeta) fsTimespec { return m.Mtime }
func ctim(m *fsMeta) fsTimespec { return m.Ctime }

// inodeKeyOf identifies path (described by info) by device and inode; the
// boolean is false when the platform has no file identity.
func inodeKeyOf(path string, info os.FileInfo) (inodeKey, bool) {
	m, ok := readFSMeta(path, info)
	if !ok || m.lacks(lscore.FieldDev|lscore.FieldInode) {
		return inodeKey{}, false
	}
	return inodeKey{m.Dev, m.Ino}, true
}

// inodeOf is inodeKeyOf for callers that compare inode numbers only.
func inodeOf(path string, info os.FileInfo) (uint64, bool) {
	m, ok := readFSMeta(path, info)
	if !ok || m.lacks(lscore.FieldInode) {
		return 0, false
	}
	return m.Ino, true
}

// posixMode builds a st_mode from a Go mode for platforms that have none.
func posixMode(m os.FileMode) uint32 {
	mode := uint32(m.Perm())
	switch {
	case m.IsDir():
		mode |= 0040000
	case m&os.ModeSymlink != 0:
		mode |= 0120000
	case m&os.ModeNamedPipe != 0:
		mode |= 0010000
	case m&os.ModeSocket != 0:
		mode |= 0140000
	case m&os.ModeCharDevice != 0:
		mode |= 0020000
	case m&os.ModeDevice != 0:
		mode |= 0060000
	default:
		mode |= 0100000
	}
	return mode
}
