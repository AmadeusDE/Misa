//go:build (unix && !aix) || windows

package filelock

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHelper(t *testing.T) {
	if os.Getenv("FILELOCK_HELPER") != "1" {
		return
	}
	f, err := Open(os.Getenv("FILELOCK_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := TryLock(f); err != nil {
		t.Fatal(err)
	}
	fmt.Println("ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := Unlock(f); err != nil {
		t.Fatal(err)
	}
}

func holder(t *testing.T, path string) (*exec.Cmd, io.WriteCloser) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelper$")
	cmd.Env = append(os.Environ(), "FILELOCK_HELPER=1", "FILELOCK_PATH="+path)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })
	type response struct {
		line string
		err  error
	}
	ready := make(chan response, 1)
	go func() { line, err := bufio.NewReader(out).ReadString('\n'); ready <- response{line, err} }()
	select {
	case r := <-ready:
		if r.err != nil || r.line != "ready\n" {
			t.Fatalf("helper: %q %v", r.line, r.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper startup timed out")
	}
	return cmd, in
}

func TestInterprocess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	if err := os.WriteFile(path, []byte("diagnostics"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd, _ := holder(t, path)
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := TryLock(f); !errors.Is(err, ErrLocked) {
		t.Fatalf("contention: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	if err := Lock(ctx, f); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel: %v", err)
	}
	if err := TryLock(f); !errors.Is(err, ErrLocked) {
		t.Fatalf("holder lost lock after cancellation: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "diagnostics" {
		t.Fatalf("read while locked: %q %v", b, err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if err := Lock(ctx2, f); err != nil {
		t.Fatalf("after crash: %v", err)
	}
	if err := Unlock(f); err != nil {
		t.Fatal(err)
	}
	if err := Unlock(f); err != nil {
		t.Fatalf("repeat unlock: %v", err)
	}
}

func TestOpenSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	f, err := Open(link)
	if err == nil {
		f.Close()
		t.Fatal("opened symlink")
	}
}

func TestLockCanceled(t *testing.T) {
	f, err := Open(filepath.Join(t.TempDir(), "lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Lock(ctx, f); !errors.Is(err, context.Canceled) {
		t.Fatalf("already canceled: %v", err)
	}
}

func TestLockAfterNormalRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	_, in := holder(t, path)
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- Lock(ctx, f) }()
	select {
	case err := <-result:
		t.Fatalf("Lock returned before release: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if err := in.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("after normal release: %v", err)
	}
	if err := Unlock(f); err != nil {
		t.Fatal(err)
	}
}
