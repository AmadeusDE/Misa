//go:build unix

package sysinfo

import "syscall"

// mountSpace reports the capacity and free bytes of the filesystem holding path.
func mountSpace(path string) (total, free uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	total, free = getMountSpace(&stat)
	return total, free, nil
}
