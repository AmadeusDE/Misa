//go:build linux

package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fallocateSize(t *testing.T, dir, name string) int64 {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	return info.Size()
}

// The three size cases below are toybox tests/fallocate.test (0BSD).
func TestFallocateGrowsFile(t *testing.T) {
	dir := t.TempDir()
	assertExitCode(t, runBuiltinIn(t, dir, "fallocate", "-l", "123", "foo"), 0)
	if got := fallocateSize(t, dir, "foo"); got != 123 {
		t.Errorf("size = %d, want 123", got)
	}
	// A shorter length does not shrink the file.
	assertExitCode(t, runBuiltinIn(t, dir, "fallocate", "-l", "12", "foo"), 0)
	if got := fallocateSize(t, dir, "foo"); got != 123 {
		t.Errorf("size = %d, want 123 after a shorter request", got)
	}
	// An offset extends past the old end.
	assertExitCode(t, runBuiltinIn(t, dir, "fallocate", "-o", "200", "-l", "12", "foo"), 0)
	if got := fallocateSize(t, dir, "foo"); got != 212 {
		t.Errorf("size = %d, want 212", got)
	}
}

func TestFallocateSuffixes(t *testing.T) {
	dir := t.TempDir()
	assertExitCode(t, runBuiltinIn(t, dir, "fallocate", "-l", "1k", "foo"), 0)
	if got := fallocateSize(t, dir, "foo"); got != 1024 {
		t.Errorf("size = %d, want 1024", got)
	}
}

// A zero length is rejected by fallocate(2) with EINVAL, exactly as toybox's
// posix_fallocate call reports it. Misa reports the failure instead of faking it.
func TestFallocateZeroLengthFails(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "fallocate", "-l", "0", "foo")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "foo:") {
		t.Errorf("stderr = %q, want the operand name", r.stderr)
	}
}

func TestFallocateNegativeLengthFails(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "fallocate", "-l", "-1", "foo")
	assertExitCode(t, r, 1)
	if r.stderr == "" {
		t.Error("a negative length must be reported")
	}
}

func TestFallocateUnwritableDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "ro")
	if err := os.MkdirAll(sub, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sub, 0755) })
	r := runBuiltinIn(t, dir, "fallocate", "-l", "10", "ro/foo")
	if os.Getuid() == 0 {
		t.Skip("root ignores the directory mode")
	}
	assertExitCode(t, r, 1)
	if r.stderr == "" {
		t.Error("an unopenable file must be reported")
	}
}
