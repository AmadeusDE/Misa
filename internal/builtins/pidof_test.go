package builtins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/proc"
)

func TestPidofRegistered(t *testing.T) {
	if _, ok := Lookup("pidof"); !ok {
		t.Fatal("pidof not registered")
	}
}

// TestPidofMatches covers the pure matcher: the comm fast path (only with -x
// and only for names that fit the 15-byte comm field), argv[0]'s basename,
// argv[1]'s basename for scripts, and path operands compared by file identity.
func TestPidofMatches(t *testing.T) {
	dir := t.TempDir()
	tool := writeFile(t, dir, "tool", "binary")
	other := writeFile(t, dir, "other", "binary")
	gone := filepath.Join(dir, "gone", "tool")

	cases := []struct {
		name    string
		operand string
		proc    proc.Process
		scripts bool
		want    bool
	}{
		{"comm with -x", "sleep", proc.Process{Comm: "sleep"}, true, true},
		{"comm without -x", "sleep", proc.Process{Comm: "sleep"}, false, false},
		{"comm 14 bytes", "aaaaaaaaaaaaaa", proc.Process{Comm: "aaaaaaaaaaaaaa"}, true, true},
		{"comm 15 bytes is never trusted", "aaaaaaaaaaaaaaa", proc.Process{Comm: "aaaaaaaaaaaaaaa"}, true, false},
		{"comm only matches the whole name", "sleep", proc.Process{Comm: "sleepy"}, true, false},
		{"argv0 absolute path", "sleep", proc.Process{Argv: []string{"/usr/bin/sleep", "60"}}, false, true},
		{"argv0 other name", "sleep", proc.Process{Argv: []string{"/usr/bin/other"}}, false, false},
		{"argv0 path operand basename", tool, proc.Process{Argv: []string{"/usr/bin/tool"}}, false, true},
		{"script without -x", "myscript.sh", proc.Process{Argv: []string{"/bin/sh", "./myscript.sh"}}, false, false},
		{"script with -x", "myscript.sh", proc.Process{Argv: []string{"/bin/sh", "./myscript.sh"}}, true, true},
		{"script with -x and another name", "other.sh", proc.Process{Argv: []string{"/bin/sh", "./myscript.sh"}}, true, false},
		{"empty argv", "sleep", proc.Process{}, false, false},
		{"path identity", tool, proc.Process{Argv: []string{"renamed"}, Exe: tool}, false, true},
		{"path relative to the invocation dir", "./tool", proc.Process{Exe: tool}, false, true},
		{"path of a different file", tool, proc.Process{Exe: other}, false, false},
		{"path without an exe link", tool, proc.Process{Argv: []string{"renamed"}}, false, false},
		{"path fallback when stat fails", gone, proc.Process{Exe: gone}, false, true},
		{"path fallback needs the same path", gone, proc.Process{Exe: filepath.Join(dir, "gone", "other")}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			op := pidofOperandFor(c.operand, dir)
			if got := pidofMatches(c.proc, op, c.scripts); got != c.want {
				t.Errorf("pidofMatches(%q, %+v, scripts=%v) = %v, want %v",
					c.operand, c.proc, c.scripts, got, c.want)
			}
		})
	}
}

func TestPidofOmitList(t *testing.T) {
	const ppid = 4242
	cases := []struct {
		list    string
		pid     int
		omitted bool
		ok      bool
	}{
		{"", 100, false, true},
		{"100", 100, true, true},
		{"100", 101, false, true},
		{"100,300", 300, true, true},
		{" 100 , 300 ", 300, true, true},
		{"100-200", 100, true, true},
		{"100-200", 200, true, true},
		{"100-200", 201, false, true},
		{"5-5", 5, true, true},
		{"%PPID", ppid, true, true},
		{"%PPID", 1, false, true},
		{"1,%PPID", ppid, true, true},
		{"x", 0, false, false},
		{"1-", 0, false, false},
		{"-3", 0, false, false},
		{"3-1", 0, false, false},
		{"1,,2", 0, false, false},
		{"%PP", 0, false, false},
	}
	for _, c := range cases {
		ranges, err := pidofParseOmit(c.list, ppid)
		if (err == nil) != c.ok {
			t.Fatalf("pidofParseOmit(%q) error = %v, want ok=%v", c.list, err, c.ok)
		}
		if !c.ok {
			if msg := err.Error(); !strings.Contains(msg, "invalid -o value") {
				t.Errorf("pidofParseOmit(%q) error = %q, want an invalid -o value message", c.list, msg)
			}
			continue
		}
		if got := pidofOmitted(ranges, c.pid); got != c.omitted {
			t.Errorf("pidofOmitted(%q, %d) = %v, want %v", c.list, c.pid, got, c.omitted)
		}
	}
}

// pidofTestProc is one process of a synthetic procfs tree.
type pidofTestProc struct {
	pid  int
	comm string
	argv []string
	exe  string
}

func pidofStatLine(pid int, comm string) string {
	id := strconv.Itoa(pid)
	return id + " (" + comm + ") S 1 " + id + " " + id +
		" 0 -1 4194560 0 0 0 0 0 0 0 0 20 0 1 0 10 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n"
}

func pidofWriteTree(t *testing.T, root string, procs []pidofTestProc) {
	t.Helper()
	for _, p := range procs {
		dir := filepath.Join(root, strconv.Itoa(p.pid))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(pidofStatLine(p.pid, p.comm)), 0o644); err != nil {
			t.Fatal(err)
		}
		if p.argv != nil {
			cmdline := strings.Join(p.argv, "\x00") + "\x00"
			if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if p.exe != "" {
			if err := os.Symlink(p.exe, filepath.Join(dir, "exe")); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// pidofFixture installs the seams and a proc tree:
//
//	100 sleep    argv "sleep 60"            (argv[0] has no path)
//	200 sleep    argv "/usr/bin/sleep 30"   (also %PPID)
//	300 init     argv "/sbin/init"
//	400 shscript argv "/bin/sh ./myscript.sh" (comm only matches with -x)
//	500 kthreadd no cmdline, no exe          (a kernel thread)
//	600 renamed  argv "renamed", exe -> the temp "tool" file
//
// It returns the path of the "tool" file for the path-operand tests.
func pidofFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("pidof reads a procfs tree, and that backend is Linux-only")
	}
	root := t.TempDir()
	tool := writeFile(t, t.TempDir(), "tool", "binary")
	pidofWriteTree(t, root, []pidofTestProc{
		{pid: 100, comm: "sleep", argv: []string{"sleep", "60"}},
		{pid: 200, comm: "sleep", argv: []string{"/usr/bin/sleep", "30"}},
		{pid: 300, comm: "init", argv: []string{"/sbin/init"}},
		{pid: 400, comm: "shscript", argv: []string{"/bin/sh", "./myscript.sh"}},
		{pid: 500, comm: "kthreadd"},
		{pid: 600, comm: "renamed", argv: []string{"renamed"}, exe: tool},
	})
	oldRoot, oldPPID := pidofProcRoot, pidofGetppid
	pidofProcRoot, pidofGetppid = root, func() int { return 200 }
	t.Cleanup(func() {
		pidofProcRoot, pidofGetppid = oldRoot, oldPPID
	})
	return tool
}

// pidofRun runs pidof in dir with an optional cancellation context.
func pidofRun(t *testing.T, dir string, base context.Context, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("pidof")
	if !ok {
		t.Fatal("pidof not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    dir,
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
		Ctx:    base,
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestPidofFixtureSingleMatch(t *testing.T) {
	pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil, "init")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "300\n")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestPidofFixtureMultipleMatches(t *testing.T) {
	pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil, "sleep")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "100 200\n")
	if n := strings.Count(r.stdout, "\n"); n != 1 {
		t.Errorf("stdout = %q, want the pids on a single line", r.stdout)
	}

	// A kernel thread has no command line, so it only matches by comm with -x.
	assertExitCode(t, pidofRun(t, t.TempDir(), nil, "kthreadd"), 1)
	assertStdout(t, pidofRun(t, t.TempDir(), nil, "-x", "kthreadd"), "500\n")
}

func TestPidofFixtureSingleShot(t *testing.T) {
	pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil, "-s", "sleep")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "100\n")
	assertStdout(t, pidofRun(t, t.TempDir(), nil, "--single", "sleep"), "100\n")
}

func TestPidofFixtureOmit(t *testing.T) {
	pidofFixture(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"pid", []string{"-o", "100", "sleep"}, "200\n"},
		{"range", []string{"-o", "100-199", "sleep"}, "200\n"},
		{"range covering both", []string{"-o", "1-1000", "sleep"}, ""},
		{"ppid", []string{"-o", "%PPID", "sleep"}, "100\n"},
		{"list", []string{"-o", "300,400", "sleep"}, "100 200\n"},
		{"long option", []string{"--omit=100", "sleep"}, "200\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := pidofRun(t, t.TempDir(), nil, c.args...)
			assertStdout(t, r, c.want)
			if c.want == "" {
				assertExitCode(t, r, 1)
			} else {
				assertExitCode(t, r, 0)
			}
		})
	}

	r := pidofRun(t, t.TempDir(), nil, "-o", "100,200", "sleep")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestPidofFixtureInvalidOmit(t *testing.T) {
	pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil, "-o", "sleep-1", "sleep")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if want := "pidof: invalid -o value \"sleep-1\"\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestPidofFixtureScripts(t *testing.T) {
	pidofFixture(t)
	// comm is the original name, argv[0] is the interpreter, argv[1] the script.
	assertExitCode(t, pidofRun(t, t.TempDir(), nil, "shscript"), 1)
	assertStdout(t, pidofRun(t, t.TempDir(), nil, "-x", "shscript"), "400\n")
	assertExitCode(t, pidofRun(t, t.TempDir(), nil, "myscript.sh"), 1)
	assertStdout(t, pidofRun(t, t.TempDir(), nil, "-x", "myscript.sh"), "400\n")
	// Without -x only argv[0]'s basename counts, which is the shell here.
	assertStdout(t, pidofRun(t, t.TempDir(), nil, "sh"), "400\n")
}

func TestPidofFixturePathOperand(t *testing.T) {
	tool := pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil, tool)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "600\n")

	r = pidofRun(t, filepath.Dir(tool), nil, "./tool")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "600\n")

	// The name alone does not match: pid 600 runs argv[0] "renamed".
	assertExitCode(t, pidofRun(t, t.TempDir(), nil, "tool"), 1)
	// A path whose basename no process runs and that is not pid 600's exe.
	assertExitCode(t, pidofRun(t, t.TempDir(), nil, filepath.Join(t.TempDir(), "absent")), 1)
}

func TestPidofFixtureNoMatch(t *testing.T) {
	pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil, "nosuchprogram")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestPidofNoOperands(t *testing.T) {
	pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil)
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestPidofHelpAndVersion(t *testing.T) {
	pidofFixture(t)
	r := pidofRun(t, t.TempDir(), nil, "--help")
	assertExitCode(t, r, 0)
	for _, want := range []string{"Usage: pidof", "-s, --single", "-o, --omit", "-x, --scripts"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("--help output %q does not contain %q", r.stdout, want)
		}
	}

	r = pidofRun(t, t.TempDir(), nil, "--version")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "pidof (Misa) 0.1.0\n")
}

func TestPidofUnknownOption(t *testing.T) {
	pidofFixture(t)
	for _, arg := range []string{"--bogus", "-Z"} {
		r := pidofRun(t, t.TempDir(), nil, arg, "sleep")
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		if !strings.Contains(r.stderr, "unknown option") {
			t.Errorf("%s: stderr = %q, want an unknown option message", arg, r.stderr)
		}
		if !strings.Contains(r.stderr, "Try 'pidof --help' for more information.") {
			t.Errorf("%s: stderr = %q, want the Try line", arg, r.stderr)
		}
	}
}

func TestPidofSnapshotError(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pidof reads a procfs tree, and that backend is Linux-only")
	}
	oldRoot := pidofProcRoot
	pidofProcRoot = filepath.Join(t.TempDir(), "absent")
	t.Cleanup(func() { pidofProcRoot = oldRoot })

	r := pidofRun(t, t.TempDir(), nil, "sleep")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if !strings.HasPrefix(r.stderr, "pidof: ") {
		t.Errorf("stderr = %q, want a pidof: error message", r.stderr)
	}
}

func TestPidofCancelled(t *testing.T) {
	pidofFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := pidofRun(t, t.TempDir(), ctx, "sleep")
	assertExitCode(t, r, 130)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want a silent cancellation", r.stderr)
	}
}

func TestPidofLiveProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pidof reads a procfs tree, and that backend is Linux-only")
	}
	oldRoot, oldPPID := pidofProcRoot, pidofGetppid
	pidofProcRoot, pidofGetppid = "", os.Getppid
	t.Cleanup(func() { pidofProcRoot, pidofGetppid = oldRoot, oldPPID })

	r := pidofRun(t, t.TempDir(), nil, filepath.Base(os.Args[0]))
	assertExitCode(t, r, 0)
	self := strconv.Itoa(os.Getpid())
	for _, pid := range strings.Fields(r.stdout) {
		if pid == self {
			return
		}
	}
	t.Errorf("stdout = %q, want it to contain our own pid %s", r.stdout, self)
}
