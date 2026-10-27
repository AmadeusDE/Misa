package builtins

import (
	"strings"
	"testing"
)

func TestHostname(t *testing.T) {
	r := runBuiltin(t, "hostname")
	assertExitCode(t, r, 0)
	out := strings.TrimSpace(r.stdout)
	if out == "" {
		t.Error("hostname: expected non-empty output")
	}
}
