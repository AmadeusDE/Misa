package sysinfo

import "syscall"

func getMountSpace(stat *syscall.Statfs_t) (total uint64, free uint64) {
	return stat.F_blocks * uint64(stat.F_bsize), uint64(stat.F_bavail) * uint64(stat.F_bsize)
}
