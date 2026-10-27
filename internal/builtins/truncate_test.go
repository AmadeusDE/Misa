package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTruncateToZero(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hello world")
	r := runBuiltinIn(t, dir, "truncate", "-s", "0", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(filepath.Join(dir, "file.txt"))
	if info.Size() != 0 {
		t.Errorf("size = %d, want 0", info.Size())
	}
}

func TestTruncateExtend(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hi")
	r := runBuiltinIn(t, dir, "truncate", "-s", "100", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(filepath.Join(dir, "file.txt"))
	if info.Size() != 100 {
		t.Errorf("size = %d, want 100", info.Size())
	}
}

func TestTruncateShrink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "hello world, this is long")
	r := runBuiltinIn(t, dir, "truncate", "-s", "5", "file.txt")
	assertExitCode(t, r, 0)
	data, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", data, "hello")
	}
}

func TestTruncateRelativePlus(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "12345")
	r := runBuiltinIn(t, dir, "truncate", "-s", "+5", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(filepath.Join(dir, "file.txt"))
	if info.Size() != 10 {
		t.Errorf("size = %d, want 10", info.Size())
	}
}

func TestTruncateRelativeMinus(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "1234567890")
	r := runBuiltinIn(t, dir, "truncate", "-s", "-3", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(filepath.Join(dir, "file.txt"))
	if info.Size() != 7 {
		t.Errorf("size = %d, want 7", info.Size())
	}
}

func TestTruncateKSuffix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "")
	r := runBuiltinIn(t, dir, "truncate", "-s", "1K", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(filepath.Join(dir, "file.txt"))
	if info.Size() != 1024 {
		t.Errorf("size = %d, want 1024", info.Size())
	}
}

func TestTruncateNoCreate(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "truncate", "-s", "10", "-c", "nonexistent.txt")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "nonexistent.txt")); !os.IsNotExist(err) {
		t.Error("file should not have been created with -c")
	}
}

func TestTruncateCreateNew(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "truncate", "-s", "10", "newfile.txt")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "newfile.txt"))
	if err != nil {
		t.Fatal("file should have been created")
	}
	if info.Size() != 10 {
		t.Errorf("size = %d, want 10", info.Size())
	}
}

func TestTruncateMissingSize(t *testing.T) {
	r := runBuiltin(t, "truncate", "file.txt")
	assertExitCode(t, r, 1)
}
