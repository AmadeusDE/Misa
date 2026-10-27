package builtins

import "syscall"

// Darwin's statfs has no name length; its file systems allow 255 bytes.
func fsInfo(s *syscall.Statfs_t) fsValues {
	return fsValues{fsid: s.Fsid.Val, namemax: 255, flags: uint64(s.Flags),
		bsize: uint64(s.Bsize), iosize: uint64(s.Iosize), blocks: s.Blocks, bfree: s.Bfree,
		bavail: s.Bavail, files: s.Files, ffree: s.Ffree}
}
