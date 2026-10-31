package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWhichFindsExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mytool"+exeSuffix)
	os.WriteFile(bin, []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := runBuiltin(t, "which", "mytool")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "mytool")
}

func TestWhichNotFound(t *testing.T) {
	r := runBuiltin(t, "which", "nonexistent_binary_12345")
	assertExitCode(t, r, 1)
}

func TestWhichAll(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "dup"+exeSuffix), []byte(""), 0755)
	os.WriteFile(filepath.Join(dir2, "dup"+exeSuffix), []byte(""), 0755)
	t.Setenv("PATH", dir1+string(os.PathListSeparator)+dir2)
	r := runBuiltin(t, "which", "-a", "dup")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) < 2 {
		t.Errorf("expected 2 results, got %d", len(lines))
	}
}

func TestWhichMissingOperand(t *testing.T) {
	r := runBuiltin(t, "which")
	assertExitCode(t, r, 1)
}

func TestUnameDefault(t *testing.T) {
	r := runBuiltin(t, "uname")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Linux")
}

func TestUnameAll(t *testing.T) {
	r := runBuiltin(t, "uname", "-a")
	assertExitCode(t, r, 0)
	out := r.stdout
	if !strings.Contains(out, "Linux") {
		t.Errorf("expected Linux in output, got %q", out)
	}
	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) < 4 {
		t.Errorf("expected multiple fields in -a output, got %d", len(parts))
	}
}

func TestUnameMachine(t *testing.T) {
	r := runBuiltin(t, "uname", "-m")
	assertExitCode(t, r, 0)
	m := strings.TrimSpace(r.stdout)
	if m == "" {
		t.Error("expected non-empty machine name")
	}
}

func TestUnameNodename(t *testing.T) {
	r := runBuiltin(t, "uname", "-n")
	assertExitCode(t, r, 0)
	if strings.TrimSpace(r.stdout) == "" {
		t.Error("expected non-empty nodename")
	}
}
