package builtins

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

func TestKillallNoName(t *testing.T) {
	r := runBuiltin(t, "killall")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "killall: no name")
	assertStderrContains(t, r, "Try 'killall --help' for more information.")
}

func TestKillallUnknownOption(t *testing.T) {
	r := runBuiltin(t, "killall", "-Z", "misa-u27-killall-nomatch")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "killall: unknown option -Z")
	assertStderrContains(t, r, "Try 'killall --help' for more information.")
}

func TestKillallInvalidSignal(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"spec", []string{"-s", "NOPE", "misa-u27-killall-nomatch"}, `killall: invalid signal "NOPE"` + "\n"},
		{"operand", []string{"-s", "SIGBOGUS", "misa-u27-killall-nomatch"}, `killall: invalid signal "SIGBOGUS"` + "\n"},
		{"quiet", []string{"-q", "-s", "NOPE", "misa-u27-killall-nomatch"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltin(t, "killall", tt.args...)
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			if r.stderr != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", r.stderr, tt.wantStderr)
			}
		})
	}
}

func TestKillallListSignals(t *testing.T) {
	r := runBuiltin(t, "killall", "-l")
	assertExitCode(t, r, 0)
	want := "EXIT HUP INT QUIT ILL TRAP ABRT BUS FPE KILL USR1 SEGV USR2 PIPE ALRM TERM " +
		"STKFLT CHLD CONT STOP TSTP TTIN TTOU URG XCPU XFSZ VTALRM PROF WINCH IO PWR SYS RTMIN RTMAX\n"
	assertStdout(t, r, want)
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestKillallHelp(t *testing.T) {
	r := runBuiltin(t, "killall", "--help")
	assertExitCode(t, r, 0)
	for _, want := range []string{"Usage: killall", "-i", "-l", "-q", "-s", "-v", "-w"} {
		assertStdoutContains(t, r, want)
	}
}

func TestKillallVersion(t *testing.T) {
	r := runBuiltin(t, "killall", "--version")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "killall (Misa) 0.1.0\n")
}

func TestKillallNoSuchProcess(t *testing.T) {
	name := "misa-u27-killall-nomatch"
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"warns", []string{name}, "killall: " + name + ": No such process\n"},
		{"quiet", []string{"-q", name}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltin(t, "killall", tt.args...)
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			if r.stderr != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", r.stderr, tt.wantStderr)
			}
		})
	}
}

func TestKillallMatchRules(t *testing.T) {
	dir := t.TempDir()
	exe := writeFile(t, dir, "sleep", "")
	link := filepath.Join(dir, "linked")
	if err := os.Symlink(exe, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	other := writeFile(t, dir, "other", "")

	tests := []struct {
		name    string
		operand string
		proc    proc.Process
		want    bool
	}{
		{"comm", "mytool", proc.Process{Comm: "mytool"}, true},
		{"comm too long for truncation", "mytoolongname1234", proc.Process{Comm: "mytoolongname1"}, false},
		{"comm differs", "mytool", proc.Process{Comm: "othertool", Argv: []string{"/usr/bin/othertool"}}, false},
		{"argv0", "mytool", proc.Process{Comm: "mytoolongname1", Argv: []string{"/usr/bin/mytool", "-x"}}, true},
		{"argv1 script", "myscript.sh", proc.Process{Argv: []string{"/bin/sh", "/tmp/myscript.sh"}}, true},
		{"path same file", exe, proc.Process{Exe: exe}, true},
		{"relative path same file", "./linked", proc.Process{Exe: exe}, true},
		{"path other file", other, proc.Process{Comm: "other", Exe: exe, Argv: []string{other}}, false},
		{"missing path falls back to argv0", filepath.Join(dir, "gone", "other"), proc.Process{Argv: []string{"/x/other"}}, true},
		{"nothing", "mytool", proc.Process{Comm: "zzz"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := killallMatch(tt.operand, dir, tt.proc); got != tt.want {
				t.Errorf("killallMatch(%q, %q) = %v, want %v", tt.operand, tt.proc.Argv, got, tt.want)
			}
		})
	}
}

// killallTestChild is one sleep(1) child the test spawned under a unique name.
type killallTestChild struct {
	name string
	cmd  *exec.Cmd
}

var killallTestCounter atomic.Int64

func (c killallTestChild) pid() int { return c.cmd.Process.Pid }

func (c killallTestChild) alive() bool { return syscall.Kill(c.pid(), 0) == nil }

func (c killallTestChild) waitSignal(t *testing.T) syscall.Signal {
	t.Helper()
	err := c.cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("child %d: Wait() = %v, want a signal exit", c.pid(), err)
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		t.Fatalf("child %d: not killed by a signal: %v", c.pid(), err)
	}
	return status.Signal()
}

// killallStartSleep symlinks the system sleep binary into a temporary
// directory under a unique name longer than the 15-byte comm truncation, so
// only the argv[0] rule can match it, then runs it for duration seconds.
func killallStartSleep(t *testing.T, duration string) killallTestChild {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("killall needs unix signals")
	}
	bin, err := exec.LookPath("sleep")
	if err != nil {
		if _, statErr := os.Stat("/bin/sleep"); statErr != nil {
			t.Skip("sleep(1) is not available")
		}
		bin = "/bin/sleep"
	}
	name := fmt.Sprintf("misa-u27-killall-%d-%d", os.Getpid(), killallTestCounter.Add(1))
	link := filepath.Join(t.TempDir(), name)
	if err := os.Symlink(bin, link); err != nil {
		t.Skipf("cannot symlink %s: %v", bin, err)
	}
	cmd := exec.Command(link, duration)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", link, err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return killallTestChild{name: name, cmd: cmd}
}

func TestKillallDefaultSignalKillsOnlyNamedChild(t *testing.T) {
	child := killallStartSleep(t, "30")
	bystander := killallStartSleep(t, "30")

	r := runBuiltin(t, "killall", child.name)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}

	if sig := child.waitSignal(t); sig != syscall.SIGTERM {
		t.Errorf("child killed by %v, want SIGTERM", sig)
	}
	if !bystander.alive() {
		t.Errorf("killall signalled the bystander %d", bystander.pid())
	}
}

func TestKillallSignalSpellings(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want syscall.Signal
	}{
		{"TERM", "-TERM", syscall.SIGTERM},
		{"KILL", "-9", syscall.SIGKILL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := killallStartSleep(t, "30")
			r := runBuiltin(t, "killall", tt.arg, child.name)
			assertExitCode(t, r, 0)
			assertStdout(t, r, "")
			if sig := child.waitSignal(t); sig != tt.want {
				t.Errorf("child killed by %v, want %v", sig, tt.want)
			}
		})
	}
}

func TestKillallVerbose(t *testing.T) {
	child := killallStartSleep(t, "30")
	r := runBuiltin(t, "killall", "-v", child.name)
	assertExitCode(t, r, 0)
	assertStdout(t, r, fmt.Sprintf("Killed %s(%d) with signal 15\n", child.name, child.pid()))
	if sig := child.waitSignal(t); sig != syscall.SIGTERM {
		t.Errorf("child killed by %v, want SIGTERM", sig)
	}
}

func TestKillallInteractive(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		wantSig bool
	}{
		{"decline", "n\n", false},
		{"eof", "", false},
		{"accept", "y\n", true},
		{"accept upper", "YES\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child := killallStartSleep(t, "30")
			r := runBuiltinStdin(t, "killall", tt.stdin, "-i", child.name)
			assertExitCode(t, r, 0)
			assertStderrContains(t, r, fmt.Sprintf("Signal %s(%d)? (y/N) ", child.name, child.pid()))
			if !tt.wantSig {
				if !child.alive() {
					t.Fatalf("child %d was signalled after a declined prompt", child.pid())
				}
				return
			}
			if sig := child.waitSignal(t); sig != syscall.SIGTERM {
				t.Errorf("child killed by %v, want SIGTERM", sig)
			}
		})
	}
}

func TestKillallWaitUntilGone(t *testing.T) {
	child := killallStartSleep(t, "0.2")
	cmd, ok := Lookup("killall")
	if !ok {
		t.Fatal("builtin \"killall\" not registered")
	}

	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Args:   []string{"-0", "-w", child.name},
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	}
	done := make(chan int, 1)
	go func() { done <- cmd.Run(ctx) }()

	select {
	case code := <-done:
		t.Fatalf("killall -w returned (%d, stderr %q) while the child was still running", code, stderr.String())
	case <-time.After(100 * time.Millisecond):
	}

	if err := child.cmd.Wait(); err != nil {
		t.Fatalf("child %d: Wait() = %v, want a clean exit", child.pid(), err)
	}

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("killall -w = %d, want 0 (stderr %q)", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("killall -w did not return after the child was gone")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestKillallSelfSkip(t *testing.T) {
	child := killallStartSleep(t, "30")
	prev := killallGetpid
	killallGetpid = func() int { return child.pid() }
	t.Cleanup(func() { killallGetpid = prev })

	r := runBuiltin(t, "killall", child.name)
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "killall: "+child.name+": No such process")
	if !child.alive() {
		t.Fatalf("killall signalled the process it considered itself")
	}

	killallGetpid = prev
	r = runBuiltin(t, "killall", child.name)
	assertExitCode(t, r, 0)
	if sig := child.waitSignal(t); sig != syscall.SIGTERM {
		t.Errorf("child killed by %v, want SIGTERM", sig)
	}
}

func TestKillallSelfNameNotSignalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("killall needs unix signals")
	}
	own := filepath.Base(os.Args[0])
	// Signal 0 keeps this harmless for any other process of the same name
	// (parallel test binaries) while still proving our own pid is skipped.
	r := runBuiltin(t, "killall", "-0", "-v", own)
	if r.code != 0 && r.code != 1 {
		t.Fatalf("exit code = %d, want 0 or 1 (stderr: %s)", r.code, r.stderr)
	}
	ownPid := fmt.Sprintf("(%d)", os.Getpid())
	if strings.Contains(r.stdout, ownPid) {
		t.Errorf("stdout %q reports a signal to killall's own pid", r.stdout)
	}
	if strings.Contains(r.stderr, fmt.Sprintf("pid %d", os.Getpid())) {
		t.Errorf("stderr %q reports a failed signal to killall's own pid", r.stderr)
	}
	if r.code == 1 && !strings.Contains(r.stderr, "No such process") {
		t.Errorf("stderr = %q, want a No such process warning", r.stderr)
	}
}
