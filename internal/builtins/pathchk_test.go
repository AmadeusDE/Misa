package builtins

import (
	"strings"
	"testing"
)

func TestPathchkRegistered(t *testing.T) {
	_, ok := Lookup("pathchk")
	if !ok {
		t.Fatal("pathchk not registered")
	}
}

func TestPathchkValid(t *testing.T) {
	r := runBuiltin(t, "pathchk", "-p", "valid_path")
	assertExitCode(t, r, 0)
}

func TestPathchkNonportable(t *testing.T) {
	r := runBuiltin(t, "pathchk", "-p", "path with spaces")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "nonportable") {
		t.Errorf("expected nonportable error, got %q", r.stderr)
	}
}

func TestPathchkLeadingHyphen(t *testing.T) {
	r := runBuiltin(t, "pathchk", "-P", "--", "-badname")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "leading hyphen") {
		t.Errorf("expected leading hyphen error, got %q", r.stderr)
	}
}

func TestPathchkEmpty(t *testing.T) {
	r := runBuiltin(t, "pathchk", "")
	assertExitCode(t, r, 1)
}

func TestPathchkMissing(t *testing.T) {
	r := runBuiltin(t, "pathchk")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "the following required arguments were not provided")
}
