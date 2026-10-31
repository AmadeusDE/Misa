package native

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPsListsOwnProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("no procfs backend on this platform")
	}
	r := run(t, "ps", "-A", "-o", "pid,args")
	if r.code != 0 {
		t.Fatalf("ps -A: exit %d, stderr %q", r.code, r.stderr)
	}
	self := strconv.Itoa(os.Getpid())
	for _, line := range strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")[1:] {
		if strings.HasPrefix(strings.TrimSpace(line), self+" ") {
			return
		}
	}
	t.Fatalf("pid %s missing from ps -A output:\n%s", self, r.stdout)
}

func TestPsRejectsUnsupportedOptions(t *testing.T) {
	for _, args := range [][]string{{"-Z"}, {"-T"}, {"-M"}, {"-O", "pid"}} {
		r := run(t, "ps", args...)
		if r.code != 1 {
			t.Errorf("%v: exit %d, want 1", args, r.code)
		}
		if r.stdout != "" {
			t.Errorf("%v: stdout %q, want none", args, r.stdout)
		}
		if !strings.Contains(r.stderr, "unknown option") {
			t.Errorf("%v: stderr %q", args, r.stderr)
		}
	}
}

func TestPsFieldSelectionRejectsUnknownField(t *testing.T) {
	r := run(t, "ps", "-o", "wchan")
	if r.code != 1 || !strings.Contains(r.stderr, "unknown field 'wchan'") {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
}

func TestPsHelpListsUnsupportedOptions(t *testing.T) {
	r := run(t, "ps", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "Unsupported (rejected") {
		t.Fatalf("exit %d, stdout %q", r.code, r.stdout)
	}
}
