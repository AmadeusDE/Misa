//go:build unix

package native

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
)

// The toybox renice.test cases cannot be represented by the shell adapter: they
// drive bash loops, background jobs and /proc nice readings. These cases cover
// the same ground natively.

func spawnTarget(t *testing.T) int {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "--process-target-helper", "sleep")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn fixture: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return cmd.Process.Pid
}

func TestReniceMultipleTargetsPartialFailure(t *testing.T) {
	first, second := spawnTarget(t), spawnTarget(t)
	firstNice, secondNice := targetNice(first), targetNice(second)
	args := []string{"-n", "1", "-p", strconv.Itoa(first), "4194304", strconv.Itoa(second)}
	r := run(t, "renice", args...)
	// toybox prints one message per failed target; Misa also reports the
	// failure through the exit status.
	wantFail(t, r)
	if r.code != 1 {
		t.Errorf("exit code = %d, want 1", r.code)
	}
	wantErrContains(t, r, "renice: setpriority 4194304: ")
	if got := targetNice(first); got != firstNice+1 {
		t.Errorf("first nice = %d, want %d", got, firstNice+1)
	}
	if got := targetNice(second); got != secondNice+1 {
		t.Errorf("second nice = %d, want %d", got, secondNice+1)
	}
}

func TestReniceBoundsAtMaximum(t *testing.T) {
	pid := spawnTarget(t)
	// Walk up to the maximum; further increments stay there and stay successful.
	for i := 0; i < 40; i++ {
		r := run(t, "renice", "-n", "1", strconv.Itoa(pid))
		wantSuccess(t, r)
	}
	if got := targetNice(pid); got != 19 {
		t.Fatalf("nice = %d, want 19", got)
	}
	r := run(t, "renice", "-n", "5", strconv.Itoa(pid))
	wantSuccess(t, r)
	if got := targetNice(pid); got != 19 {
		t.Fatalf("nice = %d after another increment, want 19", got)
	}
}

func TestReniceGroupTarget(t *testing.T) {
	pid := spawnTarget(t)
	before := targetNice(pid)
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatal(err)
	}
	r := run(t, "renice", "-n", "2", "-g", strconv.Itoa(pgid))
	wantSuccess(t, r)
	if got := targetNice(pid); got != before+2 {
		t.Fatalf("nice = %d, want %d", got, before+2)
	}
}

func TestReniceRefusesBadOperandOnly(t *testing.T) {
	r := run(t, "renice", "-n", "1", "abc")
	wantFail(t, r)
	wantErrContains(t, r, "renice: bad 'abc'")
}
