//go:build unix && !openbsd && !freebsd && !netbsd && !darwin && !dragonfly

package procmon

import "syscall"

func getDiskSpace(stat *syscall.Statfs_t) (total uint64, free uint64) {
	return stat.Blocks * uint64(stat.Bsize), stat.Bfree * uint64(stat.Bsize)
}
