//go:build freebsd || darwin || dragonfly

package procmon

import "syscall"

func getDiskSpace(stat *syscall.Statfs_t) (total uint64, free uint64) {
	return uint64(stat.Blocks) * uint64(stat.Bsize), uint64(stat.Bfree) * uint64(stat.Bsize)
}
