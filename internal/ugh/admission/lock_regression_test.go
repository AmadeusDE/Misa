package admission

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A separate Go test process owns the slot; diagnostics must remain readable
// even while its lock is held, and a crash must release the slot.
func TestProcessDiagnosticAndCrash(t *testing.T) {
	dir := t.TempDir()
	cmd := helperCmd(dir, "portable", "hold_slot")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.AfterFunc(20*time.Second, func() { _ = cmd.Process.Kill() })
	defer deadline.Stop()
	defer func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	s := bufio.NewScanner(out)
	if !s.Scan() || s.Text() != "ACQUIRED" {
		t.Fatalf("helper: %s %v", s.Text(), s.Err())
	}
	mgr := NewManager(WithBaseDir(dir))
	d, err := mgr.Diagnostics("portable")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.ActiveCombined) != 1 || !strings.HasPrefix(d.ActiveCombined[0].RequestID, "helper-") {
		t.Fatalf("diagnostics: %+v", d.ActiveCombined)
	}
	req := Request{AccountID: "portable", Provider: "codex", Model: "gpt-6-sol", Priority: PriorityChild}
	if _, err := mgr.Acquire(context.Background(), req); !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("contention: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	req.Priority = PriorityRoot
	if _, err := mgr.Acquire(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	req.Priority = PriorityChild
	h, err := mgr.Acquire(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	_ = h.Close()
}

func TestLockFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	f, err := openLockFile(link)
	if err == nil {
		_ = closeFd(f)
		t.Fatal("opened symlink lock")
	}
	if !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "untouched" {
		t.Fatalf("target changed: %q %v", data, err)
	}
}
