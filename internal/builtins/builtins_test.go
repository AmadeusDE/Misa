package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type runResult struct {
	code   int
	stdout string
	stderr string
}

func runBuiltin(t *testing.T, name string, args ...string) runResult {
	t.Helper()
	return runBuiltinIn(t, t.TempDir(), name, args...)
}

func runBuiltinIn(t *testing.T, dir string, name string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
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
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func runBuiltinStdin(t *testing.T, name string, stdin string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func assertExitCode(t *testing.T, r runResult, want int) {
	t.Helper()
	if r.code != want {
		t.Errorf("exit code = %d, want %d (stderr: %s)", r.code, want, r.stderr)
	}
}

func assertStdout(t *testing.T, r runResult, want string) {
	t.Helper()
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func assertStdoutContains(t *testing.T, r runResult, substr string) {
	t.Helper()
	if !strings.Contains(r.stdout, substr) {
		t.Errorf("stdout %q does not contain %q", r.stdout, substr)
	}
}

func runBuiltinStdinIn(t *testing.T, dir string, name string, stdin string, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    dir,
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func assertStderrContains(t *testing.T, r runResult, substr string) {
	t.Helper()
	if !strings.Contains(r.stderr, substr) {
		t.Errorf("stderr %q does not contain %q", r.stderr, substr)
	}
}
