package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRmSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "rm", "file.txt")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "file.txt")); !os.IsNotExist(err) {
		t.Error("file still exists after rm")
	}
}

func TestRmMissingFile(t *testing.T) {
	r := runBuiltin(t, "rm", "nonexistent.txt")
	assertExitCode(t, r, 1)
}

func TestRmForceMissing(t *testing.T) {
	r := runBuiltin(t, "rm", "-f", "nonexistent.txt")
	assertExitCode(t, r, 0)
}

func TestRmForceNoArgs(t *testing.T) {
	r := runBuiltin(t, "rm", "-f")
	assertExitCode(t, r, 0)
}

func TestRmDirWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	r := runBuiltinIn(t, dir, "rm", "subdir")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Is a directory")
}

func TestRmRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub/deep"), 0755)
	writeFile(t, dir, "sub/deep/file.txt", "data")
	r := runBuiltinIn(t, dir, "rm", "-r", "sub")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "sub")); !os.IsNotExist(err) {
		t.Error("directory still exists after rm -r")
	}
}

// rm refuses to operate on "/" as a safety net even without -r (stricter
// than real GNU rm, which would just fail with "Is a directory" here since
// bare rm can't remove directories at all; -r triggers the same refusal in
// both).
func TestRmRefuseRoot(t *testing.T) {
	r := runBuiltin(t, "rm", "/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "dangerous to operate recursively")
}

func TestRmVerbose(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "rm", "-v", "file.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "removed")
}

func TestRmMissingOperand(t *testing.T) {
	r := runBuiltin(t, "rm")
	assertExitCode(t, r, 1)
}
