package builtins

import (
	"strings"
	"testing"
)

func TestWhoami(t *testing.T) {
	r := runBuiltin(t, "whoami")
	assertExitCode(t, r, 0)
	out := strings.TrimSpace(r.stdout)
	if out == "" {
		t.Error("whoami: expected non-empty username")
	}
}
