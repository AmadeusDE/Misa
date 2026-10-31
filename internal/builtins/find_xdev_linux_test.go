//go:build linux && !android

package builtins

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestFindXdevProc(t *testing.T) {
	dir := t.TempDir()
	root, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	proc, err := os.Stat("/proc")
	if err != nil {
		t.Skip(err)
	}
	a, aok := root.Sys().(*syscall.Stat_t)
	b, bok := proc.Sys().(*syscall.Stat_t)
	if !aok || !bok || a.Dev == b.Dev {
		t.Skip("/proc is not a distinct device")
	}
	if err := os.Symlink("/proc", filepath.Join(dir, "proc")); err != nil {
		t.Skip(err)
	}
	out, stderr, code := findTestRun(t, dir, "-L", ".", "-xdev", "-maxdepth", "2")
	if code != 0 || stderr != "" || out != ".\n./proc\n" {
		t.Fatalf("%q %q %d", out, stderr, code)
	}
}
