//go:build unix

package builtins

import (
	"os"
	"syscall"

	"github.com/charmbracelet/x/term"
)

// defaultPSOwnTTY returns the device number of the terminal ps itself runs on:
// the first of the standard streams that is a real terminal, as Toybox's
// tty_fd() checks. A character device that is not a terminal (such as
// /dev/null) does not count. Without a terminal the default selection lists
// the processes that have none.
func defaultPSOwnTTY() int {
	for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		if !term.IsTerminal(f.Fd()) {
			continue
		}
		if fi, err := f.Stat(); err == nil {
			if st, ok := fi.Sys().(*syscall.Stat_t); ok {
				return int(st.Rdev)
			}
		}
	}
	return 0
}
