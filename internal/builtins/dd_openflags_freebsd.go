//go:build freebsd || dragonfly

// dd's open flags on FreeBSD and DragonFly. O_NOATIME does not exist there and
// FreeBSD's O_DSYNC lives in a header Go does not expose, so iflag=dsync falls
// back to O_SYNC, which asks for more than dsync does rather than less.
package builtins

import "syscall"

// ddOpenFlagBits maps an iflag=/oflag= name to its open(2) flag on this system.
func ddOpenFlagBits(name string) (int, bool) {
	switch name {
	case "direct":
		return syscall.O_DIRECT, true
	case "dsync", "sync":
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
