package builtinexec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("BUILTINEXEC_HELPER") == "argv0" {
		fmt.Fprint(os.Stdout, os.Args[0])
		os.Exit(0)
	}
	if os.Getenv("BUILTINEXEC_HELPER") == "env" {
		env := os.Environ()
		sort.Strings(env)
		dir, _ := os.Getwd()
		fmt.Fprint(os.Stdout, strings.Join(env, "\n")+"\n"+dir)
		os.Exit(0)
	}
	if os.Getenv("BUILTINEXEC_HELPER") != "1" {
		return
	}
	if os.Getenv("BUILTINEXEC_WAIT") == "1" {
		fmt.Fprintln(os.Stdout, "ready")
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	b, _ := io.ReadAll(os.Stdin)
	dir, _ := os.Getwd()
	fmt.Fprintf(os.Stdout, "%s|%s|%s", b, os.Getenv("VALUE"), dir)
	fmt.Fprint(os.Stderr, "separate")
	os.Exit(7)
}

func TestExternalInvocation(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	inv := &builtins.Context{Dir: dir, Args: []string{"-test.run=TestHelperProcess"}, Env: []string{"BUILTINEXEC_HELPER=1", "VALUE=local"}, Stdin: strings.NewReader("input"), Stdout: &out, Stderr: &stderr}
	if status := Run(context.Background(), exe, inv); status != 7 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if got, want := out.String(), "input|local|"+dir; got != want {
		t.Fatalf("stdout %q, want %q", got, want)
	}
	if stderr.String() != "separate" {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestResolution(t *testing.T) {
	var stderr bytes.Buffer
	inv := &builtins.Context{Dir: t.TempDir(), Env: []string{}, Stderr: &stderr}
	if got := Run(context.Background(), "misa-definitely-missing-82762", inv); got != 127 {
		t.Fatalf("missing status %d", got)
	}
	if got := Run(context.Background(), "./echo", inv); got != 127 {
		t.Fatalf("explicit missing builtin basename status %d", got)
	}
	if runtime.GOOS == "windows" {
		return
	}
	path := filepath.Join(inv.Dir, "blocked")
	if err := os.WriteFile(path, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := Run(context.Background(), "./blocked", inv); got != 126 {
		t.Fatalf("non-executable status %d", got)
	}
}

func TestNestedEnv(t *testing.T) {
	var out, stderr bytes.Buffer
	inv := &builtins.Context{Args: []string{"-i", "LOCAL=hello", "printenv", "LOCAL"}, Env: []string{"OUTER=untouched"}, Stdout: &out, Stderr: &stderr}
	if got := Run(context.Background(), "env", inv); got != 0 {
		t.Fatalf("status %d: %s", got, stderr.String())
	}
	if out.String() != "hello\n" {
		t.Fatalf("stdout %q", out.String())
	}
	if inv.Exec != nil || len(inv.Env) != 1 || inv.Env[0] != "OUTER=untouched" {
		t.Fatalf("invocation mutated: %+v", inv)
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := Run(ctx, "true", &builtins.Context{}); got != 130 {
		t.Fatalf("cancel status %d", got)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if got := Run(ctx, "true", &builtins.Context{}); got != 124 {
		t.Fatalf("deadline status %d", got)
	}
}

func TestInvocationPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix executable permissions")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(exe, filepath.Join(dir, "fixture")); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	inv := &builtins.Context{Dir: dir, Args: []string{"-test.run=TestHelperProcess"}, Env: []string{"PATH=", "BUILTINEXEC_HELPER=1", "VALUE=path"}, Stdout: &out, Stderr: &stderr}
	if got := Run(context.Background(), "fixture", inv); got != 7 {
		t.Fatalf("status %d: %s", got, stderr.String())
	}
	if !strings.Contains(out.String(), "|path|") {
		t.Fatalf("stdout %q", out.String())
	}
	inv.Env = []string{}
	if got := Run(context.Background(), "fixture", inv); got != 127 {
		t.Fatalf("empty environment status %d", got)
	}
}

func TestEnvNestedPrintenv(t *testing.T) {
	t.Setenv("BUILTINEXEC_ENV_TEST", "parent")
	for _, tc := range []struct {
		args   []string
		want   string
		status int
	}{
		{[]string{"BUILTINEXEC_ENV_TEST=bar", "printenv", "BUILTINEXEC_ENV_TEST"}, "bar\n", 0},
		{[]string{"-i", "printenv", "BUILTINEXEC_ENV_TEST"}, "", 1},
		{[]string{"-i", "printenv"}, "", 0},
	} {
		var out, stderr bytes.Buffer
		got := Run(context.Background(), "env", &builtins.Context{Args: tc.args, Stdout: &out, Stderr: &stderr})
		if got != tc.status || out.String() != tc.want {
			t.Fatalf("args %v: status %d stdout %q stderr %q", tc.args, got, out.String(), stderr.String())
		}
		if os.Getenv("BUILTINEXEC_ENV_TEST") != "parent" {
			t.Fatal("process environment changed")
		}
	}
}

func TestCancelRunningExternal(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, exe, &builtins.Context{Args: []string{"-test.run=TestHelperProcess"}, Env: []string{"BUILTINEXEC_HELPER=1", "BUILTINEXEC_WAIT=1"}, Stdout: writer})
	}()
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(reader).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected handshake: %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child did not start")
	}
	cancel()
	select {
	case status := <-done:
		if status != 130 {
			t.Fatalf("status %d, want 130", status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child did not exit after cancellation")
	}
	// The child's pipe descriptor must be closed, not left open by an abandoned child.
	writer.Close()
	eof := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, reader); eof <- err }()
	select {
	case err := <-eof:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child still holds stdout open")
	}
}

func TestEnvIsolation(t *testing.T) {
	t.Setenv("MISA_ENV_F03", "ambient")
	for _, tc := range []struct {
		args   []string
		want   string
		status int
	}{
		{[]string{"-i", "printenv"}, "", 0},
		{[]string{"-u", "MISA_ENV_F03", "printenv", "MISA_ENV_F03"}, "", 1},
		{[]string{"MISA_ENV_F03=local", "printenv", "MISA_ENV_F03"}, "local\n", 0},
		{[]string{"MISA_ENV_F03=local", "env", "-i", "printenv"}, "", 0},
	} {
		var out, stderr bytes.Buffer
		got := Run(context.Background(), "env", &builtins.Context{Args: tc.args, Stdout: &out, Stderr: &stderr})
		if got != tc.status || out.String() != tc.want {
			t.Errorf("%v: status %d output length %d stderr %q", tc.args, got, out.Len(), stderr.String())
		}
		if os.Getenv("MISA_ENV_F03") != "ambient" {
			t.Fatal("process environment changed")
		}
	}
}

func TestEnvExternalExactEnvironmentAndDir(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	args := []string{"-i", "-C", dir, "BUILTINEXEC_HELPER=env", "ONLY=value", exe, "-test.run=TestHelperProcess"}
	got := Run(context.Background(), "env", &builtins.Context{Args: args, Env: []string{"LEAK=bad"}, Stdout: &out, Stderr: &stderr})
	if got != 0 || out.String() != "BUILTINEXEC_HELPER=env\nONLY=value\n"+dir {
		t.Fatalf("status %d stdout %q stderr %q", got, out.String(), stderr.String())
	}
}

func TestEnvLocalExecutablePrecedesBuiltin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix executable permissions")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(exe, filepath.Join(dir, "echo")); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	inv := &builtins.Context{Dir: t.TempDir(), Args: []string{"-C", dir, "-i", "BUILTINEXEC_HELPER=argv0", "echo", "-test.run=TestHelperProcess"}, Stdout: &out, Stderr: &stderr}
	got := Run(context.Background(), "env", inv)
	if got != 0 || out.String() != "echo" {
		t.Fatalf("status %d output %q stderr %q", got, out.String(), stderr.String())
	}
}

func TestExternalArgv0(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ cmdName, want string }{{"", exe}, {"original-name", "original-name"}} {
		var out, stderr bytes.Buffer
		inv := &builtins.Context{CmdName: tc.cmdName, Args: []string{"-test.run=TestHelperProcess"}, Env: []string{"BUILTINEXEC_HELPER=argv0"}, Stdout: &out, Stderr: &stderr}
		if got := Run(context.Background(), exe, inv); got != 0 || out.String() != tc.want {
			t.Errorf("cmdname %q: status %d output %q stderr %q", tc.cmdName, got, out.String(), stderr.String())
		}
	}
}
