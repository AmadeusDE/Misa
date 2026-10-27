package builtins

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTouchCreateFile(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "touch", "newfile.txt")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "newfile.txt")); err != nil {
		t.Error("file was not created")
	}
}

func TestTouchUpdateTimestamp(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "file.txt", "content")
	old := time.Now().Add(-1 * time.Hour)
	os.Chtimes(p, old, old)
	r := runBuiltinIn(t, dir, "touch", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(p)
	if time.Since(info.ModTime()) > 5*time.Second {
		t.Error("modification time not updated")
	}
}

func TestTouchNoCreate(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "touch", "-c", "doesnotexist.txt")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "doesnotexist.txt")); !os.IsNotExist(err) {
		t.Error("file was created despite -c")
	}
}

func TestTouchMultiple(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "touch", "a.txt", "b.txt", "c.txt")
	assertExitCode(t, r, 0)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("file %s not created", name)
		}
	}
}

func TestTouchMissingOperand(t *testing.T) {
	r := runBuiltin(t, "touch")
	assertExitCode(t, r, 1)
}
