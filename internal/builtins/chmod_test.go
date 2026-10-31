package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChmodOctal(t *testing.T) {
	skipOnWindows(t, "Windows keeps no permission bits beyond read-only (windowsLimits chmod)")
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "chmod", "755", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(filepath.Join(dir, "file.txt"))
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode = %o, want 755", info.Mode().Perm())
	}
}

func TestChmodSymbolicAdd(t *testing.T) {
	skipOnWindows(t, "Windows keeps no permission bits beyond read-only (windowsLimits chmod)")
	dir := t.TempDir()
	p := writeFile(t, dir, "file.txt", "data")
	os.Chmod(p, 0644)
	r := runBuiltinIn(t, dir, "chmod", "u+x", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0744 {
		t.Errorf("mode = %o, want 744", info.Mode().Perm())
	}
}

func TestChmodSymbolicRemove(t *testing.T) {
	skipOnWindows(t, "Windows keeps no permission bits beyond read-only (windowsLimits chmod)")
	dir := t.TempDir()
	p := writeFile(t, dir, "file.txt", "data")
	os.Chmod(p, 0755)
	r := runBuiltinIn(t, dir, "chmod", "go-wx", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0744 {
		t.Errorf("mode = %o, want 744", info.Mode().Perm())
	}
}

func TestChmodSymbolicSet(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "file.txt", "data")
	os.Chmod(p, 0777)
	r := runBuiltinIn(t, dir, "chmod", "a=r", "file.txt")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0444 {
		t.Errorf("mode = %o, want 444", info.Mode().Perm())
	}
}

func TestChmodRecursive(t *testing.T) {
	skipOnWindows(t, "Windows keeps no permission bits beyond read-only (windowsLimits chmod)")
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	p := writeFile(t, dir, "sub/file.txt", "data")
	r := runBuiltinIn(t, dir, "chmod", "-R", "777", "sub")
	assertExitCode(t, r, 0)
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0777 {
		t.Errorf("mode = %o, want 777", info.Mode().Perm())
	}
}

func TestChmodVerbose(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.txt", "data")
	r := runBuiltinIn(t, dir, "chmod", "-v", "644", "file.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "mode of")
}

func TestChmodMissingOperand(t *testing.T) {
	r := runBuiltin(t, "chmod", "755")
	assertExitCode(t, r, 1)
}
