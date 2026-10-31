//go:build linux

package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fsync(2) on a directory is supported on Linux; several other platforms
// reject it with EINVAL, so the case stays in a Linux-only file.
func TestFsyncDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "fsync", "sub"), 0)
	assertExitCode(t, runBuiltinIn(t, dir, "fsync", "-d", "sub"), 0)
}

func TestFsyncNonDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file", "data")
	r := runBuiltinIn(t, dir, "fsync", "file/child")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "fsync: can't sync 'file/child'") {
		t.Errorf("stderr = %q", r.stderr)
	}
}
