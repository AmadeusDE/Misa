package builtins

import (
	"path/filepath"
	"testing"
)

func TestPwd(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "pwd")
	assertExitCode(t, r, 0)
	phys, err := filepath.EvalSymlinks(dir) // pwd is physical by default
	if err != nil {
		t.Fatal(err)
	}
	assertStdout(t, r, phys+"\n")
}
