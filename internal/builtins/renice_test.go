//go:build unix

package builtins

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/AmadeusDE/Misa/internal/proceffect"
)

// spawnNiceFixture starts a real child process to renice and returns its pid.
func spawnNiceFixture(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(u29HelperPath(t), "--process-wrapper-test-helper", "sleep")
	// Its own process group, so -g tests can target it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn fixture: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	deadline := 50
	for niceOf(cmd.Process.Pid) < 0 && deadline > 0 {
		deadline--
	}
	if nice := niceOf(cmd.Process.Pid); nice < 0 {
		t.Fatalf("no nice value for fixture %d", cmd.Process.Pid)
	}
	return cmd.Process.Pid
}

func runReniceArgs(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), "renice", &Context{
		Dir:    t.TempDir(),
		Args:   args,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	return status, stdout.String(), stderr.String()
}

func TestReniceAdjustsTargetProcess(t *testing.T) {
	pid := spawnNiceFixture(t)
	before := niceOf(pid)
	callerBefore, err := proceffect.Niceness()
	if err != nil {
		t.Skip(err)
	}
	if status, _, stderr := runReniceArgs(t, "-n", "5", "-p", strconv.Itoa(pid)); status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr)
	}
	if got, want := niceOf(pid), before+5; got != want {
		t.Fatalf("nice = %d, want %d", got, want)
	}
	// Renice adds to the current value, like toybox.
	if status, _, stderr := runReniceArgs(t, "-n", "3", strconv.Itoa(pid)); status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr)
	}
	if got, want := niceOf(pid), before+8; got != want {
		t.Fatalf("nice = %d, want %d", got, want)
	}
	if after, err := proceffect.Niceness(); err != nil || after != callerBefore {
		t.Fatalf("caller nice %d -> %d (err %v)", callerBefore, after, err)
	}
}

func TestReniceClampsToRange(t *testing.T) {
	pid := spawnNiceFixture(t)
	if status, _, stderr := runReniceArgs(t, "-n", "100", strconv.Itoa(pid)); status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr)
	}
	if got := niceOf(pid); got != 19 {
		t.Fatalf("nice = %d, want the maximum 19", got)
	}
}

func TestReniceProcessGroup(t *testing.T) {
	pid := spawnNiceFixture(t)
	before := niceOf(pid)
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatal(err)
	}
	if status, _, stderr := runReniceArgs(t, "-n", "4", "-g", strconv.Itoa(pgid)); status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr)
	}
	if got, want := niceOf(pid), before+4; got != want {
		t.Fatalf("nice = %d, want %d", got, want)
	}
}

func TestRenicePermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may lower priorities")
	}
	pid := spawnNiceFixture(t)
	before := niceOf(pid)
	status, _, stderr := runReniceArgs(t, "-n", "-3", strconv.Itoa(pid))
	if status != 1 {
		t.Fatalf("status=%d, want 1", status)
	}
	if !strings.Contains(stderr, "Permission denied") {
		t.Fatalf("stderr = %q", stderr)
	}
	if got := niceOf(pid); got != before {
		t.Fatalf("nice = %d, want unchanged %d", got, before)
	}
}

func TestReniceBadTargets(t *testing.T) {
	pid := spawnNiceFixture(t)
	before := niceOf(pid)
	// A bad operand and a missing process both fail, but the good target still
	// changes: errors per target, non-zero status overall.
	status, _, stderr := runReniceArgs(t, "-n", "2", strconv.Itoa(pid), "not-a-pid", "4194304")
	if status != 1 {
		t.Fatalf("status=%d, want 1", status)
	}
	if !strings.Contains(stderr, "renice: bad 'not-a-pid'") {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stderr, "renice: setpriority 4194304: No such process") {
		t.Fatalf("stderr = %q", stderr)
	}
	if got, want := niceOf(pid), before+2; got != want {
		t.Fatalf("nice = %d, want %d", got, want)
	}
}

func TestReniceUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no increment", []string{"123"}, "missing priority increment"},
		{"bad increment", []string{"-n", "x", "123"}, "invalid increment 'x'"},
		{"no operand", []string{"-n", "1"}, "missing operand"},
	} {
		status, _, stderr := runReniceArgs(t, tc.args...)
		if status != 1 || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%s: status=%d stderr=%q", tc.name, status, stderr)
		}
	}
}

func TestReniceHelpAndVersion(t *testing.T) {
	status, stdout, stderr := runReniceArgs(t, "--help")
	if status != 0 || !strings.HasPrefix(stdout, "Usage: renice ") {
		t.Fatalf("--help: status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, stdout, _ = runReniceArgs(t, "--version")
	if status != 0 || !strings.Contains(stdout, "renice (Misa)") {
		t.Fatalf("--version: status=%d stdout=%q", status, stdout)
	}
}

func TestReniceTargetResolution(t *testing.T) {
	uid := os.Getuid()
	for _, tc := range []struct {
		operand string
		which   int
		want    int
		ok      bool
	}{
		{strconv.Itoa(uid), syscall.PRIO_USER, uid, true},
		{"misa-user-that-does-not-exist-27934", syscall.PRIO_USER, 0, false},
		{"<current-user>", syscall.PRIO_USER, uid, true},
		{"1234", syscall.PRIO_PROCESS, 1234, true},
		{"-1", syscall.PRIO_PROCESS, 0, false},
		{"12x", syscall.PRIO_PROCESS, 0, false},
		{"0", syscall.PRIO_PGRP, 0, true},
	} {
		if tc.operand == "<current-user>" {
			tc.operand = currentUserName(t)
		}
		got, ok := reniceTarget(tc.operand, tc.which)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%q/%d = %d,%v want %d,%v", tc.operand, tc.which, got, ok, tc.want, tc.ok)
		}
	}
}

// currentUserName resolves the account running the tests, for the -u name path.
func currentUserName(t *testing.T) string {
	t.Helper()
	current, err := user.Current()
	if err != nil || current.Username == "" {
		t.Skipf("cannot resolve the current user: %v", err)
	}
	return current.Username
}
