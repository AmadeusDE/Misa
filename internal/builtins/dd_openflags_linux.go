//go:build linux

// dd's open flags on Linux (and Android, which matches the linux build tag).
// Every name in the allowlist is a real open(2) flag here; nocache is still
// unimplemented because it needs the fadvise interface, not an open flag.
package builtins

import "syscall"

// ddOpenFlagBits maps an iflag=/oflag= name to its open(2) flag on this system.
func ddOpenFlagBits(name string) (int, bool) {
	switch name {
	case "direct":
		return syscall.O_DIRECT, true
	case "dsync":
		return syscall.O_DSYNC, true
	case "sync":
		return syscall.O_SYNC, true
	case "nonblock":
		return syscall.O_NONBLOCK, true
	case "noatime":
		return syscall.O_NOATIME, true
	case "noctty":
		return syscall.O_NOCTTY, true
	case "nofollow":
		return syscall.O_NOFOLLOW, true
	case "directory":
		return syscall.O_DIRECTORY, true
	}
	return 0, false
}
