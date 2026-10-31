package builtins

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestEnvDefault(t *testing.T) {
	r := runBuiltin(t, "env")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "=") {
		t.Errorf("expected KEY=VALUE output, got %q", r.stdout)
	}
}

func TestEnvContainsPath(t *testing.T) {
	r := runBuiltin(t, "env")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "PATH=")
}

func TestEnvUnset(t *testing.T) {
	key := "MISA_TEST_ENV_VAR_12345"
	os.Setenv(key, "hello")
	defer os.Unsetenv(key)

	r := runBuiltin(t, "env", "-u", key)
	assertExitCode(t, r, 0)
	if strings.Contains(r.stdout, key+"=") {
		t.Errorf("expected %s to be unset, but found it in output", key)
	}
}

func TestEnvIgnore(t *testing.T) {
	r := runBuiltin(t, "env", "-i")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestEnvNull(t *testing.T) {
	r := runBuiltin(t, "env", "-0")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "\x00") {
		t.Error("expected null-separated output with -0")
	}
}

func TestEnvFallbackDoesNotMutateProcess(t *testing.T) {
	t.Setenv("MISA_ENV_F03", "ambient")
	var out, errout bytes.Buffer
	c := &Context{Args: []string{"-i", "MISA_ENV_F03=local", "printenv", "MISA_ENV_F03"}, Stdout: &out, Stderr: &errout}
	if got := runEnv(c); got != 0 || out.String() != "local\n" || os.Getenv("MISA_ENV_F03") != "ambient" {
		t.Fatalf("status %d output %q stderr %q", got, out.String(), errout.String())
	}
}

func TestEnvExecutorReceivesEmptyEnvironment(t *testing.T) {
	var out, errout bytes.Buffer
	called := false
	c := &Context{Args: []string{"-i", "some-command"}, Stdout: &out, Stderr: &errout}
	c.Exec = func(_ context.Context, name string, sub *Context) int {
		called = true
		if name != "some-command" || sub.Env == nil || len(sub.Env) != 0 {
			t.Errorf("name %q environment %#v", name, sub.Env)
		}
		return 42
	}
	if got := runEnv(c); got != 42 || !called {
		t.Fatalf("status %d called %v", got, called)
	}
}

var envFallbackChildInitialEnv = os.Environ()

func TestEnvFallbackExternalHelper(t *testing.T) {
	if os.Getenv("MISA_ENV_F03_CHILD") != "yes" {
		return
	}
	entries := envFallbackChildInitialEnv
	sort.Strings(entries)
	fmt.Fprint(os.Stdout, strings.Join(entries, "\n"))
	os.Exit(0)
}

func TestEnvFallbackExternal(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	c := &Context{Dir: t.TempDir(), Env: []string{"LEAK=bad"}, Args: []string{"-i", "MISA_ENV_F03_CHILD=yes", exe, "-test.run=^TestEnvFallbackExternalHelper$"}, Stdout: &out, Stderr: &errout}
	if got := runEnv(c); got != 0 || out.String() != "MISA_ENV_F03_CHILD=yes" {
		t.Fatalf("status %d output %q stderr %q", got, out.String(), errout.String())
	}
}

func TestEnvExecutorLocalResolution(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "echo"), []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blocked"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, want string }{
		{"echo", "echo"}, {"blocked", "blocked"}, {"./echo", "./echo"}, {"missing", "missing"},
	} {
		var out, errout bytes.Buffer
		called := false
		c := &Context{Dir: dir, Args: []string{tc.name}, Stdout: &out, Stderr: &errout}
		c.Exec = func(_ context.Context, name string, sub *Context) int {
			called = true
			if name != tc.want || sub.CmdName != tc.name {
				t.Errorf("%q: name %q argv0 %q", tc.name, name, sub.CmdName)
			}
			return 0
		}
		if got := runEnv(c); got != 0 || !called {
			t.Errorf("%q: status %d called %v", tc.name, got, called)
		}
	}
}

func TestEnvResolutionPolicy(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"echo", "probe", "malformed"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("not an executable format\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name   string
		env    []string
		args   []string
		status int
		out    string
	}{
		{"builtin over cwd", []string{"PATH="}, []string{"echo", "safe"}, 0, "safe\n"},
		{"unset path", []string{}, []string{"probe"}, 127, ""},
		{"empty path searches cwd", []string{"PATH="}, []string{"probe"}, 126, ""},
		{"explicit path is external", []string{}, []string{"./echo", "unsafe"}, 126, ""},
		{"no enoexec shell", []string{"PATH="}, []string{"malformed"}, 126, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			c := &Context{Dir: dir, Env: tc.env, Args: tc.args, Stdout: &out, Stderr: &errout}
			if got := runEnv(c); got != tc.status || out.String() != tc.out {
				t.Fatalf("status %d stdout %q stderr %q", got, out.String(), errout.String())
			}
		})
	}
}

func TestEnvExternalPathSnapshot(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(exe, filepath.Join(dir, "f06-probe")); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	c := &Context{Dir: dir, Env: []string{"LEAK=bad"}, Args: []string{"-i", "PATH=", "MISA_ENV_F03_CHILD=yes", "f06-probe", "-test.run=^TestEnvFallbackExternalHelper$"}, Stdout: &out, Stderr: &errout}
	if got := runEnv(c); got != 0 || out.String() != "MISA_ENV_F03_CHILD=yes\nPATH=" {
		t.Fatalf("status %d stdout %q stderr %q", got, out.String(), errout.String())
	}
}

func TestEnvExternalLaunchDiagnostics(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		status int
		stderr string
	}{
		{"./empty", 126, "env: './empty': Permission denied\n"},
		{"missing", 127, "env: 'missing': No such file or directory\n"},
		{"'-v '", 127, "env: '-v ': No such file or directory\nenv: use -[v]S to pass options in shebang lines\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			c := &Context{Dir: dir, Env: []string{}, Args: []string{tc.name}, Stdout: &out, Stderr: &errout}
			if got := runEnv(c); got != tc.status || errout.String() != tc.stderr {
				t.Fatalf("status %d stderr %q, want %d %q", got, errout.String(), tc.status, tc.stderr)
			}
		})
	}
}
