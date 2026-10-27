package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeErrorOpeningDir(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "tree", "missing")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "missing [error opening dir]\n\n0 directories, 0 files\n")
}

func TestTreeSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "testfile", "hi")
	r := runBuiltinIn(t, dir, "tree")
	assertExitCode(t, r, 0)
	assertStdout(t, r, ".\n└── testfile\n\n0 directories, 1 file\n")
}

// TestTreeSymlinkNotFollowed verifies that tree displays symlinks with their
// target ("name -> target"), always counts them as files (even when they
// point at a directory), and never descends into them.
func TestTreeSymlinkNotFollowed(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "realdir"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "realdir/inside", "x")
	if err := os.Symlink("realdir", filepath.Join(dir, "linkdir")); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "tree")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "linkdir -> realdir") {
		t.Errorf("expected symlink display with target, got %q", r.stdout)
	}
	if n := strings.Count(r.stdout, "inside"); n != 1 {
		t.Errorf("expected 'inside' to appear once (only under the real directory, not the symlink), got %d times: %q", n, r.stdout)
	}
	if !strings.Contains(r.stdout, "1 directory, 2 files") {
		t.Errorf("expected symlink-to-dir counted as a file, got %q", r.stdout)
	}
}
