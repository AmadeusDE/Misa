//go:build unix

package builtins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"
)

// helperPath returns this test binary, which the launcher runs as an external
// command through TestMain's test-helper flag.
func u29HelperPath(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// openTestPTY returns a controlling-terminal pair: the slave is what a builtin
// sees as its stdio, the master is the reading end.
func openTestPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = slave.Close()
		_ = master.Close()
	})
	return master, slave
}

func TestNohupPipesPassThrough(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), "nohup", &Context{
		Dir:    dir,
		Args:   []string{"echo", "hello"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	if status != 0 || stdout.String() != "hello\n" || stderr.String() != "" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	// Nothing was a terminal, so no log file may appear.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unexpected files: %v", entries)
	}
}

func TestNohupRedirectsTerminalOutput(t *testing.T) {
	_, tty := openTestPTY(t)
	var stderr bytes.Buffer
	dir := t.TempDir()
	status := Run(context.Background(), "nohup", &Context{
		Dir:    dir,
		Args:   []string{"sh", "-c", "echo out; echo err >&2; cat"},
		Stdin:  tty,
		Stdout: tty,
		Stderr: &stderr,
	})
	if status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	// stderr is not a terminal, so it stays where the caller pointed it: the
	// hint and the command's own stderr both reach it.
	if got, want := stderr.String(), "nohup: ignoring input and appending output to 'nohup.out'\nerr\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	log := filepath.Join(dir, "nohup.out")
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	// stdin came from /dev/null, so cat read nothing; stdout went to the log.
	if got, want := string(data), "out\n"; got != want {
		t.Fatalf("nohup.out = %q, want %q", got, want)
	}
	info, err := os.Stat(log)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("nohup.out mode = %o, want 600", perm)
	}
}

func TestNohupStderrFollowsRedirectedStdout(t *testing.T) {
	_, tty := openTestPTY(t)
	dir := t.TempDir()
	// Every stream is a terminal here, so stderr must follow stdout into the log.
	status := Run(context.Background(), "nohup", &Context{
		Dir:    dir,
		Args:   []string{"sh", "-c", "echo out; echo err >&2"},
		Stdin:  tty,
		Stdout: tty,
		Stderr: tty,
	})
	if status != 0 {
		t.Fatalf("status=%d", status)
	}
	data, err := os.ReadFile(filepath.Join(dir, "nohup.out"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "out\nerr\n"; got != want {
		t.Fatalf("nohup.out = %q, want %q", got, want)
	}
}

func TestNohupAppendsToExistingLog(t *testing.T) {
	_, tty := openTestPTY(t)
	dir := t.TempDir()
	log := filepath.Join(dir, "nohup.out")
	if err := os.WriteFile(log, []byte("existing\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if status := Run(context.Background(), "nohup", &Context{
		Dir: dir, Args: []string{"echo", "new"},
		Stdin: tty, Stdout: tty, Stderr: &stderr,
	}); status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "existing\nnew\n"; got != want {
		t.Fatalf("nohup.out = %q, want %q", got, want)
	}
}

func TestNohupFallsBackToHome(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	_, tty := openTestPTY(t)
	readonly := t.TempDir()
	home := t.TempDir()
	if err := os.Chmod(readonly, 0555); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	env := []string{"HOME=" + home}
	status := Run(context.Background(), "nohup", &Context{
		Dir: readonly, Env: env, Args: []string{"echo", "home"},
		Stdin: tty, Stdout: tty, Stderr: &stderr,
	})
	if status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	homeLog := filepath.Join(home, "nohup.out")
	if got, want := stderr.String(), "nohup: ignoring input and appending output to '"+homeLog+"'\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	data, err := os.ReadFile(homeLog)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "home\n"; got != want {
		t.Fatalf("$HOME/nohup.out = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(readonly, "nohup.out")); !os.IsNotExist(err) {
		t.Fatalf("nohup.out appeared in the read-only directory: %v", err)
	}
}

func TestNohupUnopenableOutput(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	_, tty := openTestPTY(t)
	readonly := t.TempDir()
	if err := os.Chmod(readonly, 0555); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	status := Run(context.Background(), "nohup", &Context{
		Dir: readonly, Env: []string{}, Args: []string{"echo", "nowhere"},
		Stdin: tty, Stdout: tty, Stderr: &stderr,
	})
	if status != 125 {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cannot open 'nohup.out'") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestNohupIgnoresHangup(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), "nohup", &Context{
		Dir:    t.TempDir(),
		Args:   []string{u29HelperPath(t), "--process-wrapper-test-helper", "hup"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	if status != 0 || stdout.String() != "alive\n" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	// Control: without nohup the same child dies from the default SIGHUP action.
	stdout.Reset()
	stderr.Reset()
	control := Run(context.Background(), "setpgid", &Context{
		Dir:    t.TempDir(),
		Args:   []string{u29HelperPath(t), "--process-wrapper-test-helper", "hup"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	})
	if control != 128+1 || stdout.String() != "" {
		t.Fatalf("control status=%d stdout=%q stderr=%q", control, stdout.String(), stderr.String())
	}
}

func TestNohupForwardsStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		argv []string
		want int
	}{
		{"external", []string{u29HelperPath(t), "--process-wrapper-test-helper", "exit", "42"}, 42},
		{"builtin", []string{"false"}, 1},
		{"missing", []string{"misa-missing-command-27934"}, 127},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			status := Run(context.Background(), "nohup", &Context{
				Dir: t.TempDir(), Args: tc.argv,
				Stdout: &bytes.Buffer{}, Stderr: &stderr, Stdin: strings.NewReader(""),
			})
			if status != tc.want {
				t.Fatalf("status=%d, want %d (stderr %q)", status, tc.want, stderr.String())
			}
			if tc.name == "missing" && !strings.Contains(stderr.String(), "nohup: failed to run command 'misa-missing-command-27934'") {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestNohupUsageErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := Run(context.Background(), "nohup", &Context{Dir: t.TempDir(), Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader("")})
	if status != 125 || !strings.Contains(stderr.String(), "missing operand") {
		t.Fatalf("no operand: status=%d stderr=%q", status, stderr.String())
	}
	stderr.Reset()
	status = Run(context.Background(), "nohup", &Context{Dir: t.TempDir(), Args: []string{"--invalid"}, Stdout: &stdout, Stderr: &stderr})
	if status != 125 || !strings.Contains(stderr.String(), "unrecognized option '--invalid'") {
		t.Fatalf("bad option: status=%d stderr=%q", status, stderr.String())
	}
	stdout.Reset()
	if status = Run(context.Background(), "nohup", &Context{Dir: t.TempDir(), Args: []string{"--help"}, Stdout: &stdout, Stderr: &stderr}); status != 0 {
		t.Fatalf("--help status=%d", status)
	}
	if !strings.HasPrefix(stdout.String(), "Usage: nohup ") {
		t.Fatalf("--help stdout = %q", stdout.String())
	}
	stdout.Reset()
	if status = Run(context.Background(), "nohup", &Context{Dir: t.TempDir(), Args: []string{"--version"}, Stdout: &stdout, Stderr: &stderr}); status != 0 {
		t.Fatalf("--version status=%d", status)
	}
	if !strings.Contains(stdout.String(), "nohup (Misa)") {
		t.Fatalf("--version stdout = %q", stdout.String())
	}
}
