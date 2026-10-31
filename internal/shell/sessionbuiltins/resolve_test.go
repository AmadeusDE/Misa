package sessionbuiltins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/shell/session"
)

// TestHelperProcess is the fake external command these tests create on PATH. It
// is a symlink to this test binary, so the tests need no shell script and no
// host utility.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("SESSIONBUILTINS_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "external-ran")
	os.Exit(3)
}

// fakeExecutable links the test binary under name in dir and checks that it can
// run, skipping the test where exec permission is unavailable.
func fakeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix executable permissions")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("no test executable: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.Symlink(exe, path); err != nil {
		t.Skipf("cannot link fake executable: %v", err)
	}
	cmd := exec.Command(path, "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(), "SESSIONBUILTINS_HELPER=1")
	var exitErr *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Skipf("fake executable cannot run here: %v", err)
	}
	return path
}

// builtinsSession is a session wired exactly like `misa sh`: F06 execution plus
// the registry-aware resolution hooks.
func builtinsSession(t *testing.T, dir string, env []string, opt func(*session.Options)) (*session.Session, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, stderr bytes.Buffer
	options := Configure(&session.Options{Dir: dir, Env: env, Stdout: &out, Stderr: &stderr})
	if opt != nil {
		opt(options)
	}
	s, err := session.New(*options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, &out, &stderr
}

func runSource(t *testing.T, s *session.Session, out, stderr *bytes.Buffer, source string) (string, string, int) {
	t.Helper()
	out.Reset()
	stderr.Reset()
	r := s.Run(context.Background(), source)
	if r.Err != nil {
		t.Fatalf("%q: %v", source, r.Err)
	}
	return out.String(), stderr.String(), r.Status
}

// isolatedCase is one shell submission with its expected result.
type isolatedCase struct {
	name, source, want, wantErr string
	status                      int
	// reject, when non-empty, must not appear in the output.
	reject string
}

// runIsolated runs every case in its own session, so functions and hash entries
// cannot leak from one case to the next.
func runIsolated(t *testing.T, dir string, env []string, opt func(*session.Options), cases []isolatedCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, out, stderr := builtinsSession(t, dir, env, opt)
			got, errOut, status := runSource(t, s, out, stderr, tc.source)
			if got != tc.want || status != tc.status {
				t.Fatalf("%q: out %q status %d, want %q %d (stderr %q)", tc.source, got, status, tc.want, tc.status, errOut)
			}
			if tc.wantErr != "" && !strings.Contains(errOut, tc.wantErr) {
				t.Fatalf("%q: stderr %q, want it to mention %q", tc.source, errOut, tc.wantErr)
			}
			if tc.reject != "" && strings.Contains(got, tc.reject) {
				t.Fatalf("%q: output %q must not contain %q", tc.source, got, tc.reject)
			}
		})
	}
}

func TestMisaUtilityShadowsPath(t *testing.T) {
	dir := t.TempDir()
	fakeExecutable(t, dir, "rev")
	env := []string{"PATH=" + dir, "SESSIONBUILTINS_HELPER=1"}
	runIsolated(t, dir, env, nil, []isolatedCase{
		{"misa utility wins", "printf abc | rev", "cba", "", 0, "external-ran"},
		{"command keeps the misa utility", "printf abc | command rev", "cba", "", 0, "external-ran"},
		{"explicit path is external", "printf abc | " + filepath.Join(dir, "rev") + " -test.run=TestHelperProcess", "external-ran\n", "", 3, ""},
		{"explicit relative path is external", "./rev -test.run=TestHelperProcess", "external-ran\n", "", 3, ""},
		{"type reports the utility", "type rev", "rev is a misa builtin\n", "", 0, ""},
		{"path search ignores the registry", "type -p rev", filepath.Join(dir, "rev") + "\n", "", 0, ""},
	})
}

func TestFunctionAndBuiltinCollisions(t *testing.T) {
	dir := t.TempDir()
	fakeExecutable(t, dir, "rev")
	env := []string{"PATH=" + dir, "SESSIONBUILTINS_HELPER=1"}
	runIsolated(t, dir, env, nil, []isolatedCase{
		{"function wins", "rev() { echo function; }; rev", "function\n", "", 0, "external-ran"},
		{"type sees the function", "rev() { echo function; }; type rev", "rev is a function\n", "", 0, ""},
		{"command bypasses the function", "rev() { echo function; }; command rev", "", "", 0, "function"},
		{"command -v sees the function", "rev() { echo function; }; command -v rev", "rev\n", "", 0, ""},
		{"function beats a shell builtin", "echo() { command echo function; }; echo hi", "function\n", "", 0, ""},
		{"command reaches the shell builtin", "echo() { command echo function; }; command echo hi", "hi\n", "", 0, ""},
		{"shell builtin precedes misa and path", "command -V echo", "echo is a shell builtin\n", "", 0, ""},
	})
}

func TestPathExecutablesAndStatuses(t *testing.T) {
	dir := t.TempDir()
	path := fakeExecutable(t, dir, "tool")
	if err := os.WriteFile(filepath.Join(dir, "blocked"), []byte("not executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + dir, "SESSIONBUILTINS_HELPER=1"}
	runIsolated(t, dir, env, nil, []isolatedCase{
		{"path lookup", "tool -test.run=TestHelperProcess", "external-ran\n", "", 3, ""},
		{"type reports the path", "type tool", "tool is " + path + "\n", "", 0, ""},
		{"command -v reports the path", "command -v tool", path + "\n", "", 0, ""},
		{"missing command", "absent-command", "", "executable file not found in $PATH", 127, ""},
		{"missing explicit path", "./absent-command", "", "no such file or directory", 127, ""},
		{"non-executable explicit path", "./blocked", "", "permission denied", 126, ""},
		{"type of a missing name", "type absent-command", "", "type: absent-command: not found\n", 1, ""},
	})
}

func TestCommandDefaultPathNativeEscape(t *testing.T) {
	defaultDir := t.TempDir()
	fakeExecutable(t, defaultDir, "rev")
	fakeExecutable(t, defaultDir, "external")
	emptyDir := t.TempDir()
	env := []string{"PATH=" + emptyDir, "SESSIONBUILTINS_HELPER=1"}
	opt := func(o *session.Options) { o.DefaultPath = defaultDir }
	runIsolated(t, emptyDir, env, opt, []isolatedCase{
		{"default path finds the external", "command -p external -test.run=TestHelperProcess", "external-ran\n", "", 3, ""},
		{"default path skips the misa utility", "command -p rev -test.run=TestHelperProcess", "external-ran\n", "", 3, ""},
		{"session path does not find it", "external", "", "not found", 127, ""},
		{"missing from the default path", "command -p echo hi", "", "command: echo: not found\n", 127, ""},
	})
}

func TestHashAcrossExecutions(t *testing.T) {
	dir := t.TempDir()
	path := fakeExecutable(t, dir, "tool")
	env := []string{"PATH=" + dir, "SESSIONBUILTINS_HELPER=1"}
	s, out, stderr := builtinsSession(t, dir, env, nil)
	if got, _, _ := runSource(t, s, out, stderr, "hash"); got != "" {
		t.Fatalf("fresh table %q", got)
	}
	runSource(t, s, out, stderr, "tool -test.run=TestHelperProcess")
	if got, _, _ := runSource(t, s, out, stderr, "hash"); got != "tool="+path+"\n" {
		t.Fatalf("recorded table %q", got)
	}
	if got, _, _ := runSource(t, s, out, stderr, "hash -r; hash"); got != "" {
		t.Fatalf("hash -r did not clear: %q", got)
	}
	runSource(t, s, out, stderr, "hash tool; PATH=/nonexistent-s04-path")
	if got, _, _ := runSource(t, s, out, stderr, "hash"); got != "" {
		t.Fatalf("PATH assignment did not clear the table: %q", got)
	}
}
