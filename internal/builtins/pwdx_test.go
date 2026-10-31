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
)

// pwdxFixture is a fake procfs root: pid 100 has a cwd link to a real
// directory, pid 200 is a directory without a cwd link, pid 300 does not
// exist, pid 400's cwd link holds a relative target and pid 7 backs the
// leading-zero operand 007.
type pwdxFixture struct {
	root    string
	realDir string
}

func newPwdxFixture(t *testing.T) pwdxFixture {
	t.Helper()
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := func(pid, target string) {
		t.Helper()
		dir := filepath.Join(root, pid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "cwd")); err != nil {
			t.Fatal(err)
		}
	}
	link("100", realDir)
	if err := os.MkdirAll(filepath.Join(root, "200"), 0o755); err != nil {
		t.Fatal(err)
	}
	link("400", "relative/dir")
	link("7", realDir)
	return pwdxFixture{root: root, realDir: realDir}
}

func pwdxUseProcRoot(t *testing.T, root string) {
	t.Helper()
	pwdxProcRoot = root
	t.Cleanup(func() { pwdxProcRoot = "" })
}

func TestPwdxOperands(t *testing.T) {
	fx := newPwdxFixture(t)
	pwdxUseProcRoot(t, fx.root)

	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		stdout string
	}{
		{
			name:   "single pid",
			args:   []string{"100"},
			code:   0,
			stdout: "100: " + fx.realDir + "\n",
		},
		{
			name:   "several pids in argument order",
			args:   []string{"400", "100"},
			code:   0,
			stdout: "400: relative/dir\n100: " + fx.realDir + "\n",
		},
		{
			name: "valid and invalid pids mixed",
			args: []string{"100", "200", "300"},
			code: 1,
			stdout: "100: " + fx.realDir + "\n" +
				"200: No such file or directory\n" +
				"300: No such file or directory\n",
		},
		{
			name:   "relative target printed verbatim",
			args:   []string{"400"},
			code:   0,
			stdout: "400: relative/dir\n",
		},
		{
			name:   "non numeric operand",
			args:   []string{"abc"},
			code:   1,
			stdout: "abc: Invalid process id\n",
		},
		{
			name:   "numeric zero is not treated as non numeric",
			args:   []string{"0"},
			code:   1,
			stdout: "0: No such file or directory\n",
		},
		{
			name:   "duplicate pid printed twice",
			args:   []string{"100", "100"},
			code:   0,
			stdout: "100: " + fx.realDir + "\n100: " + fx.realDir + "\n",
		},
		{
			name:   "pid printed as written",
			args:   []string{"007"},
			code:   0,
			stdout: "007: " + fx.realDir + "\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltin(t, "pwdx", tc.args...)
			assertExitCode(t, r, tc.code)
			assertStdout(t, r, tc.stdout)
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestPwdxMissingOperand(t *testing.T) {
	r := runBuiltin(t, "pwdx")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "pwdx: missing operand")
	assertStderrContains(t, r, "Try 'pwdx --help' for more information.")
}

func TestPwdxUnknownOption(t *testing.T) {
	r := runBuiltin(t, "pwdx", "-a")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "unknown option -a")
	assertStderrContains(t, r, "Try 'pwdx --help' for more information.")
}

func TestPwdxHelp(t *testing.T) {
	r := runBuiltin(t, "pwdx", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "usage: pwdx PID...")
	assertStdoutContains(t, r, "Print working directory")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestPwdxVersion(t *testing.T) {
	r := runBuiltin(t, "pwdx", "--version")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "pwdx (Misa) 0.1.0\n")
}

func TestPwdxAbsentRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "no-proc")
	pwdxUseProcRoot(t, root)

	r := runBuiltin(t, "pwdx", "1")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, root)
}

func TestPwdxCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	code := Run(ctx, "pwdx", &Context{
		Args:   []string{"1"},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 130 {
		t.Errorf("exit code = %d, want 130", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("cancelled run wrote stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestPwdxLiveSelfPID(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pwdx reads a procfs tree; skipping on " + runtime.GOOS)
	}
	pwdxUseProcRoot(t, "")

	pid := strconv.Itoa(os.Getpid())
	r := runBuiltin(t, "pwdx", pid)
	assertExitCode(t, r, 0)
	if !strings.HasPrefix(r.stdout, pid+": /") {
		t.Errorf("stdout = %q, want prefix %q", r.stdout, pid+": /")
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}
