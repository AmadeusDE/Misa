package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLnHardLink(t *testing.T) {
	skipWithoutHardlinks(t)
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "data")
	r := runBuiltinIn(t, dir, "ln", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "dst.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Errorf("content = %q", data)
	}
}

func TestLnSymlink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "data")
	r := runBuiltinIn(t, dir, "ln", "-s", "src.txt", "link.txt")
	assertExitCode(t, r, 0)
	target, err := os.Readlink(filepath.Join(dir, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "src.txt" {
		t.Errorf("symlink target = %q, want %q", target, "src.txt")
	}
}

func TestLnSymlinkPreservesRelativePath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	writeFile(t, dir, "sub/src.txt", "data")
	r := runBuiltinIn(t, dir, "ln", "-s", "./sub/src.txt", "link.txt")
	assertExitCode(t, r, 0)
	target, err := os.Readlink(filepath.Join(dir, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "./sub/src.txt" {
		t.Errorf("symlink target = %q, want %q", target, "./sub/src.txt")
	}
}

func TestLnForce(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "data")
	writeFile(t, dir, "dst.txt", "old")
	r := runBuiltinIn(t, dir, "ln", "-sf", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
}

func TestLnMissingOperand(t *testing.T) {
	r := runBuiltin(t, "ln", "onlyone")
	assertExitCode(t, r, 1)
}

func TestRmdirBasic(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "empty"), 0755)
	r := runBuiltinIn(t, dir, "rmdir", "empty")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "empty")); !os.IsNotExist(err) {
		t.Error("directory still exists")
	}
}

func TestRmdirNonEmpty(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "notempty"), 0755)
	writeFile(t, dir, "notempty/file.txt", "data")
	r := runBuiltinIn(t, dir, "rmdir", "notempty")
	assertExitCode(t, r, 1)
}

func TestRmdirParents(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a/b/c"), 0755)
	runBuiltinIn(t, dir, "rmdir", "-p", "a/b/c")
	if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Error("parent directory 'a' still exists")
	}
}

func TestRmdirMissingOperand(t *testing.T) {
	r := runBuiltin(t, "rmdir")
	assertExitCode(t, r, 1)
}
