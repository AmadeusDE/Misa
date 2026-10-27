//go:build !openbsd && !freebsd && !netbsd && !darwin && !dragonfly

package builtins

import (
	"fmt"
	"io"
	"syscall"
)

func printFSInfo(w io.Writer, name string, stfs *syscall.Statfs_t) {
	bsize := stfs.Bsize
	frsize := stfs.Frsize
	if frsize == 0 {
		frsize = bsize
	}
	fmt.Fprintf(w, "  File: %q\n", name)
	fmt.Fprintf(w, "    ID: %x%x Namelen: %d Type: %x\n", stfs.Fsid.X__val[0], stfs.Fsid.X__val[1], stfs.Namelen, stfs.Type)
	fmt.Fprintf(w, "Block size: %-10d Fundamental block size: %d\n", bsize, frsize)
	fmt.Fprintf(w, "Blocks: Total: %-10d Free: %-10d Available: %d\n", stfs.Blocks, stfs.Bfree, stfs.Bavail)
	fmt.Fprintf(w, "Inodes: Total: %-10d Free: %d\n", stfs.Files, stfs.Ffree)
}
