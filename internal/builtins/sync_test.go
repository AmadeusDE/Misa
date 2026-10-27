//go:build linux

// Behaviour adapted from toybox sync.c (0BSD) and uutils coreutils sync (MIT).
package builtins

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSyncCommandErrors(t *testing.T) {
	for _, args := range [][]string{{"-d"}, {"-d", "-f"}, {"--invalid"}} {
		assertExitCode(t, runBuiltin(t, "sync", args...), 1)
	}
	for _, option := range []string{"--data", "--file-system", "--"} {
		r := runBuiltin(t, "sync", option, "bad1", "bad2")
		assertExitCode(t, r, 1)
		if r.stderr != "sync: error opening 'bad1': No such file or directory\nsync: error opening 'bad2': No such file or directory\n" {
			t.Errorf("stderr = %q", r.stderr)
		}
	}
}

func TestSyncFileAndFIFO(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file", "content")
	for _, flag := range []string{"--data", "--file-system"} {
		assertExitCode(t, runBuiltinIn(t, dir, "sync", flag, "file", "."), 0)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	r := runBuiltinIn(t, dir, "sync", "--data", "pipe")
	assertExitCode(t, r, 1)
	if !strings.Contains(r.stderr, "error syncing 'pipe': Invalid") {
		t.Errorf("stderr = %q", r.stderr)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("sync on FIFO blocked")
	}
}
