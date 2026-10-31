package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestShellEntryHelper(t *testing.T) {
	if os.Getenv("MISA_SH_ENTRY_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = os.Args[i+1:]
			main()
			os.Exit(0)
		}
	}
	os.Exit(99)
}

func TestShellEntryPoints(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "script")
	if err := os.WriteFile(script, []byte("printf '%s|%s' \"$0\" \"$1\"; exit 9\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, input, out string
		args             []string
		status           int
	}{
		{"subcommand", "", "named|a b", []string{"misa", "sh", "-c", "printf '%s|%s' \"$0\" \"$1\"", "named", "a b"}, 0},
		{"sh alias", "", "sh", []string{"sh", "-c", "printf '%s' \"$0\""}, 0},
		{"misa-sh alias", "", "misa-sh", []string{"misa-sh", "-c", "printf '%s' \"$0\""}, 0},
		{"stdin", "echo stdin\n", "stdin\n", []string{"misa", "sh"}, 0},
		{"script", "", script + "|argument", []string{"misa", "sh", script, "argument"}, 9},
		{"missing", "", "", []string{"misa", "sh", filepath.Join(dir, "absent")}, 127},
		{"syntax", "", "", []string{"misa", "sh", "-c", "if"}, 2},
		{"exit", "", "", []string{"misa", "sh", "-c", "exit 23; echo wrong"}, 23},
		{"builtin with no host shell", "", "registry\n", []string{"misa", "sh", "-c", "printenv ENTRY_VALUE"}, 0},
		{"no host fallback", "", "", []string{"misa", "sh", "-c", "misa-no-such-command-23894"}, 127},
		{"usage", "", "", []string{"misa", "sh", "-c"}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestShellEntryHelper$", "--"}, tt.args...)
			cmd := exec.Command(exe, args...)
			cmd.Dir = dir
			cmd.Env = []string{"PATH=", "HOME=" + dir, "MISA_SH_ENTRY_HELPER=1", "ENTRY_VALUE=registry"}
			cmd.Stdin = strings.NewReader(tt.input)
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			err := cmd.Run()
			status := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				status = exit.ExitCode()
			}
			if status != tt.status || out.String() != tt.out {
				t.Fatalf("status %d, stdout %q, stderr %q; want %d, %q", status, out.String(), stderr.String(), tt.status, tt.out)
			}
		})
	}
}

func TestInstallShellLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require privileges on Windows")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "misa")
	if err := os.WriteFile(exe, nil, 0700); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := installShellLinks(exe, nil, &stderr); code != 1 {
		t.Fatalf("usage status = %d", code)
	}
	links := filepath.Join(dir, "shells")
	if code := installShellLinks(exe, []string{links}, &stderr); code != 0 {
		t.Fatalf("install status = %d: %s", code, &stderr)
	}
	for _, name := range []string{"sh", "misa-sh"} {
		got, err := os.Readlink(filepath.Join(links, name))
		if err != nil || got != exe {
			t.Fatalf("%s target %q, %v", name, got, err)
		}
	}
	entries, err := os.ReadDir(links)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected installed links: %v, %v", entries, err)
	}
	// Reinstall is idempotent, but must never remove a real file.
	if code := installShellLinks(exe, []string{links}, &stderr); code != 0 {
		t.Fatalf("reinstall status = %d", code)
	}
	if err := os.Remove(filepath.Join(links, "sh")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(links, "sh"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := installShellLinks(exe, []string{links}, &stderr); code != 1 {
		t.Fatalf("conflict status = %d", code)
	}
	b, err := os.ReadFile(filepath.Join(links, "sh"))
	if err != nil || string(b) != "preserve" {
		t.Fatalf("replaced real file: %q, %v", b, err)
	}
	fresh := t.TempDir()
	if err := os.WriteFile(filepath.Join(fresh, "sh"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := installShellLinks(exe, []string{fresh}, &stderr); code != 1 {
		t.Fatalf("fresh conflict status = %d", code)
	}
	if _, err := os.Lstat(filepath.Join(fresh, "misa-sh")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial install: %v", err)
	}
	if err := os.Remove(filepath.Join(links, "sh")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nonexistent/misa", filepath.Join(links, "sh")); err != nil {
		t.Fatal(err)
	}
	if code := installShellLinks(exe, []string{links}, &stderr); code != 0 {
		t.Fatalf("stale link status = %d: %s", code, &stderr)
	}
	if got, err := os.Readlink(filepath.Join(links, "sh")); err != nil || got != exe {
		t.Fatalf("stale link not repaired: %q, %v", got, err)
	}
}

func TestNormalInstallDoesNotSelectShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require privileges on Windows")
	}
	dir := t.TempDir()
	var stderr bytes.Buffer
	if code := builtins.InstallLinks(filepath.Join(dir, "misa"), []string{dir}, &stderr); code != 0 {
		t.Fatalf("normal install = %d: %s", code, &stderr)
	}
	for _, name := range []string{"sh", "misa-sh"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("normal install selected %s: %v", name, err)
		}
	}
}

func TestInstallShellLinksResolvesBareArgv0(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require privileges on Windows")
	}
	dir := t.TempDir()
	var stderr bytes.Buffer
	if code := installShellLinks("misa", []string{dir}, &stderr); code != 0 {
		t.Fatalf("install status = %d: %s", code, &stderr)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	got, err := os.Readlink(filepath.Join(dir, "sh"))
	if err != nil || got != exe {
		t.Fatalf("sh target %q, want %q: %v", got, exe, err)
	}
}
