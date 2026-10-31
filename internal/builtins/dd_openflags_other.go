//go:build !linux && !darwin && !freebsd && !dragonfly && !openbsd && !netbsd && !solaris

// dd's open flags where this port has no iflag=/oflag= support at all: every
// platform flag is refused with "unsupported on this platform".
package builtins

// ddOpenFlagBits maps an iflag=/oflag= name to its open(2) flag on this system.
func ddOpenFlagBits(string) (int, bool) { return 0, false }
