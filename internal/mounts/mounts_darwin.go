//go:build darwin

package mounts

import "golang.org/x/sys/unix"

// statFrom maps Darwin's statfs. It has no separate fragment size (Iosize is
// the optimal transfer size), so Bsize is the allocation block size and is
// already the fundamental block size df reports.
func statFrom(st *unix.Statfs_t) Stat {
	return Stat{
		Blocks: st.Blocks,
		Bfree:  st.Bfree,
		Bavail: st.Bavail,
		Files:  st.Files,
		Ffree:  st.Ffree,
		Bsize:  clamp(st.Bsize),
	}
}

func mountFrom(st *unix.Statfs_t) Mount {
	return mountOf(st.Mntfromname[:], st.Mntonname[:], st.Fstypename[:])
}
