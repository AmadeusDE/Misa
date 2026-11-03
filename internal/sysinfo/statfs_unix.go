//go:build unix && !openbsd && !freebsd && !netbsd && !darwin && !dragonfly

package sysinfo

import "syscall"

func getMountSpace(stat *syscall.Statfs_t) (total uint64, free uint64) {
	return stat.Blocks * uint64(stat.Bsize), uint64(stat.Bavail) * uint64(stat.Bsize)
}
