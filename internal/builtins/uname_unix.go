//go:build unix

package builtins

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

func utsFieldPtr(ptr unsafe.Pointer, maxLen int) string {
	b := unsafe.Slice((*byte)(ptr), maxLen)
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return string(b[:n])
}

func readUname() (unameInfo, error) {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return unameInfo{}, err
	}
	return unameInfo{
		sysname:  utsFieldPtr(unsafe.Pointer(&uts.Sysname[0]), len(uts.Sysname)),
		nodename: utsFieldPtr(unsafe.Pointer(&uts.Nodename[0]), len(uts.Nodename)),
		release:  utsFieldPtr(unsafe.Pointer(&uts.Release[0]), len(uts.Release)),
		version:  utsFieldPtr(unsafe.Pointer(&uts.Version[0]), len(uts.Version)),
		machine:  utsFieldPtr(unsafe.Pointer(&uts.Machine[0]), len(uts.Machine)),
		osName:   "GNU/Linux",
	}, nil
}
