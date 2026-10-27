package native

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestFalse_ExitOne(t *testing.T) {
	r := run(t, "false")
	wantCode(t, r, 1)
}

func TestYes_ProducesOutput(t *testing.T) {
	r := runStdin(t, "yes", "")
	if len(r.stdout) == 0 {
		t.Skip("yes may not produce output without tty")
	}
	wantOutContains(t, r, "y")
}

func TestArch_ReturnsArch(t *testing.T) {
	r := run(t, "arch")
	wantSuccess(t, r)
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected arch output")
	}
}

func TestHostname_ReturnsHostname(t *testing.T) {
	r := run(t, "hostname")
	wantSuccess(t, r)
	host, _ := os.Hostname()
	wantOut(t, r, host+"\n")
}

func TestWhoami_ReturnsUser(t *testing.T) {
	r := run(t, "whoami")
	wantSuccess(t, r)
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected username")
	}
}

func TestNproc_ReturnsNumber(t *testing.T) {
	r := run(t, "nproc")
	wantSuccess(t, r)
	if n, err := strconv.Atoi(strings.TrimSpace(r.stdout)); err != nil || n < 1 {
		t.Errorf("nproc printed %q, want a positive number", r.stdout)
	}
}

func TestUname_Default(t *testing.T) {
	r := run(t, "uname")
	wantSuccess(t, r)
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected uname output")
	}
}

func TestUname_All(t *testing.T) {
	r := run(t, "uname", "-a")
	wantSuccess(t, r)
	if len(strings.Fields(r.stdout)) < 3 {
		t.Errorf("expected multiple fields for uname -a, got %q", r.stdout)
	}
}

func TestPwd_ReturnsDir(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "pwd")
	wantSuccess(t, r)
	got := strings.TrimSpace(r.stdout)
	abs, _ := filepath.EvalSymlinks(dir)
	gotAbs, _ := filepath.EvalSymlinks(got)
	if gotAbs != abs {
		t.Errorf("pwd = %q, want %q", gotAbs, abs)
	}
}

func TestPwd_Physical(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "a", "b")
	os.MkdirAll(targetDir, 0755)
	symlink(t, "a/b", filepath.Join(dir, "c"))

	linkDir := filepath.Join(dir, "c")
	cmd, ok := builtins.Lookup("pwd")
	if !ok {
		t.Fatal("pwd not registered")
	}
	var stdout, stderr strings.Builder
	ctx := &builtins.Context{
		Dir:    linkDir,
		Args:   []string{"-P"},
		Stdout: &stdout,
		Stderr: &stderr,
	}
	code := cmd.Run(ctx)
	if code != 0 {
		t.Fatalf("pwd -P exit code = %d", code)
	}
	phys, _ := filepath.EvalSymlinks(targetDir)
	if strings.TrimSpace(stdout.String()) != phys {
		t.Errorf("pwd -P = %q, want %q", strings.TrimSpace(stdout.String()), phys)
	}
}

func TestPwd_IgnoringArguments(t *testing.T) {
	r := run(t, "pwd", "extra", "args")
	wantSuccess(t, r)
	wantErrContains(t, r, "pwd: ignoring non-option arguments")
}

func TestLogname_ReturnsName(t *testing.T) {
	r := run(t, "logname")
	if r.code != 0 {
		t.Skip("logname may not work in test environment")
	}
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected logname output")
	}
}

func TestTty_NotATty(t *testing.T) {
	r := run(t, "tty")
	wantFail(t, r)
	wantOutContains(t, r, "not a tty")
}

func TestGroups_ReturnsGroups(t *testing.T) {
	r := run(t, "groups")
	wantSuccess(t, r)
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected groups output")
	}
}

func TestId_Default(t *testing.T) {
	r := run(t, "id")
	wantSuccess(t, r)
	wantOutContains(t, r, "uid=")
	wantOutContains(t, r, "gid=")
}

func TestId_UserName(t *testing.T) {
	r := run(t, "id", "-un")
	wantSuccess(t, r)
	if len(strings.TrimSpace(r.stdout)) == 0 {
		t.Error("expected username")
	}
}

func TestPrintenv_All(t *testing.T) {
	t.Setenv("MISA_NATIVE_TEST", "hello")
	r := run(t, "printenv")
	wantSuccess(t, r)
	wantOutContains(t, r, "MISA_NATIVE_TEST=hello")
}

func TestPrintenv_One(t *testing.T) {
	t.Setenv("MISA_NATIVE_TEST2", "val42")
	r := run(t, "printenv", "MISA_NATIVE_TEST2")
	wantSuccess(t, r)
	wantOut(t, r, "val42\n")
}

func TestPrintenv_Missing(t *testing.T) {
	r := run(t, "printenv", "MISA_NONEXISTENT_99999")
	wantFail(t, r)
}

func TestWhich_FindsBuiltin(t *testing.T) {
	r := run(t, "which", "cat")
	wantSuccess(t, r)
}

func TestSleep_ZeroDuration(t *testing.T) {
	r := run(t, "sleep", "0")
	wantSuccess(t, r)
}

func TestSleep_SubSecond(t *testing.T) {
	r := run(t, "sleep", "0.01")
	wantSuccess(t, r)
}

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, os.ErrPermission
}

func TestYes_WriteError(t *testing.T) {
	cmd, ok := builtins.Lookup("yes")
	if !ok {
		t.Fatal("yes builtin not registered")
	}
	var stderr strings.Builder
	ctx := &builtins.Context{
		Dir:    t.TempDir(),
		Args:   nil,
		Stdout: errWriter{},
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	}
	code := cmd.Run(ctx)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "yes: standard output:") {
		t.Errorf("stderr %q does not contain 'yes: standard output:'", stderr.String())
	}
}
