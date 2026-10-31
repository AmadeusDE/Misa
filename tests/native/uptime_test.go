package native

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestUptimeLine(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uptime needs procfs")
	}
	r := run(t, "uptime")
	if r.code != 0 {
		t.Fatalf("uptime: exit %d, stderr %q", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, " up ") {
		t.Errorf("uptime output %q has no ' up '", r.stdout)
	}
	if strings.Count(r.stdout, "\n") != 1 {
		t.Errorf("uptime output %q, want one line", r.stdout)
	}
}

func TestUptimePrettyAndSince(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uptime needs procfs")
	}
	r := run(t, "uptime", "-p")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "up ") {
		t.Errorf("uptime -p: exit %d, stdout %q", r.code, r.stdout)
	}

	r = run(t, "uptime", "-s")
	if r.code != 0 {
		t.Fatalf("uptime -s: exit %d, stderr %q", r.code, r.stderr)
	}
	stamp := strings.TrimSpace(r.stdout)
	if _, err := time.Parse("2006-01-02 15:04:05", stamp); err != nil {
		t.Errorf("uptime -s printed %q, want a YYYY-MM-DD hh:mm:ss stamp", stamp)
	}
}

func TestUptimeRejectsOperands(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uptime needs procfs")
	}
	r := run(t, "uptime", "extra")
	if r.code != 1 || !strings.Contains(r.stderr, "extra operand") {
		t.Errorf("exit %d, stderr %q", r.code, r.stderr)
	}
}
