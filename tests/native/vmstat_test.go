package native

import (
	"runtime"
	"strings"
	"testing"
)

func TestVmstatBootLine(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("vmstat needs procfs")
	}
	r := run(t, "vmstat")
	if r.code != 0 {
		t.Fatalf("vmstat: exit %d, stderr %q", r.code, r.stderr)
	}
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("vmstat printed %d lines, want 3:\n%s", len(lines), r.stdout)
	}
	if !strings.HasPrefix(lines[0], "procs ") {
		t.Errorf("group header = %q", lines[0])
	}
	cols := strings.Fields(lines[1])
	if got, want := strings.Join(cols, ","), "r,b,swpd,free,buff,cache,si,so,bi,bo,in,cs,us,sy,id,wa"; got != want {
		t.Errorf("columns = %s, want %s", got, want)
	}
	if got := len(strings.Fields(lines[2])); got != len(cols) {
		t.Errorf("sample has %d fields, want %d:\n%s", got, len(cols), lines[2])
	}
}

func TestVmstatOptions(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("vmstat needs procfs")
	}
	r := run(t, "vmstat", "-a")
	if r.code != 0 || !strings.Contains(r.stdout, "inact") || !strings.Contains(r.stdout, "active") {
		t.Errorf("vmstat -a: exit %d, stdout %q", r.code, r.stdout)
	}
	r = run(t, "vmstat", "-S", "m")
	if r.code != 0 {
		t.Errorf("vmstat -S m: exit %d, stderr %q", r.code, r.stderr)
	}
	// DELAY COUNT takes one sample and never sleeps.
	r = run(t, "vmstat", "1", "1")
	if r.code != 0 || strings.Count(r.stdout, "\n") != 3 {
		t.Errorf("vmstat 1 1: exit %d, stdout %q", r.code, r.stdout)
	}
}

func TestVmstatRejectsUnsupportedOptions(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("vmstat needs procfs")
	}
	cases := [][]string{
		{"-w"}, {"-t"}, {"-y"}, {"-s"}, {"-d"}, {"-D"}, {"-p"}, {"-f"}, {"-S", "x"},
		{"1", "2", "3"}, {"1", "0"},
	}
	for _, args := range cases {
		r := run(t, "vmstat", args...)
		if r.code != 1 {
			t.Errorf("%v: exit %d, want 1", args, r.code)
		}
		if r.stdout != "" {
			t.Errorf("%v: stdout %q, want none", args, r.stdout)
		}
	}
}
