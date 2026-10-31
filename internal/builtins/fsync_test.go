package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFsyncFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file", "data")
	assertExitCode(t, runBuiltinIn(t, dir, "fsync", "file"), 0)
}

func TestFsyncDataOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file", "data")
	assertExitCode(t, runBuiltinIn(t, dir, "fsync", "-d", "file"), 0)
	assertExitCode(t, runBuiltinIn(t, dir, "fsync", "--data-only", "file"), 0)
}

func TestFsyncMultipleOperands(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one", "1")
	writeFile(t, dir, "two", "2")
	assertExitCode(t, runBuiltinIn(t, dir, "fsync", "one", "two"), 0)

	// A failing operand still lets the others sync, and sets the status.
	r := runBuiltinIn(t, dir, "fsync", "one", "missing", "two")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "fsync: can't sync 'missing': No such file or directory") {
		t.Errorf("stderr = %q, want the missing-file diagnostic", r.stderr)
	}
}

func TestFsyncMissingOperand(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "fsync")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "missing operand") {
		t.Errorf("stderr = %q", r.stderr)
	}
}

func TestFsyncUnknownOption(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file", "data")
	r := runBuiltinIn(t, dir, "fsync", "-q", "file")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "unknown option -q") {
		t.Errorf("stderr = %q", r.stderr)
	}
}

func TestFsyncHelp(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "fsync", "--help")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "Usage:") || !strings.Contains(r.stdout, "--data-only") {
		t.Errorf("--help = %q", r.stdout)
	}
}

func TestFsyncSymlinkToFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "target", "data")
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "fsync", "link"), 0)
}
