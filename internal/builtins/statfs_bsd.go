//go:build openbsd || freebsd || darwin

package builtins

import (
	"fmt"
	"io"
	"syscall"
)

// fsValues are the statfs fields stat -f prints, with the BSDs' differing
// field names and types smoothed over.
type fsValues struct {
	fsid                  [2]int32
	namemax, flags        uint64
	bsize, iosize         uint64
	blocks, bfree, bavail uint64
	files, ffree          uint64
}

func printFSInfo(w io.Writer, name string, stfs *syscall.Statfs_t) {
	v := fsInfo(stfs)
	frsize := v.iosize
	if frsize == 0 {
		frsize = v.bsize
	}
	fmt.Fprintf(w, "  File: %q\n", name)
	fmt.Fprintf(w, "    ID: %x%x Namelen: %d Type: %x\n", v.fsid[0], v.fsid[1], v.namemax, v.flags)
	fmt.Fprintf(w, "Block size: %-10d Fundamental block size: %d\n", v.bsize, frsize)
	fmt.Fprintf(w, "Blocks: Total: %-10d Free: %-10d Available: %d\n", v.blocks, v.bfree, v.bavail)
	fmt.Fprintf(w, "Inodes: Total: %-10d Free: %d\n", v.files, v.ffree)
}
