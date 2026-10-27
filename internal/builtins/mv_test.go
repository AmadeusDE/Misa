package builtins

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMvSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "content")
	r := runBuiltinIn(t, dir, "mv", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "src.txt")); !os.IsNotExist(err) {
		t.Error("source still exists after mv")
	}
	data, err := os.ReadFile(filepath.Join(dir, "dst.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content" {
		t.Errorf("dst content = %q", data)
	}
}

func TestMvNoClobber(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "new")
	writeFile(t, dir, "dst.txt", "old")
	r := runBuiltinIn(t, dir, "mv", "-n", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
	data, _ := os.ReadFile(filepath.Join(dir, "dst.txt"))
	if string(data) != "old" {
		t.Error("dst was overwritten despite -n")
	}
}

func TestMvVerbose(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "data")
	r := runBuiltinIn(t, dir, "mv", "-v", "a.txt", "b.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "renamed")
}

func TestMvToDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	os.Mkdir(filepath.Join(dir, "dest"), 0755)
	r := runBuiltinIn(t, dir, "mv", "file.txt", "dest")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "dest", "file.txt")); err != nil {
		t.Error("file not moved into directory")
	}
}

func TestMvMissingOperand(t *testing.T) {
	r := runBuiltin(t, "mv", "onlyone")
	assertExitCode(t, r, 1)
}

func TestMvMissingFileOperandMessage(t *testing.T) {
	r := runBuiltin(t, "mv")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing file operand")
}

func TestMvMissingDestinationMessage(t *testing.T) {
	r := runBuiltin(t, "mv", "no-file")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing destination file operand after 'no-file'")
}

func TestMvTargetNotDirectoryMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f1", "a")
	writeFile(t, dir, "f2", "b")
	r := runBuiltinIn(t, dir, "mv", "f1", "f2", "f1")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "target 'f1': Not a directory")
}

// TestMvIntoSelfDirectory covers moving a directory into a subdirectory
// of itself, which must be refused with a clear diagnostic rather than
// failing with a raw rename() error.
func TestMvIntoSelfDirectory(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "SRC", "a"), 0755)
	writeFile(t, dir, "FILE", "x")
	r := runBuiltinIn(t, dir, "mv", "SRC", "FILE", "SRC")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot move")
	assertStderrContains(t, r, "subdirectory of itself")
	if _, err := os.Stat(filepath.Join(dir, "SRC")); err != nil {
		t.Error("SRC should not have been moved/removed")
	}
}

// TestMvBackupSimple covers mv --backup, including the verbose
// "(backup: 'X')" annotation.
func TestMvBackupSimple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "A", "a-content")
	writeFile(t, dir, "B", "b-content")
	r := runBuiltinIn(t, dir, "mv", "--verbose", "--backup=simple", "A", "B")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "backup: 'B~'")
	data, err := os.ReadFile(filepath.Join(dir, "B~"))
	if err != nil || string(data) != "b-content" {
		t.Errorf("backup missing/wrong: %v %q", err, data)
	}
	data2, _ := os.ReadFile(filepath.Join(dir, "B"))
	if string(data2) != "a-content" {
		t.Errorf("B after mv --backup = %q", data2)
	}
}

// TestMvVerboseUsesGivenDestinationName covers a regression where the
// verbose message printed the resolved absolute path instead of the
// destination name as given on the command line.
func TestMvVerboseUsesGivenDestinationName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "x")
	r := runBuiltinIn(t, dir, "mv", "-v", "a", "b")
	assertExitCode(t, r, 0)
	if got := r.stdout; got != "renamed 'a' -> 'b'\n" {
		t.Errorf("stdout = %q, want %q", got, "renamed 'a' -> 'b'\n")
	}
}

// TestMvSameFileThroughSymlink covers moving a symlink onto its own
// dereferenced target when another hard link to that target still exists:
// this must be refused as "same file", not silently corrupt the link.
func TestMvSameFileThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	f := writeFile(t, dir, "f", "x")
	if err := os.Link(f, filepath.Join(dir, "h")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("f", filepath.Join(dir, "s")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "mv", "s", "f")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "'s' and 'f' are the same file")
	if _, err := os.Lstat(filepath.Join(dir, "s")); err != nil {
		t.Error("s should still exist after the refused move")
	}
}

// TestMvDuplicateSourceWarning covers mv's "specified more than once"
// warning when a source operand's canonical path repeats.
func TestMvDuplicateSourceWarning(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "b"), 0755)
	os.Mkdir(filepath.Join(dir, "d"), 0755)
	r := runBuiltinIn(t, dir, "mv", "--verbose", "./b", "b", "b/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot move")
	assertStderrContains(t, r, "warning: source directory 'b' specified more than once")
	if _, err := os.Stat(filepath.Join(dir, "b")); err != nil {
		t.Error("b should still exist; the duplicate must not have been re-attempted")
	}
}

// TestMvUpdateOlderSkipsWhenNotNewer mirrors the cp update test for mv.
func TestMvUpdateOlderSkipsWhenNotNewer(t *testing.T) {
	dir := t.TempDir()
	old := writeFile(t, dir, "old", "old\n")
	newf := writeFile(t, dir, "new", "new\n")
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	os.Chtimes(old, past, past)
	os.Chtimes(newf, future, future)
	r := runBuiltinIn(t, dir, "mv", "--update", "old", "new")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(old); err != nil {
		t.Error("--update should have skipped the move, leaving old in place")
	}
	data, _ := os.ReadFile(newf)
	if string(data) != "new\n" {
		t.Errorf("--update overwrote a newer dest with an older source: %q", data)
	}
}
