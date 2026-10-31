//go:build !unix

package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// mkdirParents creates every missing component of path. Without directory
// descriptors there is no ancestor-permission walk to preserve; parents get
// the default access and only an explicit -m touches the final directory (on
// Windows that can only toggle the read-only attribute, see support.go).
func mkdirParents(ctx *Context, rawDir, path string, perm os.FileMode, hasMode, verbose bool) error {
	path = filepath.Clean(path)
	var missing []string
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Stat(p)
		if err == nil {
			if !info.IsDir() {
				if p == path {
					return fmt.Errorf("File exists")
				}
				return &os.PathError{Op: "mkdir", Path: p, Err: fmt.Errorf("Not a directory")}
			}
			break
		}
		missing = append([]string{p}, missing...)
		if filepath.Dir(p) == p {
			break
		}
	}
	rawParts := strings.Split(filepath.ToSlash(filepath.Clean(rawDir)), "/")
	for i, p := range missing {
		if err := os.Mkdir(p, 0777); err != nil {
			return err
		}
		if i == len(missing)-1 && hasMode {
			os.Chmod(p, perm)
		}
		if verbose {
			keep := len(rawParts) - (len(missing) - 1 - i)
			if keep < 1 || keep > len(rawParts) {
				keep = len(rawParts)
			}
			fmt.Fprintf(ctx.Stdout, "mkdir: created directory '%s'\n", strings.Join(rawParts[:keep], "/"))
		}
	}
	return nil
}
