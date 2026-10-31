package native

import (
	"runtime"
	"strings"
	"testing"
)

func TestFreeListsMemory(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("free needs procfs")
	}
	r := run(t, "free")
	if r.code != 0 {
		t.Fatalf("free: exit %d, stderr %q", r.code, r.stderr)
	}
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("free printed %d lines, want 3:\n%s", len(lines), r.stdout)
	}
	for _, col := range []string{"total", "used", "free", "shared", "buff/cache", "available"} {
		if !strings.Contains(lines[0], col) {
			t.Errorf("header %q is missing %q", lines[0], col)
		}
	}
	if !strings.HasPrefix(lines[1], "Mem:") || !strings.HasPrefix(lines[2], "Swap:") {
		t.Errorf("rows = %q / %q, want Mem: then Swap:", lines[1], lines[2])
	}
}

func TestFreeTotalWideUnits(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("free needs procfs")
	}
	r := run(t, "free", "-w", "-t")
	if r.code != 0 {
		t.Fatalf("free -w -t: exit %d, stderr %q", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "buffers") || !strings.HasPrefix(strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")[3], "Total:") {
		t.Errorf("free -w -t output:\n%s", r.stdout)
	}

	// A human-readable row uses a scaled suffix, not a bare byte count.
	r = run(t, "free", "-h")
	if r.code != 0 {
		t.Fatalf("free -h: exit %d, stderr %q", r.code, r.stderr)
	}
	if !strings.ContainsAny(r.stdout, "KMGTP") {
		t.Errorf("free -h output has no scaled unit:\n%s", r.stdout)
	}
}

func TestFreeRejectsUsageErrors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("free needs procfs")
	}
	for _, args := range [][]string{
		{"-l"}, {"-v"}, {"-L"}, {"--kilo"}, {"-c", "0"}, {"-s", "0"}, {"-k", "-g"},
	} {
		r := run(t, "free", args...)
		if r.code != 1 {
			t.Errorf("%v: exit %d, want 1", args, r.code)
		}
		if r.stdout != "" {
			t.Errorf("%v: stdout %q, want none", args, r.stdout)
		}
		if !strings.Contains(r.stderr, "free:") {
			t.Errorf("%v: stderr %q", args, r.stderr)
		}
	}
}
