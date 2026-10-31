package builtins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

// pidwaitRunWith executes the builtin with an explicit cancellation context.
func pidwaitRunWith(t *testing.T, ctx context.Context, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("pidwait")
	if !ok {
		t.Fatal("pidwait not registered")
	}
	var stdout, stderr bytes.Buffer
	inv := &Context{
		Dir:    t.TempDir(),
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	}
	if ctx != nil {
		inv.Ctx = ctx
	}
	code := cmd.Run(inv)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// pidwaitInstallSeams points the builtin at a fixture proc tree, removes the
// poll delay and pins the account lookups.
func pidwaitInstallSeams(t *testing.T, root string) {
	t.Helper()
	oldRoot, oldInterval := pidwaitProcRoot, pidwaitPollInterval
	oldSleep, oldLookup := pidwaitSleep, pidwaitLookupUser
	pidwaitProcRoot, pidwaitPollInterval = root, 0
	pidwaitSleep = func(ctx context.Context, d time.Duration) error { return ctx.Err() }
	pidwaitLookupUser = func(name string) (*user.User, error) {
		if name == "bob" {
			return &user.User{Username: "bob", Uid: "1000"}, nil
		}
		return nil, errors.New("unknown user")
	}
	t.Cleanup(func() {
		pidwaitProcRoot, pidwaitPollInterval = oldRoot, oldInterval
		pidwaitSleep, pidwaitLookupUser = oldSleep, oldLookup
	})
}

// pidwaitFakeSleep returns a pidwaitSleep stand-in that runs step on every poll
// and refuses to poll forever, so a matching bug fails the test instead of
// hanging it.
func pidwaitFakeSleep(t *testing.T, step func(call int)) func(context.Context, time.Duration) error {
	t.Helper()
	calls := 0
	return func(ctx context.Context, d time.Duration) error {
		calls++
		if calls > 100 {
			t.Fatalf("pidwait polled %d times without finishing", calls)
		}
		if step != nil {
			step(calls)
		}
		return ctx.Err()
	}
}

// pidwaitStatLine builds a /proc/PID/stat line with enough fields for the
// parser (it needs 22 after the comm) and the given start time.
func pidwaitStatLine(pid int, comm string, startTicks uint64) string {
	return fmt.Sprintf("%d (%s) S 1 %d %d 0 %d 4194560 100 0 0 0 0 0 0 0 20 0 1 0 %d 4096 128 "+
		"18446744073709551615 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0",
		pid, comm, pid, pid, pid, startTicks)
}

// pidwaitWriteProc creates one process in a fixture proc tree.
func pidwaitWriteProc(t *testing.T, root string, pid int, comm string, uid uint32, startTicks uint64) {
	t.Helper()
	dir := filepath.Join(root, fmt.Sprint(pid))
	writeFile(t, dir, "stat", pidwaitStatLine(pid, comm, startTicks))
	writeFile(t, dir, "status", fmt.Sprintf("Name:\t%s\nUid:\t%d\t%d\t%d\t%d\nGid:\t0\t0\t0\t0\n",
		comm, uid, uid, uid, uid))
}

func TestPidwaitStillRunning(t *testing.T) {
	cases := []struct {
		name     string
		tracked  map[int]uint64
		snapshot []proc.Process
		want     map[int]uint64
	}{
		{
			name:     "same start ticks stays",
			tracked:  map[int]uint64{7: 100},
			snapshot: []proc.Process{{PID: 7, StartTicks: 100}},
			want:     map[int]uint64{7: 100},
		},
		{
			name:     "new start ticks means the pid was reused",
			tracked:  map[int]uint64{7: 100},
			snapshot: []proc.Process{{PID: 7, StartTicks: 200}},
			want:     map[int]uint64{},
		},
		{
			name:     "absent pid is done",
			tracked:  map[int]uint64{7: 100},
			snapshot: []proc.Process{{PID: 8, StartTicks: 200}},
			want:     map[int]uint64{},
		},
		{
			name:     "empty snapshot finishes everything",
			tracked:  map[int]uint64{7: 100, 8: 200},
			snapshot: nil,
			want:     map[int]uint64{},
		},
		{
			name:    "mixed outcomes",
			tracked: map[int]uint64{7: 100, 8: 200, 9: 300},
			snapshot: []proc.Process{
				{PID: 7, StartTicks: 100},
				{PID: 8, StartTicks: 999},
				{PID: 42, StartTicks: 1},
			},
			want: map[int]uint64{7: 100},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tracked := make(map[int]uint64, len(tc.tracked))
			for pid, ticks := range tc.tracked {
				tracked[pid] = ticks
			}
			got := pidwaitStillRunning(tracked, tc.snapshot)
			if len(got) != len(tc.want) {
				t.Fatalf("still running = %v, want %v", got, tc.want)
			}
			for pid, ticks := range tc.want {
				gotTicks, ok := got[pid]
				if !ok || gotTicks != ticks {
					t.Fatalf("still running = %v, want %v", got, tc.want)
				}
			}
			if len(tracked) != len(tc.tracked) {
				t.Fatalf("pidwaitStillRunning mutated its input: %v", tracked)
			}
			for pid, ticks := range tc.tracked {
				if tracked[pid] != ticks {
					t.Fatalf("pidwaitStillRunning mutated its input: %v", tracked)
				}
			}
		})
	}
}

func TestPidwaitMatchTable(t *testing.T) {
	pidwaitInstallSeams(t, t.TempDir())

	procs := []proc.Process{
		{PID: 10, Comm: "bash", UID: 1000, StartTicks: 1, Argv: []string{"/bin/bash", "-l"}},
		{PID: 11, Comm: "bashful", UID: 1000, StartTicks: 2, Argv: []string{"/usr/bin/bashful"}},
		{PID: 12, Comm: "sleep", UID: 1001, StartTicks: 3, Argv: []string{"sleep", "10"}},
		{PID: 13, Comm: "", UID: 0, StartTicks: 4},
		{PID: 14, Comm: "", UID: 0, StartTicks: 5, Argv: []string{"kernel", "thread"}},
		{PID: 42, Comm: "bash", UID: 1000, StartTicks: 6, Argv: []string{"/bin/bash"}},
	}

	cases := []struct {
		name string
		opts pidwaitMatchOptions
		want []int
	}{
		{"default matches a comm substring", pidwaitMatchOptions{pattern: "bash"}, []int{10, 11}},
		{"exact matches the whole comm", pidwaitMatchOptions{pattern: "bash", exact: true}, []int{10}},
		{"full matches the command line", pidwaitMatchOptions{pattern: "^/usr/bin/bashful$", full: true}, []int{11}},
		{"exact full matches the whole command line", pidwaitMatchOptions{pattern: "/bin/bash -l", exact: true, full: true}, []int{10}},
		{"numeric uid", pidwaitMatchOptions{pattern: ".*", user: "1001"}, []int{12}},
		{"user by name", pidwaitMatchOptions{pattern: "bash", user: "bob"}, []int{10, 11}},
		{"empty comm never matches without full", pidwaitMatchOptions{pattern: "kernel"}, nil},
		{"empty comm matches with full", pidwaitMatchOptions{pattern: "kernel", full: true}, []int{14}},
		{"nothing matches", pidwaitMatchOptions{pattern: "nosuchthing"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched, err := pidwaitMatch(procs, tc.opts, 42)
			if err != nil {
				t.Fatalf("pidwaitMatch: %v", err)
			}
			var got []int
			for _, p := range matched {
				got = append(got, p.PID)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("matched pids = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("matched pids = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestPidwaitMatchErrors(t *testing.T) {
	pidwaitInstallSeams(t, t.TempDir())
	procs := []proc.Process{{PID: 10, Comm: "bash", UID: 1000}}

	if _, err := pidwaitMatch(procs, pidwaitMatchOptions{pattern: "["}, 0); err == nil {
		t.Error("invalid pattern: err = nil, want a pattern error")
	} else if want := `invalid pattern "[":`; !strings.Contains(err.Error(), want) {
		t.Errorf("invalid pattern: err = %q, want it to contain %q", err, want)
	}
	if _, err := pidwaitMatch(procs, pidwaitMatchOptions{pattern: "bash", user: "nobody-u27"}, 0); err == nil {
		t.Error("unknown user: err = nil, want a user error")
	} else if want := "no such user: nobody-u27"; err.Error() != want {
		t.Errorf("unknown user: err = %q, want %q", err, want)
	}
}

func TestPidwaitUsageAndVersion(t *testing.T) {
	r := pidwaitRunWith(t, nil, "--help")
	assertExitCode(t, r, 0)
	for _, want := range []string{
		"Usage: pidwait [OPTIONS] PATTERN",
		"regular expression",
		"-f, --full",
		"-x, --exact",
		"-u, --user",
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("--help output %q does not contain %q", r.stdout, want)
		}
	}

	r = pidwaitRunWith(t, nil, "--version")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "pidwait (Misa) 0.1.0\n")
}

func TestPidwaitArgumentErrors(t *testing.T) {
	pidwaitInstallSeams(t, t.TempDir())

	t.Run("missing pattern", func(t *testing.T) {
		r := pidwaitRunWith(t, nil)
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "pidwait: missing pattern\n")
		assertStderrContains(t, r, "Try 'pidwait --help' for more information.")
	})

	t.Run("unknown option", func(t *testing.T) {
		r := pidwaitRunWith(t, nil, "-e", "sh")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "pidwait: unknown option -e\n")
		assertStderrContains(t, r, "Try 'pidwait --help' for more information.")
	})

	t.Run("invalid pattern", func(t *testing.T) {
		r := pidwaitRunWith(t, nil, "[")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, `pidwait: invalid pattern "[":`)
	})

	t.Run("unknown user", func(t *testing.T) {
		r := pidwaitRunWith(t, nil, "-u", "nobody-u27", "sh")
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "pidwait: no such user: nobody-u27\n")
	})

	t.Run("no match", func(t *testing.T) {
		r := pidwaitRunWith(t, nil, "no-such-comm-u27")
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "")
		if r.stderr != "" {
			t.Errorf("stderr = %q, want nothing", r.stderr)
		}
	})

	t.Run("unreadable proc root", func(t *testing.T) {
		pidwaitProcRoot = filepath.Join(t.TempDir(), "absent")
		r := pidwaitRunWith(t, nil, "sh")
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, "pidwait: ")
	})
}

func TestPidwaitWaitsForFixtureProcesses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the procfs backend is linux-only")
	}

	t.Run("returns once the process is gone", func(t *testing.T) {
		root := t.TempDir()
		pidwaitWriteProc(t, root, 4242, "u27wait", 0, 777)
		pidwaitInstallSeams(t, root)
		polls := 0
		pidwaitSleep = pidwaitFakeSleep(t, func(call int) {
			polls = call
			if call == 3 {
				if err := os.RemoveAll(filepath.Join(root, "4242")); err != nil {
					t.Fatalf("remove fixture: %v", err)
				}
			}
		})

		r := pidwaitRunWith(t, nil, "-x", "u27wait")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "")
		if polls != 3 {
			t.Errorf("polls = %d, want 3", polls)
		}
	})

	t.Run("returns once the start ticks change", func(t *testing.T) {
		root := t.TempDir()
		pidwaitWriteProc(t, root, 4243, "u27reuse", 0, 100)
		pidwaitInstallSeams(t, root)
		polls := 0
		pidwaitSleep = pidwaitFakeSleep(t, func(call int) {
			polls = call
			if call == 2 {
				writeFile(t, filepath.Join(root, "4243"), "stat", pidwaitStatLine(4243, "u27reuse", 999))
			}
		})

		r := pidwaitRunWith(t, nil, "-x", "u27reuse")
		assertExitCode(t, r, 0)
		if polls != 2 {
			t.Errorf("polls = %d, want 2 (a reused pid ends the wait)", polls)
		}
	})

	t.Run("matches the full command line", func(t *testing.T) {
		root := t.TempDir()
		pidwaitWriteProc(t, root, 4244, "u27cmd", 0, 100)
		writeFile(t, filepath.Join(root, "4244"), "cmdline", "u27cmd\x00--flag\x00")
		pidwaitInstallSeams(t, root)
		pidwaitSleep = pidwaitFakeSleep(t, func(call int) {
			if call == 2 {
				os.RemoveAll(filepath.Join(root, "4244"))
			}
		})

		r := pidwaitRunWith(t, nil, "-f", "-x", "u27cmd --flag")
		assertExitCode(t, r, 0)
	})

	t.Run("nothing matched", func(t *testing.T) {
		root := t.TempDir()
		pidwaitWriteProc(t, root, 4245, "u27other", 0, 100)
		pidwaitInstallSeams(t, root)
		pidwaitSleep = pidwaitFakeSleep(t, nil)

		r := pidwaitRunWith(t, nil, "-x", "u27wait")
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
	})

	t.Run("cancelled while waiting", func(t *testing.T) {
		root := t.TempDir()
		pidwaitWriteProc(t, root, 4246, "u27cancel", 0, 100)
		pidwaitInstallSeams(t, root)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		pidwaitSleep = pidwaitFakeSleep(t, func(call int) {
			if call == 2 {
				cancel()
			}
		})

		r := pidwaitRunWith(t, ctx, "-x", "u27cancel")
		assertExitCode(t, r, 130)
		assertStdout(t, r, "")
	})
}

var pidwaitLiveSeq atomic.Int32

// TestPidwaitLiveChild waits for a child this test starts and reaps. The comm
// the kernel reports is truncated to 15 bytes, so the unique name stays short.
func TestPidwaitLiveChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no procfs on windows")
	}
	// The test binary in helper mode stands in for sleep(1); a multicall
	// coreutils sleep refuses to run under an unknown argv0.
	sleepPath, err := os.Executable()
	if err != nil {
		t.Skipf("cannot locate test binary: %v", err)
	}
	dir := t.TempDir()

	spawn := func(t *testing.T, seconds string) string {
		t.Helper()
		name := fmt.Sprintf("u27pw%d-%d", os.Getpid(), pidwaitLiveSeq.Add(1))
		if len(name) > 15 {
			t.Fatalf("comm name %q is longer than the kernel's 15-byte limit", name)
		}
		link := filepath.Join(dir, name)
		if err := os.Symlink(sleepPath, link); err != nil {
			t.Fatalf("symlink %s: %v", link, err)
		}
		child := exec.Command(link)
		child.Env = append(os.Environ(), sleepHelperEnv+"="+seconds)
		if err := child.Start(); err != nil {
			t.Fatalf("start %s: %v", link, err)
		}
		reaped := make(chan error, 1)
		go func() { reaped <- child.Wait() }()
		t.Cleanup(func() {
			_ = child.Process.Kill()
			<-reaped
		})
		return name
	}

	t.Run("waits until the child exits", func(t *testing.T) {
		name := spawn(t, "0.2")
		start := time.Now()
		r := pidwaitRunWith(t, nil, "-x", name)
		elapsed := time.Since(start)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "")
		if elapsed < 150*time.Millisecond {
			t.Errorf("returned after %v, want it to wait for the child", elapsed)
		}
		if elapsed > 5*time.Second {
			t.Errorf("returned after %v, want a prompt exit", elapsed)
		}
	})

	t.Run("cancelled while the child runs", func(t *testing.T) {
		name := spawn(t, "5")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		r := pidwaitRunWith(t, ctx, "-x", name)
		assertExitCode(t, r, 130)
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("cancellation took %v", elapsed)
		}
	})
}
