package native

import (
	"strings"
	"testing"
)

func TestReadlink_Basic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target.txt", "data")
	symlink(t, dir+"/target.txt", dir+"/link.txt")
	r := runIn(t, dir, "readlink", "link.txt")
	wantSuccess(t, r)
	got := strings.TrimSpace(r.stdout)
	if got != dir+"/target.txt" {
		t.Errorf("readlink = %q, want %q", got, dir+"/target.txt")
	}
}

func TestReadlink_NotSymlink(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "regular.txt", "data")
	r := runIn(t, dir, "readlink", "regular.txt")
	wantFail(t, r)
}

func TestReadlink_Missing(t *testing.T) {
	r := run(t, "readlink", "nonexistent")
	wantFail(t, r)
}

func TestReadlink_Canonicalize(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target.txt", "data")
	symlink(t, dir+"/target.txt", dir+"/link.txt")
	r := runIn(t, dir, "readlink", "-f", "link.txt")
	wantSuccess(t, r)
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected canonicalized path")
	}
}

func TestReadlink_SymlinkLoop(t *testing.T) {
	dir := t.TempDir()
	symlink(t, dir+"/loop", dir+"/loop")
	r := runIn(t, dir, "readlink", "-ev", "loop")
	wantFail(t, r)
	wantErrContains(t, r, "Too many levels of symbolic links")
}
