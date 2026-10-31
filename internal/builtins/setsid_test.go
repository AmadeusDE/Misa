//go:build unix

package builtins

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// ownIdent returns this process's process group and session ids.
func ownIdent(t *testing.T) (pgid, sid int) {
	t.Helper()
	pgid, err := unix.Getpgid(0)
	if err != nil {
		t.Fatal(err)
	}
	sid, err = unix.Getsid(0)
	if err != nil {
		t.Fatal(err)
	}
	return pgid, sid
}

// parseIdent parses the "pid=… pgid=… sid=… nice=…" line of the wrapper helper.
func parseIdent(t *testing.T, out string) map[string]int {
	t.Helper()
	fields := map[string]int{}
	for _, f := range strings.Fields(out) {
		key, value, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("unexpected ident field %q", f)
		}
		fields[key] = n
	}
	if _, ok := fields["pid"]; !ok {
		t.Fatalf("no ident in %q", out)
	}
	return fields
}

func TestSetsidNewSession(t *testing.T) {
	pgid, sid := ownIdent(t)
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), "setsid", &Context{
		Dir:    t.TempDir(),
		Args:   []string{u29HelperPath(t), "--process-wrapper-test-helper", "ids"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	if status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	ident := parseIdent(t, stdout.String())
	if ident["pid"] != ident["sid"] || ident["pid"] != ident["pgid"] {
		t.Fatalf("child ident = %v, want pid == sid == pgid", ident)
	}
	if ident["sid"] == sid || ident["pgid"] == pgid {
		t.Fatalf("child ident = %v did not leave the caller's session %d group %d", ident, sid, pgid)
	}
	if after, sidAfter := ownIdent(t); after != pgid || sidAfter != sid {
		t.Fatalf("caller group/session changed: %d/%d -> %d/%d", pgid, sid, after, sidAfter)
	}
}

// TestSetsidControllingTerminal covers -c: only that flag makes the new
// session's stdin its controlling terminal.
func TestSetsidControllingTerminal(t *testing.T) {
	_, tty := openTestPTY(t)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"default", []string{u29HelperPath(t), "--process-wrapper-test-helper", "ctty"}, "no-ctty"},
		{"-c", []string{"-c", u29HelperPath(t), "--process-wrapper-test-helper", "ctty"}, "ctty"},
		{"-d", []string{"-d", u29HelperPath(t), "--process-wrapper-test-helper", "ctty"}, "no-ctty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			status := Run(context.Background(), "setsid", &Context{
				Dir: t.TempDir(), Args: tc.args,
				Stdout: &stdout, Stderr: &stderr, Stdin: tty,
			})
			if status != 0 {
				t.Fatalf("status=%d stderr=%q", status, stderr.String())
			}
			if got := strings.TrimSpace(stdout.String()); got != tc.want {
				t.Fatalf("stdout = %q, want %q (stderr %q)", got, tc.want, stderr.String())
			}
		})
	}
}

func TestSetsidForwardsStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		argv []string
		want int
	}{
		{"external", []string{u29HelperPath(t), "--process-wrapper-test-helper", "exit", "7"}, 7},
		{"builtin", []string{"false"}, 1},
		{"missing", []string{"misa-missing-command-27934"}, 127},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			status := Run(context.Background(), "setsid", &Context{
				Dir: t.TempDir(), Args: tc.argv,
				Stdout: &bytes.Buffer{}, Stderr: &stderr, Stdin: strings.NewReader(""),
			})
			if status != tc.want {
				t.Fatalf("status=%d, want %d (stderr %q)", status, tc.want, stderr.String())
			}
			if tc.name == "missing" && !strings.Contains(stderr.String(), "setsid: failed to run command") {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestSetsidBuiltinDispatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), "setsid", &Context{
		Dir:    t.TempDir(),
		Args:   []string{"echo", "detached"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	if status != 0 || stdout.String() != "detached\n" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func TestSetsidUsageErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := Run(context.Background(), "setsid", &Context{Dir: t.TempDir(), Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}); status != 1 {
		t.Fatalf("missing operand status=%d stderr=%q", status, stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing operand") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	stderr.Reset()
	if status := Run(context.Background(), "setsid", &Context{Dir: t.TempDir(), Args: []string{"-c", "-d", "true"}, Stdout: &stdout, Stderr: &stderr}); status != 1 {
		t.Fatalf("-c -d status=%d", status)
	}
	if !strings.Contains(stderr.String(), "-c and -d are mutually exclusive") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	stderr.Reset()
	if status := Run(context.Background(), "setsid", &Context{Dir: t.TempDir(), Args: []string{"-Z", "true"}, Stdout: &stdout, Stderr: &stderr}); status != 1 {
		t.Fatalf("unknown option status=%d", status)
	}
	if !strings.Contains(stderr.String(), "unknown option -Z") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	stdout.Reset()
	if status := Run(context.Background(), "setsid", &Context{Dir: t.TempDir(), Args: []string{"--help"}, Stdout: &stdout, Stderr: &stderr}); status != 0 {
		t.Fatalf("--help status=%d", status)
	}
	if !strings.HasPrefix(stdout.String(), "Usage: setsid ") {
		t.Fatalf("--help stdout = %q", stdout.String())
	}
}

func TestSetpgidNewGroup(t *testing.T) {
	pgid, sid := ownIdent(t)
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), "setpgid", &Context{
		Dir:    t.TempDir(),
		Args:   []string{u29HelperPath(t), "--process-wrapper-test-helper", "ids"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	if status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	ident := parseIdent(t, stdout.String())
	if ident["pgid"] != ident["pid"] {
		t.Fatalf("child ident = %v, want its own group", ident)
	}
	if ident["sid"] != sid {
		t.Fatalf("child session = %d, want the caller's %d", ident["sid"], sid)
	}
	if after, sidAfter := ownIdent(t); after != pgid || sidAfter != sid {
		t.Fatalf("caller group/session changed: %d/%d -> %d/%d", pgid, sid, after, sidAfter)
	}
}

func TestSetpgidStatusAndUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, tc := range []struct {
		name string
		argv []string
		want int
	}{
		{"status", []string{u29HelperPath(t), "--process-wrapper-test-helper", "exit", "9"}, 9},
		{"builtin", []string{"false"}, 1},
		{"missing", []string{"misa-missing-command-27934"}, 127},
	} {
		status := Run(context.Background(), "setpgid", &Context{
			Dir: t.TempDir(), Args: tc.argv,
			Stdout: &bytes.Buffer{}, Stderr: &stderr, Stdin: strings.NewReader(""),
		})
		if status != tc.want {
			t.Fatalf("%s: status=%d, want %d (stderr %q)", tc.name, status, tc.want, stderr.String())
		}
		if tc.name == "missing" && !strings.Contains(stderr.String(), "setpgid: failed to run command") {
			t.Fatalf("stderr = %q", stderr.String())
		}
		stderr.Reset()
	}
	if status := Run(context.Background(), "setpgid", &Context{Dir: t.TempDir(), Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")}); status != ExitInternal {
		t.Fatalf("missing operand status=%d", status)
	}
	if !strings.Contains(stderr.String(), "missing operand") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	stderr.Reset()
	if status := Run(context.Background(), "setpgid", &Context{Dir: t.TempDir(), Args: []string{"--", "echo", "after"}, Stdout: &stdout, Stderr: &stderr}); status != 0 {
		t.Fatalf("-- status=%d stderr=%q", status, stderr.String())
	}
	if stdout.String() != "after\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	stdout.Reset()
	if status := Run(context.Background(), "setpgid", &Context{Dir: t.TempDir(), Args: []string{"--help"}, Stdout: &stdout, Stderr: &stderr}); status != 0 {
		t.Fatalf("--help status=%d", status)
	}
	if !strings.HasPrefix(stdout.String(), "Usage: setpgid ") {
		t.Fatalf("--help stdout = %q", stdout.String())
	}
}

// TestProcessWrapperLeavesCallerPriority proves none of the wrappers reniced
// the process that ran them.
func TestProcessWrapperLeavesCallerPriority(t *testing.T) {
	before := niceOf(os.Getpid())
	if before < 0 {
		t.Skip("cannot read own nicer value")
	}
	for _, name := range []string{"setsid", "setpgid", "nohup"} {
		stderr := &bytes.Buffer{}
		if status := Run(context.Background(), name, &Context{
			Dir: t.TempDir(), Args: []string{"true"},
			Stdout: &bytes.Buffer{}, Stderr: stderr, Stdin: strings.NewReader(""),
		}); status != 0 {
			t.Fatalf("%s status=%d stderr=%q", name, status, stderr.String())
		}
		if after := niceOf(os.Getpid()); after != before {
			t.Fatalf("%s changed the caller's nice value %d -> %d", name, before, after)
		}
	}
}
