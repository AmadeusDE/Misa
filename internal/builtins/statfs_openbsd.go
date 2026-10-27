package builtins

import "syscall"

func fsInfo(s *syscall.Statfs_t) fsValues {
	return fsValues{fsid: s.F_fsid.Val, namemax: uint64(s.F_namemax), flags: uint64(s.F_flags),
		bsize: uint64(s.F_bsize), iosize: uint64(s.F_iosize), blocks: s.F_blocks, bfree: s.F_bfree,
		bavail: uint64(s.F_bavail), files: s.F_files, ffree: uint64(s.F_ffree)}
}
