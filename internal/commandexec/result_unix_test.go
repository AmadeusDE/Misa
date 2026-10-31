//go:build unix

package commandexec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestResultHelper(t *testing.T) {
	mode := os.Getenv("COMMAND_EXEC_RESULT_HELPER")
	if mode == "" {
		return
	}
	if mode == "signal" {
		_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
		os.Exit(99)
	}
	if mode == "wait" {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM)
		fmt.Fprint(os.Stdout, "ready")
		<-ch
		os.Exit(0)
	}
	code, err := strconv.Atoi(mode)
	if err != nil {
		os.Exit(99)
	}
	os.Exit(code)
}

func startResultHelper(t *testing.T, ctx context.Context, mode string, out *notifyWriter) *Process {
	t.Helper()
	inv := Invocation{Args: []string{"-test.run=^TestResultHelper$"}, Env: append(os.Environ(), "COMMAND_EXEC_RESULT_HELPER="+mode)}
	if out != nil {
		inv.Stdout = out
	}
	p, err := Start(ctx, os.Args[0], inv, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWaitResultExits(t *testing.T) {
	for _, code := range []int{0, 126, 127, 130, 200} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			p := startResultHelper(t, context.Background(), strconv.Itoa(code), nil)
			want := Result{Status: code}
			if got := p.WaitResult(); got != want {
				t.Fatalf("WaitResult() = %+v, want %+v", got, want)
			}
			if got := p.Wait(); got != code {
				t.Fatalf("Wait() = %d, want %d", got, code)
			}
			if got := p.WaitResult(); got != want {
				t.Fatalf("repeated WaitResult() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestWaitResultSignal(t *testing.T) {
	p := startResultHelper(t, context.Background(), "signal", nil)
	want := Result{Status: 128 + int(syscall.SIGKILL), Signaled: true}
	if got := p.WaitResult(); got != want {
		t.Fatalf("WaitResult() = %+v, want %+v", got, want)
	}
	if got := p.Wait(); got != want.Status {
		t.Fatalf("Wait() = %d, want %d", got, want.Status)
	}
}

func TestWaitResultCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &notifyWriter{ready: make(chan struct{})}
	p := startResultHelper(t, ctx, "wait", out)
	select {
	case <-out.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("helper not ready")
	}
	cancel()
	if got := p.WaitResult(); got != (Result{Status: 130, Signaled: true}) {
		t.Fatalf("cancelled WaitResult() = %+v, want status 130 and signaled", got)
	}
	if got := p.Wait(); got != 130 {
		t.Fatalf("cancelled Wait() = %d, want 130", got)
	}
}

func TestResultLaunchErrors(t *testing.T) {
	dir := t.TempDir()
	if p, err := Start(context.Background(), filepath.Join(dir, "missing"), Invocation{}, Options{}); p != nil || err == nil || ErrorStatus(err) != 127 {
		t.Fatalf("missing: process %v, error %v", p, err)
	}
	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("invalid executable"), 0755); err != nil {
		t.Fatal(err)
	}
	if p, err := Start(context.Background(), bad, Invocation{}, Options{}); p != nil || err == nil || ErrorStatus(err) != 126 {
		t.Fatalf("bad executable: process %v, error %v", p, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p, err := Start(ctx, os.Args[0], Invocation{}, Options{}); p != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled launch: process %v, error %v", p, err)
	}
}
