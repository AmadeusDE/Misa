package builtins

import (
	"strconv"
	"strings"
	"testing"
)

func TestNproc(t *testing.T) {
	r := runBuiltin(t, "nproc")
	assertExitCode(t, r, 0)
	n, err := strconv.Atoi(strings.TrimSpace(r.stdout))
	if err != nil {
		t.Fatalf("nproc output not a number: %q", r.stdout)
	}
	if n < 1 {
		t.Errorf("nproc = %d, want >= 1", n)
	}
}

func TestNprocAll(t *testing.T) {
	r := runBuiltin(t, "nproc", "--all")
	assertExitCode(t, r, 0)
	n, err := strconv.Atoi(strings.TrimSpace(r.stdout))
	if err != nil {
		t.Fatalf("nproc --all output not a number: %q", r.stdout)
	}
	if n < 1 {
		t.Errorf("nproc --all = %d, want >= 1", n)
	}
}

func TestNprocIgnore(t *testing.T) {
	r := runBuiltin(t, "nproc", "--ignore=999")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}
