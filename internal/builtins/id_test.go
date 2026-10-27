package builtins

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestIdDefault(t *testing.T) {
	r := runBuiltin(t, "id")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "uid=")
	assertStdoutContains(t, r, "gid=")
}

func TestIdUser(t *testing.T) {
	r := runBuiltin(t, "id", "-u")
	assertExitCode(t, r, 0)
	s := strings.TrimSpace(r.stdout)
	if _, err := strconv.Atoi(s); err != nil {
		t.Errorf("expected numeric uid, got %q", s)
	}
}

func TestIdUserName(t *testing.T) {
	r := runBuiltin(t, "id", "-u", "-n")
	assertExitCode(t, r, 0)
	name := strings.TrimSpace(r.stdout)
	if len(name) == 0 {
		t.Error("expected non-empty username")
	}
	if regexp.MustCompile(`^\d+$`).MatchString(name) {
		t.Skip("could not resolve username, only got numeric")
	}
}

func TestIdGroup(t *testing.T) {
	r := runBuiltin(t, "id", "-g")
	assertExitCode(t, r, 0)
	s := strings.TrimSpace(r.stdout)
	if _, err := strconv.Atoi(s); err != nil {
		t.Errorf("expected numeric gid, got %q", s)
	}
}

func TestIdGroups(t *testing.T) {
	r := runBuiltin(t, "id", "-G")
	assertExitCode(t, r, 0)
	parts := strings.Fields(strings.TrimSpace(r.stdout))
	if len(parts) == 0 {
		t.Error("expected at least one group")
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			t.Errorf("expected numeric group, got %q", p)
		}
	}
}

func TestIdGroupsNames(t *testing.T) {
	r := runBuiltin(t, "id", "-G", "-n")
	assertExitCode(t, r, 0)
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected non-empty groups output")
	}
}

func TestIdZero(t *testing.T) {
	r := runBuiltin(t, "id", "-u", "-z")
	assertExitCode(t, r, 0)
	if !strings.HasSuffix(r.stdout, "\x00") {
		t.Error("expected output to be null-terminated")
	}
}
