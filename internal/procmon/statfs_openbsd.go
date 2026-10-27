package procmon

import "syscall"

func getDiskSpace(stat *syscall.Statfs_t) (total uint64, free uint64) {
	return stat.F_blocks * uint64(stat.F_bsize), stat.F_bfree * uint64(stat.F_bsize)
}
