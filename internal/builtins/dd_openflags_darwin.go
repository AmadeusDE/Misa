//go:build darwin

// dd's open flags on macOS. O_DIRECT and O_NOATIME do not exist there, so
// iflag=direct and iflag=noatime fail with "unsupported on this platform".
package builtins

import "syscall"

// ddOpenFlagBits maps an iflag=/oflag= name to its open(2) flag on this system.
func ddOpenFlagBits(name string) (int, bool) {
	switch name {
	case "dsync":
		return syscall.O_DSYNC, true
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
