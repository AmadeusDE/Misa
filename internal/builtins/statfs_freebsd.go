package builtins

import "syscall"

func fsInfo(s *syscall.Statfs_t) fsValues {
	return fsValues{fsid: s.Fsid.Val, namemax: uint64(s.Namemax), flags: s.Flags,
		bsize: s.Bsize, iosize: s.Iosize, blocks: s.Blocks, bfree: s.Bfree,
		bavail: uint64(s.Bavail), files: s.Files, ffree: uint64(s.Ffree)}
}
