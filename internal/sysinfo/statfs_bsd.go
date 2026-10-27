//go:build freebsd || darwin || dragonfly

package sysinfo

import "syscall"

func getMountSpace(stat *syscall.Statfs_t) (total uint64, free uint64) {
	return uint64(stat.Blocks) * uint64(stat.Bsize), uint64(stat.Bavail) * uint64(stat.Bsize)
}
