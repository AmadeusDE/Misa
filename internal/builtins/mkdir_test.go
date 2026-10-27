package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMkdirBasic(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "mkdir", "newdir")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "newdir"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("not a directory")
	}
}

func TestMkdirParents(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "mkdir", "-p", "a/b/c")
	assertExitCode(t, r, 0)
	if _, err := os.Stat(filepath.Join(dir, "a/b/c")); err != nil {
		t.Error("nested dirs not created")
	}
}

func TestMkdirExistingNoParents(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "existing"), 0755)
	r := runBuiltinIn(t, dir, "mkdir", "existing")
	assertExitCode(t, r, 1)
}

func TestMkdirVerbose(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "mkdir", "-v", "mydir")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "created directory")
}

func TestMkdirMissingOperand(t *testing.T) {
	r := runBuiltin(t, "mkdir")
	assertExitCode(t, r, 1)
}

func TestMkdirMode(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "mkdir", "-m", "700", "restricted")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "restricted"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("mode = %o, want 700", info.Mode().Perm())
	}
}

func TestMkdirEmptyArg(t *testing.T) {
	r := runBuiltin(t, "mkdir", "")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot create directory ''")
}

func TestMkdirInexistentDir(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "mkdir", "a/b")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot create directory 'a/b'")
	assertStderrContains(t, r, "No such file or directory")
}
