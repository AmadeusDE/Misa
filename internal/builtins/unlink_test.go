// Behaviour adapted from toybox unlink.c (0BSD) and uutils coreutils unlink (MIT).
package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnlinkCommand(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "file", "content")
	if err := os.Symlink("file", filepath.Join(dir, "sym")); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "unlink", "sym"), 0)
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, reason string }{{"sub", "Is a directory"}, {"missing", "No such file or directory"}, {"", "No such file or directory"}} {
		r := runBuiltinIn(t, dir, "unlink", tc.name)
		assertExitCode(t, r, 1)
		if r.stderr != "unlink: cannot unlink '"+tc.name+"': "+tc.reason+"\n" {
			t.Errorf("stderr = %q", r.stderr)
		}
	}
	assertExitCode(t, runBuiltinIn(t, dir, "unlink", "file", "sym"), 1)
	assertExitCode(t, runBuiltinIn(t, dir, "unlink", "file"), 0)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("file remains: %v", err)
	}
}
