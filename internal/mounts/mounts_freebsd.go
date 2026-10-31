//go:build freebsd

package mounts

import "golang.org/x/sys/unix"

// statFrom maps FreeBSD's statfs. Bavail and Ffree are int64 there (the kernel
// reports -1 for an unknown inode count), so they are clamped.
func statFrom(st *unix.Statfs_t) Stat {
	return Stat{
		Blocks: st.Blocks,
		Bfree:  st.Bfree,
		Bavail: clamp(st.Bavail),
		Files:  st.Files,
		Ffree:  clamp(st.Ffree),
		Bsize:  clamp(st.Bsize),
	}
}

func mountFrom(st *unix.Statfs_t) Mount {
	return mountOf(st.Mntfromname[:], st.Mntonname[:], st.Fstypename[:])
}
