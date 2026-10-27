package native

import (
	"strings"
	"testing"
)

func TestHostname_Default(t *testing.T) {
	r := run(t, "hostname")
	wantSuccess(t, r)
	if strings.TrimSpace(r.stdout) == "" {
		t.Errorf("expected non-empty hostname output")
	}
}

func TestHostname_Short(t *testing.T) {
	r := run(t, "hostname", "-s")
	wantSuccess(t, r)
	out := strings.TrimSpace(r.stdout)
	if strings.Contains(out, ".") {
		t.Errorf("short hostname should not contain dot, got %q", out)
	}
}

func TestHostname_Domain(t *testing.T) {
	r := run(t, "hostname", "-d")
	wantSuccess(t, r)
}

func TestHostname_IP(t *testing.T) {
	r := run(t, "hostname", "-i")
	wantSuccess(t, r)
	if strings.TrimSpace(r.stdout) == "" {
		t.Errorf("expected non-empty IP address output")
	}
}

func TestHostname_InvalidOption(t *testing.T) {
	r := run(t, "hostname", "--definitely-invalid")
	wantFail(t, r)
}
