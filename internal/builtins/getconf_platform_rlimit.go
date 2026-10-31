//go:build linux || darwin || freebsd || openbsd

package builtins

import "golang.org/x/sys/unix"

// The rlimits behind OPEN_MAX and CHILD_MAX. x/sys/unix has both on every
// platform this file is built for; android matches the linux tag.

// platformOpenMax is the soft RLIMIT_NOFILE: the per-process descriptor cap
// POSIX calls OPEN_MAX.
func platformOpenMax() (int64, bool) { return platformRlimit(unix.RLIMIT_NOFILE) }

// platformChildMax is the soft RLIMIT_NPROC: the kernel's cap on processes per
// real user ID, which is what POSIX's CHILD_MAX describes.
func platformChildMax() (int64, bool) { return platformRlimit(unix.RLIMIT_NPROC) }

// platformRlimit reports a soft rlimit. An unlimited one has no finite value to
// print, so it counts as "cannot determine" rather than a huge number.
func platformRlimit(resource int) (int64, bool) {
	var lim unix.Rlimit
	if err := unix.Getrlimit(resource, &lim); err != nil {
		return 0, false
	}
	if lim.Cur == unix.RLIM_INFINITY {
		return 0, false
	}
	return int64(lim.Cur), true
}
