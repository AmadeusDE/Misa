package api

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestListenModeAndStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "ughbot.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	if _, err := Listen(path); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("live daemon: got %v", err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	ln2, err := Listen(path)
	if err != nil {
		t.Fatalf("stale socket not replaced: %v", err)
	}
	ln2.Close()
}

func TestListenRefusesNonSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	os.WriteFile(path, []byte("x"), 0o600)
	if _, err := Listen(path); err == nil {
		t.Fatal("overwrote a regular file")
	}
}
