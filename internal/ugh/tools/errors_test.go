package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// C3: read on a directory used to return "read : read <cwd>: is a directory" —
// an empty name before the colon and no useful direction. It must name the
// path, say it is a directory, and suggest ls.
func TestReadDirectorySuggestsLs(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "inner.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, "read", dir, `{"path":"sub"}`)
	if err == nil {
		t.Fatal("read on a directory did not error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "read sub:") || !strings.Contains(msg, "directory") || !strings.Contains(msg, "ls") {
		t.Fatalf("error = %q, want the path, 'directory' and an ls hint", msg)
	}
	if strings.Contains(msg, "read :") {
		t.Fatalf("malformed error prefix in %q", msg)
	}
}

// C3: a missing or empty path errors with what was received and the expected
// shape, never a bare "read :" that resolves to the working directory.
func TestReadEmptyPathNamesArgs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inner.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string]string{
		"missing key": `{"file":"inner.txt"}`,
		"empty":       `{"path":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := run(t, "read", dir, args)
			if err == nil {
				t.Fatal("empty path did not error")
			}
			msg := err.Error()
			if !strings.Contains(msg, "path must not be empty") || !strings.Contains(msg, `"path"`) || !strings.Contains(msg, "args:") {
				t.Fatalf("error = %q, want what was received and the expected {\"path\": ...}", msg)
			}
			if strings.Contains(msg, "read :") {
				t.Fatalf("malformed error prefix in %q", msg)
			}
		})
	}
}

// C3: the empty bash error states what was received and what is expected.
func TestBashEmptyCommandStatesArgs(t *testing.T) {
	dir := t.TempDir()
	for name, args := range map[string]string{
		"whitespace": `{"command":"  "}`,
		"wrong key":  `{"cmd":"ls"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := run(t, "bash", dir, args)
			if err == nil {
				t.Fatal("empty command did not error")
			}
			msg := err.Error()
			if !strings.Contains(msg, "command must not be empty") {
				t.Fatalf("error = %q", msg)
			}
			if !strings.Contains(msg, "args:") || !strings.Contains(msg, `"command"`) {
				t.Fatalf("error = %q, want the received args and the expected {\"command\": ...}", msg)
			}
		})
	}
}
