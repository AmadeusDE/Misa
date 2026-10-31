//go:build unix

package procmon

import "syscall"

// diskSpace reports the capacity and free bytes of the filesystem holding path.
func diskSpace(path string) (total, free uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	total, free = getDiskSpace(&stat)
	return total, free, nil
}
