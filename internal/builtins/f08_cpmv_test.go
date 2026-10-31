// Regression scenarios informed by Toybox tests/cp.test at
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD), and MIT uutils/coreutils
// tests/by-util/test_mv.rs at fee61ff5d4f87e0f912a52e639942b5f7ad58663.
// Written natively for Misa; local adapter fixtures differ from the pins.
package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestF08CopySymlinkSourceMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("contents"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "source"), 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("source", filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	// Create a control with the same requested mode, so the test respects umask.
	f, err := os.OpenFile(filepath.Join(dir, "control"), os.O_CREATE|os.O_WRONLY, 0400)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	r := runBuiltinIn(t, dir, "cp", "link", "copy")
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Fatalf("cp: %+v", r)
	}
	got, err := os.Stat(filepath.Join(dir, "copy"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(filepath.Join(dir, "control"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode().Perm() != want.Mode().Perm() {
		t.Errorf("mode = %o, want %o", got.Mode().Perm(), want.Mode().Perm())
	}
}

func TestF08MoveDotOntoDot(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "mv", ".", ".")
	if r.code != 1 || r.stdout != "" || r.stderr != "mv: '.' and '.' are the same file\n" {
		t.Fatalf("mv: %+v", r)
	}
}

func TestF08MoveHardlinkedOperands(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "a"), filepath.Join(dir, "b")); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "target"), 0700); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "mv", "a", "b", "target")
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("mv: %+v", r)
	}
	a, ea := os.Stat(filepath.Join(dir, "target", "a"))
	b, eb := os.Stat(filepath.Join(dir, "target", "b"))
	if ea != nil || eb != nil {
		t.Fatalf("destination files: %v, %v", ea, eb)
	}
	if !os.SameFile(a, b) {
		t.Error("destination files lost their hardlink")
	}
	for _, name := range []string{"a", "b"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("source %s remains: %v", name, err)
		}
	}
}

func TestF08CopyKeepsExistingDestinationMode(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"source", "destination"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(dir, "source"), 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "destination"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("destination", filepath.Join(dir, "destlink")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	r := runBuiltinIn(t, dir, "cp", "source", "destlink")
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Fatalf("cp: %+v", r)
	}
	fi, err := os.Stat(filepath.Join(dir, "destination"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("mode = %o, want 600", fi.Mode().Perm())
	}
	data, err := os.ReadFile(filepath.Join(dir, "destination"))
	if err != nil || string(data) != "source" {
		t.Errorf("content = %q, err = %v", data, err)
	}
	link, err := os.Readlink(filepath.Join(dir, "destlink"))
	if err != nil || link != "destination" {
		t.Errorf("destination link = %q, err = %v", link, err)
	}
}
