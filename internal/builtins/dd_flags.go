// dd's platform-specific iflag=/oflag= flags.
//
// direct, dsync, sync, nonblock, noatime, noctty, nofollow and directory are
// open(2) flags, so each platform file maps the ones its kernel defines onto
// the matching syscall constants, and the copy reports "unsupported on this
// platform" for the rest rather than pretending the flag worked (the name list
// comes from uutils coreutils dd, src/uu/dd/src/parseargs.rs, MIT, which gates
// the same names on Linux).
package builtins

import (
	"errors"
	"os"
	"unsafe"
)

// errPlatformFlag is what a platform file returns for an iflag=/oflag= name it
// knows but cannot honour on this operating system.
var errPlatformFlag = errors.New("unsupported on this platform")

// ddPlatformOpenFlag resolves an iflag=/oflag= name to os.OpenFile flag bits.
// A name this platform does not implement at all returns 0 with a nil error so
// that the parser can report it as invalid.
func ddPlatformOpenFlag(name string) (int, error) {
	if !ddPlatformFlagNames[name] {
		return 0, nil
	}
	bits, ok := ddOpenFlagBits(name)
	if !ok {
		return 0, errPlatformFlag
	}
	return bits, nil
}

// errNotDir reports iflag=directory on a redirected stream that is a file.
var errNotDir = errors.New("not a directory")

// ddSetStreamFlags applies the iflag=/oflag= names to an inherited stream such
// as standard input, which dd cannot reopen with the flags it was asked for.
// The flags that only exist at open(2) time cannot be set on an open
// descriptor, so they either check what they can (directory) or fail honestly
// instead of being dropped.
func ddSetStreamFlags(file *os.File, names []string, bits int) error {
	if len(names) == 0 || file == nil {
		return nil
	}
	for _, name := range names {
		switch name {
		case "directory":
			info, err := file.Stat()
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return errNotDir
			}
		case "nofollow", "noctty":
			return errPlatformFlag
		}
	}
	return ddFcntlSetFlags(file, bits)
}

// ddAlignBytes returns a slice of n bytes whose first byte is aligned to
// align. iflag=/oflag=direct transfers must be aligned to the device block
// size, which the Go heap does not guarantee.
func ddAlignBytes(n, align int) []byte {
	buf := make([]byte, n+align)
	off := (align - int(uintptr(unsafe.Pointer(&buf[0])))%align) % align
	return buf[off : off+n]
}

// ddDirectBit is the platform bit for iflag=/oflag=direct, which the copy loop
// clears and restores around an unaligned write.
func ddDirectBit() int {
	bits, _ := ddOpenFlagBits("direct")
	return bits
}
