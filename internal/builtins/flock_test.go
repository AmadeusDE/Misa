//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris

package builtins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// runTool runs a registered builtin with an explicit Context so tests can own
// stdin/stdout and the cancellation context. It never calls t.Fatal, because
// several cases run on goroutines.
func runTool(ctx *Context) runResult {
	cmd, ok := Lookup(ctx.CmdName)
	if !ok {
		return runResult{code: 127, stderr: "builtin not registered"}
	}
	var stdout, stderr bytes.Buffer
	stdoutFile, _ := ctx.Stdout.(*os.File)
	stderrFile, _ := ctx.Stderr.(*os.File)
	if stdoutFile == nil {
		ctx.Stdout = &stdout
	}
	if stderrFile == nil {
		ctx.Stderr = &stderr
	}
	if ctx.Args == nil {
		ctx.Args = []string{}
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func toolContext(dir, name string, args ...string) *Context {
	return &Context{Dir: dir, Args: args, CmdName: name, Width: 80, Height: 24, Stdin: strings.NewReader("")}
}

// holdLock takes an advisory lock on path and returns the owning file.
func holdLock(t *testing.T, path string, op int) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	if err := unix.Flock(int(file.Fd()), op); err != nil {
		file.Close()
		t.Fatalf("hold lock: %v", err)
	}
	return file
}

func TestFlockPathRunsCommand(t *testing.T) {
	dir := t.TempDir()
	ctx := toolContext(dir, "flock", "lock", "echo", "hi")
	r := runTool(ctx)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hi\n")
	if _, err := os.Stat(filepath.Join(dir, "lock")); err != nil {
		t.Errorf("flock must create a missing lock file: %v", err)
	}
}

func TestFlockDirectoryOperand(t *testing.T) {
	dir := t.TempDir()
	ctx := toolContext(dir, "flock", ".", "true")
	assertExitCode(t, runTool(ctx), 0)
}

func TestFlockExclusiveBlocksUntilReleased(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "lock")
	holder := holdLock(t, lockPath, unix.LOCK_EX)
	defer holder.Close()

	done := make(chan runResult, 1)
	go func() {
		done <- runTool(toolContext(dir, "flock", "lock", "true"))
	}()
	select {
	case r := <-done:
		t.Fatalf("exclusive flock returned while the lock was held: %+v", r)
	case <-time.After(250 * time.Millisecond):
	}
	if err := unix.Flock(int(holder.Fd()), unix.LOCK_UN); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	select {
	case r := <-done:
		assertExitCode(t, r, 0)
	case <-time.After(15 * time.Second):
		t.Fatal("flock did not acquire the released lock")
	}
}

func TestFlockSharedSharedSucceeds(t *testing.T) {
	dir := t.TempDir()
	holder := holdLock(t, filepath.Join(dir, "lock"), unix.LOCK_SH)
	defer holder.Close()

	done := make(chan runResult, 1)
	go func() {
		done <- runTool(toolContext(dir, "flock", "-s", "lock", "true"))
	}()
	select {
	case r := <-done:
		assertExitCode(t, r, 0)
	case <-time.After(15 * time.Second):
		t.Fatal("shared flock blocked behind a shared lock")
	}
}

func TestFlockNonblockFailsFast(t *testing.T) {
	dir := t.TempDir()
	holder := holdLock(t, filepath.Join(dir, "lock"), unix.LOCK_EX)
	defer holder.Close()

	start := time.Now()
	r := runTool(toolContext(dir, "flock", "-n", "lock", "true"))
	assertExitCode(t, r, 1)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("-n waited %v instead of failing fast", elapsed)
	}
	if r.stderr != "" {
		t.Errorf("-n on a busy lock must be silent, got %q", r.stderr)
	}
}

func TestFlockNonblockExclusiveAgainstShared(t *testing.T) {
	dir := t.TempDir()
	holder := holdLock(t, filepath.Join(dir, "lock"), unix.LOCK_SH)
	defer holder.Close()

	assertExitCode(t, runTool(toolContext(dir, "flock", "-x", "-n", "lock", "true")), 1)
}

func TestFlockOptionsAfterFileGoToCommand(t *testing.T) {
	dir := t.TempDir()
	r := runTool(toolContext(dir, "flock", "lock", "echo", "-n", "hi"))
	assertExitCode(t, r, 0)
	// -n belongs to echo here, not to flock: no trailing newline.
	assertStdout(t, r, "hi")
}

func TestFlockNonblockBeatsWait(t *testing.T) {
	dir := t.TempDir()
	holder := holdLock(t, filepath.Join(dir, "lock"), unix.LOCK_EX)
	defer holder.Close()

	start := time.Now()
	assertExitCode(t, runTool(toolContext(dir, "flock", "-n", "-w", "5", "lock", "true")), 1)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("-n must fail fast even with -w, took %v", elapsed)
	}
}

func TestFlockWaitTimesOut(t *testing.T) {
	dir := t.TempDir()
	holder := holdLock(t, filepath.Join(dir, "lock"), unix.LOCK_EX)
	defer holder.Close()

	start := time.Now()
	r := runTool(toolContext(dir, "flock", "-w", "0.2", "lock", "true"))
	assertExitCode(t, r, 1)
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Errorf("-w 0.2 returned after %v, before the timeout", elapsed)
	}
	if !strings.Contains(r.stderr, "timed out") {
		t.Errorf("stderr = %q, want a timeout message", r.stderr)
	}
}

func TestFlockWaitCancelled(t *testing.T) {
	dir := t.TempDir()
	holder := holdLock(t, filepath.Join(dir, "lock"), unix.LOCK_EX)
	defer holder.Close()

	ctx, cancel := context.WithCancel(context.Background())
	runCtx := toolContext(dir, "flock", "lock", "true")
	runCtx.Ctx = ctx
	done := make(chan runResult, 1)
	go func() { done <- runTool(runCtx) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case r := <-done:
		assertExitCode(t, r, 130)
	case <-time.After(15 * time.Second):
		t.Fatal("cancelling the context did not stop flock")
	}
}

func TestFlockHoldsLockWhileCommandRuns(t *testing.T) {
	dir := t.TempDir()
	done := make(chan runResult, 1)
	go func() {
		done <- runTool(toolContext(dir, "flock", "lock", "sleep", "0.5"))
	}()
	time.Sleep(200 * time.Millisecond)

	probe, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open probe: %v", err)
	}
	defer probe.Close()
	if err := unix.Flock(int(probe.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
		t.Error("the lock must be held while the wrapped command runs")
		unix.Flock(int(probe.Fd()), unix.LOCK_UN)
	}
	select {
	case r := <-done:
		assertExitCode(t, r, 0)
	case <-time.After(15 * time.Second):
		t.Fatal("wrapped command did not finish")
	}
	if err := unix.Flock(int(probe.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Errorf("the lock must be free once the command exits: %v", err)
	}
}

func TestFlockUnlocksWhenCommandExits(t *testing.T) {
	dir := t.TempDir()
	assertExitCode(t, runTool(toolContext(dir, "flock", "lock", "true")), 0)

	probe, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer probe.Close()
	if err := unix.Flock(int(probe.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Errorf("lock still held after the command exited: %v", err)
	}
}

func TestFlockCommandStatusPropagates(t *testing.T) {
	dir := t.TempDir()
	assertExitCode(t, runTool(toolContext(dir, "flock", "lock", "false")), 1)
	assertExitCode(t, runTool(toolContext(dir, "flock", "lock", "true")), 0)
}

func TestFlockMissingCommand(t *testing.T) {
	dir := t.TempDir()
	r := runTool(toolContext(dir, "flock", "lock", "misa-no-such-command"))
	assertExitCode(t, r, 127)
	if !strings.Contains(r.stderr, "failed to run command") {
		t.Errorf("stderr = %q, want a launcher error", r.stderr)
	}
}

func TestFlockDescriptorForm(t *testing.T) {
	dir := t.TempDir()
	target, err := os.OpenFile(filepath.Join(dir, "target"), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := toolContext(dir, "flock", "-s", "0")
	ctx.Stdin = target
	assertExitCode(t, runTool(ctx), 0)

	// The descriptor lock lives as long as the caller's descriptor: not even
	// an exclusive request can take it while target stays open.
	probe, err := os.OpenFile(filepath.Join(dir, "target"), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open probe: %v", err)
	}
	defer probe.Close()
	if err := unix.Flock(int(probe.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
		t.Errorf("a shared descriptor lock must still be held while the descriptor is open")
	}
	target.Close()
	if err := unix.Flock(int(probe.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Errorf("closing the descriptor must release the lock: %v", err)
	}
}

func TestFlockDescriptorUnlock(t *testing.T) {
	dir := t.TempDir()
	held := holdLock(t, filepath.Join(dir, "target"), unix.LOCK_EX)
	defer held.Close()

	ctx := toolContext(dir, "flock", "-u", "0")
	ctx.Stdin = held
	assertExitCode(t, runTool(ctx), 0)

	probe, err := os.OpenFile(filepath.Join(dir, "target"), os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open probe: %v", err)
	}
	defer probe.Close()
	if err := unix.Flock(int(probe.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Errorf("-u did not release descriptor 0: %v", err)
	}
}

func TestFlockDescriptorUnreachable(t *testing.T) {
	dir := t.TempDir()
	r := runTool(toolContext(dir, "flock", "3"))
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "fd 3 not available (needs shell redirection support, S09)") {
		t.Errorf("stderr = %q, want the S09 diagnostic", r.stderr)
	}

	r = runTool(toolContext(dir, "flock", "1"))
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "fd 1 is not an open file") {
		t.Errorf("stderr = %q, want the standard-stream diagnostic", r.stderr)
	}
}

func TestFlockUsageErrors(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no operand", nil, "missing operand"},
		{"no command", []string{"lock"}, "missing command"},
		{"unknown option", []string{"-q", "lock", "true"}, "unknown option -q"},
		{"conflicting modes", []string{"-s", "-x", "lock", "true"}, "mutually exclusive"},
		{"bad wait", []string{"-w", "soon", "lock", "true"}, "invalid wait time"},
		{"negative wait", []string{"-w", "-1", "lock", "true"}, "invalid wait time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runTool(toolContext(dir, "flock", tc.args...))
			assertExitCode(t, r, 1)
			if !strings.Contains(r.stderr, tc.want) {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.want)
			}
		})
	}
}

func TestFlockHelp(t *testing.T) {
	dir := t.TempDir()
	r := runTool(toolContext(dir, "flock", "--help"))
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "S09") || !strings.Contains(r.stdout, "-w") {
		t.Errorf("--help = %q, want the descriptor limitation and -w", r.stdout)
	}
	r = runTool(toolContext(dir, "flock", "--version"))
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "flock (Misa)") {
		t.Errorf("--version = %q", r.stdout)
	}
}
