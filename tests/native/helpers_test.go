package native

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

type result struct {
	code   int
	stdout string
	stderr string
}

func run(t *testing.T, name string, args ...string) result {
	t.Helper()
	return runIn(t, t.TempDir(), name, args...)
}

func runIn(t *testing.T, dir string, name string, args ...string) result {
	t.Helper()
	cmd, ok := builtins.Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stdout, stderr bytes.Buffer
	ctx := &builtins.Context{
		Dir:    dir,
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	}
	code := cmd.Run(ctx)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func runInWithUmask(t *testing.T, dir string, mask int, name string, args ...string) result {
	t.Helper()
	oldUmask := syscall.Umask(mask)
	defer syscall.Umask(oldUmask)
	return runIn(t, dir, name, args...)
}

func runStdin(t *testing.T, name string, stdin string, args ...string) result {
	t.Helper()
	return runStdinIn(t, t.TempDir(), name, stdin, args...)
}

func runStdinIn(t *testing.T, dir string, name string, stdin string, args ...string) result {
	t.Helper()
	cmd, ok := builtins.Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stdout, stderr bytes.Buffer
	ctx := &builtins.Context{
		Dir:    dir,
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func runStdinBytes(t *testing.T, name string, stdin []byte, args ...string) result {
	t.Helper()
	cmd, ok := builtins.Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stdout, stderr bytes.Buffer
	ctx := &builtins.Context{
		Dir:    t.TempDir(),
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  bytes.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func runStdoutBytes(t *testing.T, name string, stdin string, args ...string) (int, []byte, string) {
	t.Helper()
	cmd, ok := builtins.Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stdout, stderr bytes.Buffer
	ctx := &builtins.Context{
		Dir:    t.TempDir(),
		Args:   args,
		Width:  80,
		Height: 24,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(stdin),
	}
	code := cmd.Run(ctx)
	return code, stdout.Bytes(), stderr.String()
}

func mkFile(t *testing.T, dir, name, content string) string {
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

func mkFileMode(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	p := mkFile(t, dir, name, content)
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func wantCode(t *testing.T, r result, want int) {
	t.Helper()
	if r.code != want {
		t.Errorf("exit code = %d, want %d (stderr: %s)", r.code, want, r.stderr)
	}
}

func wantOut(t *testing.T, r result, want string) {
	t.Helper()
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func wantOutContains(t *testing.T, r result, substr string) {
	t.Helper()
	if !strings.Contains(r.stdout, substr) {
		t.Errorf("stdout %q does not contain %q", r.stdout, substr)
	}
}

func wantErr(t *testing.T, r result, want string) {
	t.Helper()
	if r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func wantErrContains(t *testing.T, r result, substr string) {
	t.Helper()
	if !strings.Contains(r.stderr, substr) {
		t.Errorf("stderr %q does not contain %q", r.stderr, substr)
	}
}

func wantSuccess(t *testing.T, r result) {
	t.Helper()
	wantCode(t, r, 0)
}

func wantFail(t *testing.T, r result) {
	t.Helper()
	if r.code == 0 {
		t.Errorf("expected non-zero exit code, got 0")
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// devNull returns an io.Writer that discards everything.
func devNull() io.Writer {
	return io.Discard
}
