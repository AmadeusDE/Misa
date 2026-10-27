package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func InstallLinks(self string, args []string, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: misa --install <directory>")
		return 2
	}

	dir := args[0]
	exe, err := filepath.Abs(self)
	if err != nil {
		fmt.Fprintln(stderr, "misa: --install:", err)
		return 1
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil {
		exe = resolved
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(stderr, "misa: --install:", err)
		return 1
	}

	for _, name := range Names() {
		link := filepath.Join(dir, name)
		os.Remove(link)
		if err := os.Symlink(exe, link); err != nil {
			fmt.Fprintf(stderr, "misa: --install: %s: %v\n", name, err)
		} else {
			fmt.Fprintf(stderr, "%s -> %s\n", link, exe)
		}
	}

	return 0
}
