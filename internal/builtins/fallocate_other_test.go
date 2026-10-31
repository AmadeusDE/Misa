//go:build !linux

package builtins

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fallocate(2) is Linux-only for Go, so every other platform must fail
// honestly instead of pretending to reserve space.
func TestFallocateUnsupportedPlatform(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "fallocate", "-l", "10", "file")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "not supported on this platform") {
		t.Errorf("stderr = %q on %s, want an unsupported-platform error", r.stderr, runtime.GOOS)
	}
	if _, err := os.Stat(filepath.Join(dir, "file")); !os.IsNotExist(err) {
		t.Errorf("fallocate must not touch the file when unsupported (stat err = %v)", err)
	}
}
