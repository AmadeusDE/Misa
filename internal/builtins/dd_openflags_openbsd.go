//go:build openbsd || netbsd || solaris

// dd's open flags on the other unix systems Go builds for. OpenBSD has no
// O_DIRECT and none of them have O_NOATIME, so those flags are refused;
// O_DSYNC is spelled O_SYNC where the constant is missing.
package builtins

import "syscall"

// ddOpenFlagBits maps an iflag=/oflag= name to its open(2) flag on this system.
func ddOpenFlagBits(name string) (int, bool) {
	switch name {
	case "sync":
		return syscall.O_SYNC, true
	case "nonblock":
		return syscall.O_NONBLOCK, true
	case "noctty":
		return syscall.O_NOCTTY, true
	case "nofollow":
		return syscall.O_NOFOLLOW, true
	case "directory":
		return syscall.O_DIRECTORY, true
	}
	return 0, false
}
