package worktree

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestRepoLockHelper holds the same repository lock in a separate process.
func TestRepoLockHelper(t *testing.T) {
	if os.Getenv("MISA_WORKTREE_LOCK_HELPER") != "1" {
		return
	}
	lock, err := LockRepo(context.Background(), os.Getenv("MISA_WORKTREE_LOCK_DIR"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Unlock()
	fmt.Println("ready")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestRepoLockProcessAndUnlock(t *testing.T) {
	dir := t.TempDir()
	processCtx, processCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer processCancel()
	cmd := exec.CommandContext(processCtx, os.Args[0], "-test.run=^TestRepoLockHelper$")
	cmd.Env = append(os.Environ(), "MISA_WORKTREE_LOCK_HELPER=1", "MISA_WORKTREE_LOCK_DIR="+dir)
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
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("helper: %q %v", line, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := LockRepo(ctx, dir); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contended lock: %v", err)
	}
	if err := in.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	lock, err := LockRepo(ctx2, dir)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := lock.Unlock(); err != nil {
				t.Errorf("unlock: %v", err)
			}
		}()
	}
	wg.Wait()
	lock2, err := LockRepo(ctx2, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock2.Unlock()
}

func TestRepoLockRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "ugh-worktree.lock")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	lock, err := LockRepo(context.Background(), dir)
	if err == nil {
		lock.Unlock()
		t.Fatal("opened symlink")
	}
	b, err := os.ReadFile(target)
	if err != nil || string(b) != "untouched" {
		t.Fatalf("target: %q %v", b, err)
	}
}

func TestRepoLockUnlockErrorIsNotRepeated(t *testing.T) {
	var nilLock *RepoLock
	if err := nilLock.Unlock(); err != nil {
		t.Fatal(err)
	}
	lock, err := LockRepo(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an invalidated descriptor: the unlock error must be returned once,
	// while the lock is still marked closed for subsequent calls.
	if err := lock.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Unlock(); err == nil {
		t.Fatal("expected unlock error")
	}
	if err := lock.Unlock(); err != nil {
		t.Fatalf("repeat unlock: %v", err)
	}
}
