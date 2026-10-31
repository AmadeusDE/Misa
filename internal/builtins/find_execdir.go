// Execdir path handling informed by uutils findutils exec.rs, commit
// d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT).
package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findExecDir preserves the final lexical component and resolves the parent
// before dispatch, so a symlink followed by .. is not cleaned prematurely.
func findExecDir(e *findEntry, ctx *Context) (dir, name string, validate func() error, err error) {
	if e.validate != nil {
		if err = e.validate(); err != nil {
			return
		}
	}
	path := strings.TrimRight(e.path, "/")
	parent, base := ".", path
	if path == "" {
		parent, base = "/", "."
	} else if i := strings.LastIndexByte(path, '/'); i >= 0 {
		parent, base = path[:i], path[i+1:]
		if parent == "" {
			parent = "/"
		}
	}
	raw := findFullPath(ctx.Dir, parent)
	var info os.FileInfo
	info, err = os.Stat(raw)
	if err != nil {
		return
	}
	dir, err = filepath.EvalSymlinks(raw)
	if err != nil {
		return
	}
	name = "./" + base
	validate = func() error {
		if e.validate != nil {
			if err := e.validate(); err != nil {
				return err
			}
		}
		current, err := os.Stat(raw)
		if err != nil {
			return err
		}
		if !os.SameFile(info, current) {
			return fmt.Errorf("directory changed during traversal: %s", raw)
		}
		return nil
	}
	return
}
