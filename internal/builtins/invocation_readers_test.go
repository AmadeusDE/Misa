package builtins

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvocationEnvironmentReaders(t *testing.T) {
	t.Setenv("POSIXLY_CORRECT", "1")
	t.Setenv("LOGNAME", "host-user")
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("SIMPLE_BACKUP_SUFFIX", ".host")
	t.Setenv("VERSION_CONTROL", "numbered")
	t.Setenv("HOME", t.TempDir())
	var out, errout bytes.Buffer
	ctx := &Context{Env: []string{}, Stdout: &out, Stderr: &errout, Stdin: strings.NewReader("")}
	if code := runPrintenv(ctx); code != 0 || out.Len() != 0 {
		t.Errorf("empty printenv: %d %q", code, out.String())
	}
	out.Reset()
	ctx.Args = []string{"-n", "hello"}
	if code := runEcho(ctx); code != 0 || out.String() != "hello" {
		t.Errorf("echo: %d %q", code, out.String())
	}
	out.Reset()
	ctx.Args = nil
	if code := runLogname(ctx); code == 0 && strings.TrimSpace(out.String()) == "host-user" {
		t.Errorf("logname inherited host: %q", out.String())
	}
	ctx.Env = []string{"SIMPLE_BACKUP_SUFFIX=.local", "VERSION_CONTROL=simple"}
	flags, e := cpSpecParsed.ParseArgs([]string{"-b"})
	if e != nil {
		t.Fatal(e)
	}
	if got := backupSuffix(ctx, flags); got != ".local" {
		t.Errorf("suffix %q", got)
	}
	if got, _, _ := backupControl(ctx, flags); got != "simple" {
		t.Errorf("control %q", got)
	}
	if got := lnBackupSuffix(ctx, "", false); got != ".local" {
		t.Errorf("ln suffix %q", got)
	}
	ctx.Env = []string{}
	ctx.Args = nil
	out.Reset()
	errout.Reset()
	if code := runMktemp(ctx); code != 0 {
		t.Fatalf("mktemp %d: %s", code, errout.String())
	}
	name := strings.TrimSpace(out.String())
	defer os.Remove(name)
	if filepath.Dir(name) == os.Getenv("TMPDIR") {
		t.Errorf("mktemp used host TMPDIR: %s", name)
	}
	ctx.Args = nil
	out.Reset()
	errout.Reset()
	if code := runTopCommands(ctx); code == 0 || !strings.Contains(errout.String(), "cannot determine home") {
		t.Errorf("top-commands: %d %q", code, errout.String())
	}
}

func TestDiffInvocationTimezone(t *testing.T) {
	ctx := &Context{Env: []string{"TZ=UTC"}}
	got := diffOptionHeader(ctx, "missing", nil, diffOptions{})
	if !strings.Contains(got, "1970-01-01 00:00:00.000000000 +0000") {
		t.Fatalf("header: %q", got)
	}
}

func TestInvocationBackupAndPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SIMPLE_BACKUP_SUFFIX", ".host")
	t.Setenv("VERSION_CONTROL", "numbered")
	dir := t.TempDir()
	for _, name := range []string{"src", "dst"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	ctx := &Context{Dir: dir, Args: []string{"-b", "src", "dst"}, Env: []string{"SIMPLE_BACKUP_SUFFIX=.local", "VERSION_CONTROL=simple"}, Stdout: &out, Stderr: &errout}
	if code := runCp(ctx); code != 0 {
		t.Fatalf("cp: %d %s", code, errout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "dst.local")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errout.Reset()
	ctx.Args = []string{"src"}
	ctx.Env = []string{"PATH=" + dir}
	if code := runWhich(ctx); code != 0 || strings.TrimSpace(out.String()) != filepath.Join(dir, "src") {
		t.Errorf("which: %d %q %q", code, out.String(), errout.String())
	}
}
