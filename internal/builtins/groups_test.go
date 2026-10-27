package builtins

import (
	"strings"
	"testing"
)

func TestGroupsRegistered(t *testing.T) {
	_, ok := Lookup("groups")
	if !ok {
		t.Fatal("groups not registered")
	}
}

func TestGroupsOutput(t *testing.T) {
	r := runBuiltin(t, "groups")
	assertExitCode(t, r, 0)
	if strings.TrimSpace(r.stdout) == "" {
		t.Error("expected non-empty group list")
	}
}

func TestGroupsUnknownUser(t *testing.T) {
	r := runBuiltin(t, "groups", "nonexistentuser12345")
	assertExitCode(t, r, 1)
	if r.stderr == "" {
		t.Error("expected error on stderr")
	}
}

func TestGroupsDoubleHyphen(t *testing.T) {
	r := runBuiltin(t, "groups", "--")
	assertExitCode(t, r, 0)
	if strings.TrimSpace(r.stdout) == "" {
		t.Error("expected non-empty group list")
	}
}

func TestGroupsVersion(t *testing.T) {
	r := runBuiltin(t, "groups", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Misa")
}
