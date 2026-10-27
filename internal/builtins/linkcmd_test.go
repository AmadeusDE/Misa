// Behaviour adapted from toybox link.c (0BSD) and uutils coreutils link (MIT).
package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkCommand(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "old", "content")
	assertExitCode(t, runBuiltinIn(t, dir, "link", "old", "new"), 0)
	a, _ := os.Stat(src)
	b, err := os.Stat(filepath.Join(dir, "new"))
	if err != nil || !os.SameFile(a, b) {
		t.Fatalf("not hardlinked: %v", err)
	}
	r := runBuiltinIn(t, dir, "link", "missing", "newer")
	assertExitCode(t, r, 1)
	if r.stderr != "link: cannot create link 'newer' to 'missing': No such file or directory\n" {
		t.Errorf("stderr = %q", r.stderr)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "link", "old", "new"), 1)
	for _, args := range [][]string{nil, {"old"}, {"old", "new", "extra"}, {"", "empty"}} {
		assertExitCode(t, runBuiltinIn(t, dir, "link", args...), 1)
	}
}
