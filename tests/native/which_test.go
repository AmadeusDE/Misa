package native

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWhich_Found(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "mytool")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	r := run(t, "which", "mytool")
	wantSuccess(t, r)
	wantOut(t, r, exe+"\n")
}

func TestWhich_NotFoundExit1(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r := run(t, "which", "nonexistent_binary_99999")
	wantCode(t, r, 1)
}

func TestWhich_SlashPath(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "mytool")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0"), 0755); err != nil {
		t.Fatal(err)
	}

	r := run(t, "which", exe)
	wantSuccess(t, r)
	wantOut(t, r, exe+"\n")
}
