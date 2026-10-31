//go:build unix

package builtins

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestXargsProcessHelper(t *testing.T) {
	switch os.Getenv("MISA_XARGS_PROCESS_HELPER") {
	case "signal":
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		os.Exit(99)
	case "wait":
		fmt.Fprintln(os.Stdout, "ready")
		for {
			time.Sleep(time.Hour)
		}
	}
}

func TestXargsProcessSignal(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	inv := &Context{Args: []string{"-P2", "-n1", exe, "-test.run=^TestXargsProcessHelper$"}, Env: append(os.Environ(), "MISA_XARGS_PROCESS_HELPER=signal"), Stdin: strings.NewReader("a b"), Stderr: &stderr}
	if got := Run(context.Background(), "xargs", inv); got != 125 {
		t.Fatalf("status=%d want125 stderr=%q", got, stderr.String())
	}
}

type xargsReadyWriter struct {
	ready chan struct{}
}

func (w xargsReadyWriter) Write(p []byte) (int, error) {
	w.ready <- struct{}{}
	return len(p), nil
}

func TestXargsProcessCancellation(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{}, 2)
	done := make(chan int, 1)
	var stderr bytes.Buffer
	inv := &Context{Args: []string{"-P2", "-n1", exe, "-test.run=^TestXargsProcessHelper$"}, Env: append(os.Environ(), "MISA_XARGS_PROCESS_HELPER=wait"), Stdin: strings.NewReader("a b c d"), Stdout: xargsReadyWriter{ready}, Stderr: &stderr}
	go func() { done <- Run(ctx, "xargs", inv) }()
	for i := 0; i < 2; i++ {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			t.Fatal("children did not become ready")
		}
	}
	cancel()
	select {
	case got := <-done:
		if got != 130 {
			t.Fatalf("status=%d want130 stderr=%q", got, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("xargs did not reap cancelled children")
	}
}

// The integer-only injection point keeps its old mapping. Direct children use
// WaitResult instead, as covered by TestXargsIntegrationExternalHighExitStatuses.
func TestXargsParallelInjectedSignal(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	inv := &Context{Args: []string{"-P2", "-n1", "worker"}, Env: []string{}, Stdin: strings.NewReader("a b c"), Stderr: io.Discard}
	inv.Exec = func(context.Context, string, *Context) int {
		mu.Lock()
		calls++
		mu.Unlock()
		return 130
	}
	if got := Run(context.Background(), "xargs", inv); got != 125 {
		t.Fatalf("status=%d want125", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls > 2 {
		t.Fatalf("launched %d children after signal", calls)
	}
}
