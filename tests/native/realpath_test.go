package native

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRealpath_Absolute(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "realpath", "f.txt")
	wantSuccess(t, r)
	got := strings.TrimSpace(r.stdout)
	abs, _ := filepath.EvalSymlinks(filepath.Join(dir, "f.txt"))
	if got != abs {
		t.Errorf("realpath = %q, want %q", got, abs)
	}
}

func TestRealpath_Symlink(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target.txt", "data")
	symlink(t, dir+"/target.txt", dir+"/link.txt")
	r := runIn(t, dir, "realpath", "link.txt")
	wantSuccess(t, r)
	got := strings.TrimSpace(r.stdout)
	abs, _ := filepath.EvalSymlinks(filepath.Join(dir, "target.txt"))
	if got != abs {
		t.Errorf("realpath = %q, want %q", got, abs)
	}
}

func TestRealpath_Missing(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "realpath", "nonexistent")
	wantSuccess(t, r)
	got := strings.TrimSpace(r.stdout)
	if got != filepath.Join(dir, "nonexistent") {
		t.Errorf("realpath nonexistent = %q, want %q", got, filepath.Join(dir, "nonexistent"))
	}
}

func TestRealpath_SymlinkLoop(t *testing.T) {
	dir := t.TempDir()
	symlink(t, dir+"/loop", dir+"/loop")
	r := runIn(t, dir, "realpath", "loop")
	wantFail(t, r)
	wantErrContains(t, r, "Too many levels of symbolic links")
}

func TestRealpath_RelativeToNotDir(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file.txt", "data")
	r := runIn(t, dir, "realpath", "-e", "--relative-to=file.txt", ".")
	wantFail(t, r)
	wantErrContains(t, r, "Not a directory")
}
