//go:build darwin || freebsd || openbsd

package mounts

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const (
	fsstatInitial = 16
	fsstatMax     = 4096 // 4096 Statfs_t is ~8 MB on FreeBSD, far past any real table
)

// List reads the BSD mount table through getfsstat. It fills the buffer and
// returns how many entries fit; n == len(buf) means the kernel may have
// truncated the table, so the buffer doubles until it has room.
func List() ([]Mount, error) {
	for size := fsstatInitial; ; size *= 2 {
		buf := make([]unix.Statfs_t, size)
		n, err := unix.Getfsstat(buf, unix.MNT_NOWAIT)
		if err != nil {
			return nil, err
		}
		if n < len(buf) {
			return mountsFrom(buf[:n]), nil
		}
		if size >= fsstatMax {
			return nil, fmt.Errorf("getfsstat: more than %d mounted filesystems", fsstatMax)
		}
	}
}

// mountsFrom converts the kernel's mount table. MNT_NOWAIT also lists stale
// NFS mounts, and the per-mount BSD flags are not an option list, so Options
// stays empty.
func mountsFrom(buf []unix.Statfs_t) []Mount {
	mounts := make([]Mount, 0, len(buf))
	for i := range buf {
		mounts = append(mounts, mountFrom(&buf[i]))
	}
	return mounts
}

func mountOf(device, dir, fstype []byte) Mount {
	return Mount{
		Device: unix.ByteSliceToString(device),
		Dir:    unix.ByteSliceToString(dir),
		FSType: unix.ByteSliceToString(fstype),
	}
}

func Statfs(path string) (Stat, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return Stat{}, err
	}
	return statFrom(&st), nil
}
