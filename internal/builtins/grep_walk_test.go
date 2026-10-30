//go:build unix

package builtins

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func grepTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"src/sub", "src/vendor", "other"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}
	writeFile(t, dir, "src/a.go", "hit a\n")
	writeFile(t, dir, "src/b.txt", "hit b\n")
	writeFile(t, dir, "src/sub/c.go", "hit c\nmiss\n")
	writeFile(t, dir, "src/vendor/d.go", "hit d\n")
	writeFile(t, dir, "other/e.go", "hit e\n")
	return dir
}

func TestGrepRecursive(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
	}{
		{"dir operand", []string{"-r", "hit", "src"}, 0,
			"src/a.go:hit a\nsrc/b.txt:hit b\nsrc/sub/c.go:hit c\nsrc/vendor/d.go:hit d\n"},
		{"no operand searches cwd without ./", []string{"-r", "hit e"}, 0, "other/e.go:hit e\n"},
		{"dot operand keeps ./", []string{"-r", "hit e", "."}, 0, "./other/e.go:hit e\n"},
		{"trailing slash", []string{"-r", "hit a", "src/"}, 0, "src/a.go:hit a\n"},
		{"file operand has no name", []string{"-r", "hit", "src/a.go"}, 0, "hit a\n"},
		{"-h drops names", []string{"-rh", "hit c", "src"}, 0, "hit c\n"},
		{"-l lists", []string{"-rl", "hit", "src/sub", "other"}, 0, "src/sub/c.go\nother/e.go\n"},
		{"-c counts", []string{"-rc", "hit", "src/sub"}, 0, "src/sub/c.go:1\n"},
		{"include", []string{"-r", "--include=*.txt", "hit", "src"}, 0, "src/b.txt:hit b\n"},
		{"exclude", []string{"-r", "--exclude=*.go", "hit", "src"}, 0, "src/b.txt:hit b\n"},
		{"last match wins", []string{"-r", "--exclude=*.go", "--include=a.*", "hit", "src"}, 0,
			"src/a.go:hit a\nsrc/b.txt:hit b\n"},
		{"exclude-dir", []string{"-r", "--exclude-dir=vendor", "--exclude-dir=sub", "hit", "src"}, 0,
			"src/a.go:hit a\nsrc/b.txt:hit b\n"},
		{"exclude-dir glob", []string{"-r", "--exclude-dir=v*", "hit d", "src"}, 1, ""},
		{"exclude-dir operand", []string{"-r", "--exclude-dir=other", "hit e", "other"}, 1, ""},
		{"exclude applies to file operands", []string{"--exclude=*.go", "hit", "src/a.go", "src/b.txt"}, 0,
			"src/b.txt:hit b\n"},
		{"-q stops early", []string{"-rq", "hit", "src"}, 0, ""},
		{"no match", []string{"-r", "nothing", "src"}, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltinIn(t, grepTree(t), "grep", tt.args...)
			assertExitCode(t, r, tt.code)
			assertStdout(t, r, tt.stdout)
		})
	}
}

func TestGrepExcludeFrom(t *testing.T) {
	dir := grepTree(t)
	writeFile(t, dir, "pats", "*.go\nnothing\n")
	r := runBuiltinIn(t, dir, "grep", "-r", "--exclude-from=pats", "hit", "src")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "src/b.txt:hit b\n")

	r = runBuiltinIn(t, dir, "grep", "-r", "--exclude-from=missing", "hit", "src")
	assertExitCode(t, r, 2)
	assertStderrContains(t, r, "missing: No such file or directory")
}

func TestGrepRecursiveSymlinks(t *testing.T) {
	dir := grepTree(t)
	os.Symlink("../other", filepath.Join(dir, "src", "linkdir"))
	os.Symlink("../other/e.go", filepath.Join(dir, "src", "linkfile"))
	os.Symlink("nowhere", filepath.Join(dir, "src", "dangling"))
	os.Symlink("src", filepath.Join(dir, "srclink"))

	r := runBuiltinIn(t, dir, "grep", "-r", "hit e", "src")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")

	r = runBuiltinIn(t, dir, "grep", "-r", "hit a", "srclink")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "srclink/a.go:hit a\n")

	r = runBuiltinIn(t, dir, "grep", "-R", "hit e", "src")
	assertExitCode(t, r, 2)
	assertStdout(t, r, "src/linkdir/e.go:hit e\nsrc/linkfile:hit e\n")
	assertStderrContains(t, r, "grep: src/dangling: No such file or directory")
}

func TestGrepRecursiveLoop(t *testing.T) {
	dir := grepTree(t)
	os.Symlink("..", filepath.Join(dir, "src", "sub", "up"))
	r := runBuiltinIn(t, dir, "grep", "-R", "hit c", "src")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "src/sub/c.go:hit c\n")
	assertStderrContains(t, r, "grep: src/sub/up: warning: recursive directory loop")
}

func TestGrepRecursiveSkipsFIFO(t *testing.T) {
	dir := grepTree(t)
	if err := syscall.Mkfifo(filepath.Join(dir, "src", "fifo"), 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	r := runBuiltinIn(t, dir, "grep", "-rh", "hit a", "src")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hit a\n")
}

func TestGrepRecursiveUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	dir := grepTree(t)
	locked := filepath.Join(dir, "src", "sub")
	os.Chmod(locked, 0)
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	r := runBuiltinIn(t, dir, "grep", "-r", "hit a", "src")
	assertExitCode(t, r, 2)
	assertStdout(t, r, "src/a.go:hit a\n")
	assertStderrContains(t, r, "grep: src/sub: Permission denied")

	r = runBuiltinIn(t, dir, "grep", "-rs", "hit a", "src")
	assertExitCode(t, r, 2)
	if r.stderr != "" {
		t.Errorf("-s: stderr = %q", r.stderr)
	}
}
