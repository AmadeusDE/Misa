package commandexec

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestResolveEnvironment(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "tool")
	if err := os.WriteFile(p, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve("tool", Invocation{Dir: d, Env: []string{}}); err == nil || ErrorStatus(err) != 127 {
		t.Fatalf("unset PATH: %v", err)
	}
	got, err := Resolve("tool", Invocation{Dir: d, Env: []string{"PATH="}})
	if err != nil || got != p {
		t.Fatalf("empty PATH: %q %v", got, err)
	}
}
func TestRunStreamsAndStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	var out bytes.Buffer
	code := Run(context.Background(), "/bin/sh", Invocation{Args: []string{"-c", "cat; exit 7"}, Stdin: bytes.NewBufferString("hello"), Stdout: &out})
	if code != 7 || out.String() != "hello" {
		t.Fatalf("%d %q", code, out.String())
	}
}
func TestCancelDrain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	p, err := Start(ctx, "/bin/sh", Invocation{Args: []string{"-c", "sleep 30 & echo ready"}, Stdout: &out}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Wait(); got != 124 {
		t.Fatalf("status %d", got)
	}
	<-p.Done()
	if p.Wait() != 124 {
		t.Fatal("unstable wait")
	}
}
func TestStatus(t *testing.T) {
	if ContextStatus(context.Canceled) != 130 || ContextStatus(context.DeadlineExceeded) != 124 || ExitStatus(nil) != 0 || ExitStatus(errors.New("x")) != 125 {
		t.Fatal("status")
	}
}

func TestStartFailureAndCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	d := t.TempDir()
	p := filepath.Join(d, "bad")
	if err := os.WriteFile(p, []byte("not an executable format"), 0755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := Start(context.Background(), p, Invocation{Stdin: bytes.NewBufferString("input"), Stdout: &out, Stderr: &out}, Options{}); err == nil || ErrorStatus(err) != 126 {
		t.Fatalf("ENOEXEC: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Start(ctx, "/bin/sh", Invocation{}, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(p, Invocation{}); err == nil || ErrorStatus(err) != 126 {
		t.Fatalf("permission: %v", err)
	}
}

func TestBoundedOutputDrain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	var out bytes.Buffer
	start := time.Now()
	p, err := Start(context.Background(), "/bin/sh", Invocation{Args: []string{"-c", "sleep .5 & echo done"}, Stdout: &out}, Options{WaitDelay: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	code := p.Wait()
	if code != 0 || time.Since(start) > 400*time.Millisecond || out.String() != "done\n" {
		t.Fatalf("status=%d duration=%v output=%q", code, time.Since(start), out.String())
	}
}
func TestGracefulCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	w := &notifyWriter{ready: ready}
	p, err := Start(ctx, "/bin/sh", Invocation{Args: []string{"-c", `trap 'echo terminated; exit 0' TERM; echo ready; while :; do :; done`}, Stdout: w}, Options{CancelGrace: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("not ready")
	}
	cancel()
	if got := p.Wait(); got != 130 {
		t.Fatalf("status=%d", got)
	}
	if !bytes.Contains(w.buf.Bytes(), []byte("terminated")) {
		t.Fatalf("no trap output: %q", w.buf.String())
	}
}

type notifyWriter struct {
	buf   bytes.Buffer
	ready chan struct{}
	once  sync.Once
}

func (w *notifyWriter) Write(b []byte) (int, error) {
	n, e := w.buf.Write(b)
	w.once.Do(func() { close(w.ready) })
	return n, e
}
func TestBackslashExplicit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix")
	}
	d := t.TempDir()
	name := `a\b`
	p := filepath.Join(d, name)
	if err := os.WriteFile(p, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(name, Invocation{Dir: d, Env: []string{}})
	if err != nil || got != p {
		t.Fatalf("%q %v", got, err)
	}
}
