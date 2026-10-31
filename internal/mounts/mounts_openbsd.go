//go:build openbsd

package mounts

import "golang.org/x/sys/unix"

// statFrom maps OpenBSD's statfs, whose fields carry an F_ prefix. x/sys v0.48.0
// exposes no F_frsize for OpenBSD (the kernel keeps it private), so F_bsize is
// the fundamental block size; F_bavail is int64 and is clamped.
func statFrom(st *unix.Statfs_t) Stat {
	return Stat{
		Blocks: st.F_blocks,
		Bfree:  st.F_bfree,
		Bavail: clamp(st.F_bavail),
		Files:  st.F_files,
		Ffree:  st.F_ffree,
		Bsize:  clamp(st.F_bsize),
	}
}

func mountFrom(st *unix.Statfs_t) Mount {
	return mountOf(st.F_mntfromname[:], st.F_mntonname[:], st.F_fstypename[:])
}
