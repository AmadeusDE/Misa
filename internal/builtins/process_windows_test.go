//go:build windows

package builtins

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestKillListsOnlyWhatWindowsCanDeliver(t *testing.T) {
	r := runBuiltin(t, "kill", "-l")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "EXIT KILL TERM\n")
}

func TestKillRefusesSignalsWindowsCannotDeliver(t *testing.T) {
	r := runBuiltin(t, "kill", "-s", "HUP", "4")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not supported")
}

func TestKillTerminatesAProcess(t *testing.T) {
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	defer cmd.Process.Kill()
	pid := cmd.Process.Pid
	assertExitCode(t, runBuiltin(t, "kill", "-0", strconv.Itoa(pid)), 0)
	assertExitCode(t, runBuiltin(t, "kill", "-s", "KILL", strconv.Itoa(pid)), 0)
	err := cmd.Wait()
	if err == nil {
		t.Fatal("the process exited cleanly after kill")
	}
	if code := err.(*exec.ExitError).ExitCode(); code != 137 {
		t.Errorf("exit code = %d, want 137 (128+KILL)", code)
	}
}

func TestPgrepIsUnsupported(t *testing.T) {
	r := runBuiltin(t, "pgrep", "x")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not supported")
}

func TestTimeoutEndsTheCommand(t *testing.T) {
	r := runBuiltin(t, "timeout", "1", "ping", "-n", "30", "127.0.0.1")
	assertExitCode(t, r, 124)
}

func TestTimeoutRejectsOtherSignals(t *testing.T) {
	r := runBuiltin(t, "timeout", "-s", "HUP", "1", "ping", "127.0.0.1")
	assertExitCode(t, r, 125)
	assertStderrContains(t, r, "not supported")
}

func TestNiceChangesTheChildOnly(t *testing.T) {
	before, err := windows.GetPriorityClass(windows.CurrentProcess())
	if err != nil {
		t.Fatal(err)
	}
	r := runBuiltin(t, "nice", "-n", "15", "cmd", "/c", "exit", "3")
	assertExitCode(t, r, 3)
	after, _ := windows.GetPriorityClass(windows.CurrentProcess())
	if before != after {
		t.Errorf("Misa's priority class changed from %#x to %#x", before, after)
	}
}

func TestSyncFlushesNamedFilesAndRefusesTheBareForm(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltin(t, "sync", file), 0)
	r := runBuiltin(t, "sync")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not supported")
	assertExitCode(t, runBuiltin(t, "sync", filepath.Join(dir, "missing")), 1)
}

func TestUnameReportsWindows(t *testing.T) {
	r := runBuiltin(t, "uname", "-s")
	assertStdout(t, r, "Windows_NT\n")
	m := runBuiltin(t, "uname", "-m")
	if got := strings.TrimSpace(m.stdout); got != "x86_64" && got != "aarch64" && got != "i686" && got != "armv7l" && got != "unknown" {
		t.Errorf("uname -m = %q", got)
	}
}
