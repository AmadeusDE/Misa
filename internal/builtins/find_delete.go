// Delete semantics informed by Toybox find.c, copyright 2014 Rob Landley,
// commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD), and uutils
// findutils delete.rs, commit d71f3242b0bc3b21bb311f04b23ae62e5f3fed67 (MIT).
package builtins

import (
	"errors"
	"os"
	"path/filepath"
)

// findDelete checks identities immediately before removal. Like ValidateDirs,
// this is not atomic and does not defend against a hostile concurrent renamer.
func findDelete(e *findEntry) bool {
	if err := e.ctx.DoneContext().Err(); err != nil {
		return e.fail(err)
	}
	// Preserve dot roots, but still visit and delete their children. A dot-dot
	// root is rejected before traversal to avoid deleting its contents.
	if e.depth == 0 {
		switch filepath.Base(e.root) {
		case ".":
			return true
		case "..":
			return e.fail(errors.New("refusing to delete .."))
		}
	}
	if e.validate != nil {
		if err := e.validate(); err != nil {
			return e.fail(err)
		}
	}
	current, err := os.Lstat(e.full)
	if err != nil {
		return e.fail(err)
	}
	if !os.SameFile(e.info, current) || e.info.Mode().Type() != current.Mode().Type() {
		return e.fail(errors.New("entry changed during traversal"))
	}
	if err := os.Remove(e.full); err != nil {
		return e.fail(err)
	}
	return true
}
