package builtins

import (
	"strings"
	"testing"
)

func TestWindowsLimitsNameRegisteredCommands(t *testing.T) {
	seen := map[[2]string]bool{}
	for _, l := range windowsLimits {
		if _, ok := Lookup(l.Command); !ok && l.Command != "sync" { // sync exists on Linux and Windows only
			t.Errorf("limit for %q: command is not registered", l.Command)
		}
		key := [2]string{l.Command, l.Option}
		if seen[key] {
			t.Errorf("duplicate limit %v", key)
		}
		seen[key] = true
		if strings.TrimSpace(l.Reason) == "" {
			t.Errorf("limit %v has no reason", key)
		}
		if l.Kind != Unsupported && l.Kind != Partial {
			t.Errorf("limit %v has kind %q", key, l.Kind)
		}
	}
}

func TestWindowsLimitsCoverTheUnixOnlyOperations(t *testing.T) {
	for _, want := range [][2]string{{"chown", ""}, {"mkfifo", ""}, {"stat", "-f"}, {"chmod", ""}} {
		l, ok := findLimit(windowsLimits, want[0], want[1])
		if !ok {
			t.Errorf("no Windows limit recorded for %v", want)
			continue
		}
		if want[1] == "" && want[0] != "chmod" && l.Kind != Unsupported {
			t.Errorf("%v must be unsupported, not %s", want, l.Kind)
		}
	}
}

func TestFindLimitMatchesCommandAndOption(t *testing.T) {
	table := []Limit{{"x", "", Unsupported, "a"}, {"x", "-o", Partial, "b"}}
	if l, ok := findLimit(table, "x", "-o"); !ok || l.Reason != "b" {
		t.Errorf("findLimit(x, -o) = %v, %v", l, ok)
	}
	if _, ok := findLimit(table, "y", ""); ok {
		t.Error("findLimit found a command that is not in the table")
	}
}

func TestFailUnsupportedNamesCommandOptionAndReason(t *testing.T) {
	var out, errb strings.Builder
	ctx := &Context{Stdout: &out, Stderr: &errb}
	status := failUnsupported(ctx, Limit{"stat", "-f", Unsupported, "no statfs"})
	if status != 1 {
		t.Errorf("status = %d, want 1", status)
	}
	for _, want := range []string{"stat:", "stat -f", "no statfs"} {
		if !strings.Contains(errb.String(), want) {
			t.Errorf("stderr %q lacks %q", errb.String(), want)
		}
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestMachineFromGOARCH(t *testing.T) {
	for goarch, want := range map[string]string{"amd64": "x86_64", "386": "i686", "arm64": "aarch64", "arm": "armv7l", "riscv64": "riscv64"} {
		if got := machineFromGOARCH(goarch); got != want {
			t.Errorf("machineFromGOARCH(%q) = %q, want %q", goarch, got, want)
		}
	}
}
