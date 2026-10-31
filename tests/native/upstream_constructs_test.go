package native

import (
	"os"
	"path/filepath"
	"testing"
)

// These independently authored cases express multi-step constructs directly,
// without asking the upstream adapters to interpret shell syntax.
func TestAdapterNativeConstructs(t *testing.T) {
	t.Run("chmod loop", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"one", "two", "three"} {
			mkFileMode(t, dir, name, "data", 0600)
		}
		for _, name := range []string{"one", "two", "three"} {
			wantSuccess(t, runIn(t, dir, "chmod", "u+x", name))
			info, err := os.Stat(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0700 {
				t.Errorf("%s: mode %04o", name, got)
			}
		}
	})
	t.Run("pipeline bytes", func(t *testing.T) {
		first := runStdin(t, "tr", "alpha\nbeta\n", "a-z", "A-Z")
		wantSuccess(t, first)
		second := runStdin(t, "head", first.stdout, "-n", "1")
		wantSuccess(t, second)
		wantOut(t, second, "ALPHA\n")
	})
	t.Run("relative symlink chain", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
			t.Fatal(err)
		}
		mkFile(t, dir, "target", "chain data\n")
		if err := os.Symlink("../target", filepath.Join(dir, "sub", "first")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("sub/first", filepath.Join(dir, "second")); err != nil {
			t.Fatal(err)
		}
		r := runIn(t, dir, "cat", "second")
		wantSuccess(t, r)
		wantOut(t, r, "chain data\n")
	})
}
