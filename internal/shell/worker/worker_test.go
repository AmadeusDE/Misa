package worker

import (
	"bytes"
	"context"
	"errors"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/shell/session"
)

func TestPersistenceAndSyntax(t *testing.T) {
	c, e := Start(context.Background(), Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Shutdown(context.Background())
	for _, s := range []string{"x=ok; f() { test \"$x\" = ok; }", "f"} {
		r, e := c.Run(context.Background(), s)
		if e != nil || r.Status != 0 {
			t.Fatalf("%+v %v", r, e)
		}
	}
	r, e := c.Run(context.Background(), "if")
	if e != nil || r.ParseError == nil {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestCancelAndRecovery(t *testing.T) {
	c, e := Start(context.Background(), Options{Session: session.Options{Exec: func(ctx context.Context, i session.Invocation) int { <-ctx.Done(); return 1 }}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Shutdown(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	r, e := c.Run(ctx, "waiter")
	if e != nil && !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	if e == nil && r.Err == nil {
		t.Fatal(r)
	}
	r, e = c.Run(context.Background(), "true")
	if e != nil || r.Status != 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestMismatch(t *testing.T) {
	_, e := Start(context.Background(), Options{Version: Version + 1})
	if e == nil {
		t.Fatal("expected mismatch")
	}
}
func TestShutdownTwice(t *testing.T) {
	c, e := Start(context.Background(), Options{})
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Shutdown(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = c.Shutdown(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestCrash(t *testing.T) {
	c, e := Start(context.Background(), Options{Session: session.Options{Exec: func(context.Context, session.Invocation) int { panic("secret source") }}})
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.Run(context.Background(), "boom")
	if !errors.Is(e, ErrCrash) {
		t.Fatalf("%v", e)
	}
	_, e = c.Run(context.Background(), "true")
	if !errors.Is(e, ErrCrash) {
		t.Fatal(e)
	}
}
func TestFrameBounds(t *testing.T) {
	for _, b := range [][]byte{{0, 0, 0, 0}, {0, 16, 0, 1}, {0, 0, 0, 1, '{'}} {
		if _, e := readFrame(bytes.NewReader(b)); !errors.Is(e, ErrProtocol) {
			t.Fatalf("%v", e)
		}
	}
}
func TestConcurrentShutdown(t *testing.T) {
	c, e := Start(context.Background(), Options{})
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if e := c.Shutdown(context.Background()); e != nil {
				t.Error(e)
			}
		})
	}
	wg.Wait()
}
func BenchmarkStartup(b *testing.B) {
	for b.Loop() {
		c, e := Start(context.Background(), Options{})
		if e != nil {
			b.Fatal(e)
		}
		if e = c.Shutdown(context.Background()); e != nil {
			b.Fatal(e)
		}
	}
}
func BenchmarkRun(b *testing.B) {
	c, e := Start(context.Background(), Options{})
	if e != nil {
		b.Fatal(e)
	}
	defer c.Shutdown(context.Background())
	b.ResetTimer()
	for b.Loop() {
		if _, e = c.Run(context.Background(), "true"); e != nil {
			b.Fatal(e)
		}
	}
}

func TestShutdownWaitsForRun(t *testing.T) {
	started := make(chan struct{})
	released := make(chan struct{})
	c, e := Start(context.Background(), Options{Session: session.Options{Exec: func(ctx context.Context, _ session.Invocation) int {
		close(started)
		<-ctx.Done()
		<-released
		return 1
	}}})
	if e != nil {
		t.Fatal(e)
	}
	go c.Run(context.Background(), "block")
	<-started
	finished := make(chan error, 1)
	go func() { finished <- c.Shutdown(context.Background()) }()
	select {
	case <-finished:
		t.Fatal("shutdown returned before handler finished")
	case <-time.After(20 * time.Millisecond):
	}
	close(released)
	select {
	case e := <-finished:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown stuck")
	}
}

func TestBlockedTransportRunContext(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	go func() { _, _ = readFrame(b); _ = writeFrame(b, frame{Type: "hello", Version: Version}) }()
	c, e := Connect(context.Background(), a, Version)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, e = c.Run(ctx, "true")
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}
func TestIsolation(t *testing.T) {
	a, e := Start(context.Background(), Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer a.Shutdown(context.Background())
	b, e := Start(context.Background(), Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer b.Shutdown(context.Background())
	if _, e = a.Run(context.Background(), "x=secret"); e != nil {
		t.Fatal(e)
	}
	r, e := b.Run(context.Background(), "test -z \"$x\"")
	if e != nil || r.Status != 0 {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestBlockedTransportCancelContext(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	go func() { _, _ = readFrame(b); _ = writeFrame(b, frame{Type: "hello", Version: Version}) }()
	c, e := Connect(context.Background(), a, Version)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e = c.Cancel(ctx, 1); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}

func TestCooperativeWorkersTerminate(t *testing.T) {
	before := runtime.NumGoroutine()
	for range 25 {
		c, e := Start(context.Background(), Options{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Run(context.Background(), "true"); e != nil {
			t.Fatal(e)
		}
		if e = c.Shutdown(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > before+8 && time.Now().Before(deadline) {
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before+8 {
		t.Fatalf("goroutines before %d after %d", before, got)
	}
}

func TestInitializationError(t *testing.T) {
	_, e := Start(context.Background(), Options{Session: session.Options{Dir: filepath.Join(t.TempDir(), "missing")}})
	if !errors.Is(e, ErrRuntime) || strings.Contains(e.Error(), "version mismatch") {
		t.Fatalf("expected session initialization error, got %v", e)
	}
}

func TestStartupContextDoesNotOwnLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c, e := Start(ctx, Options{})
	if e != nil {
		cancel()
		t.Fatal(e)
	}
	cancel()
	r, e := c.Run(context.Background(), "true")
	if e != nil || r.Status != 0 {
		t.Fatalf("run after startup cancellation: %+v, %v", r, e)
	}
	if e = c.Shutdown(context.Background()); e != nil {
		t.Fatal(e)
	}
}
