//go:build unix

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"golang.org/x/sys/unix"
	"os"
)

func linkRoots(src *os.Root, old string, dst *os.Root, name string) error {
	a, e := src.Open(".")
	if e != nil {
		return e
	}
	defer a.Close()
	b, e := dst.Open(".")
	if e != nil {
		return e
	}
	defer b.Close()
	return unix.Linkat(int(a.Fd()), old, int(b.Fd()), name, 0)
}
