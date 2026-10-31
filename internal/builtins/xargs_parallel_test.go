package builtins

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func xargsWait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for xargs")
		var zero T
		return zero
	}
}

func TestXargsParallelBoundedAndUnlimited(t *testing.T) {
	for _, tc := range []struct {
		name, flag string
		width      int
	}{{"bounded", "-P2", 2}, {"unlimited", "-P0", 4}} {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan struct{}, 4)
			release := make(chan struct{})
			defer close(release)
			done := make(chan int, 1)
			var mu sync.Mutex
			active, max, calls := 0, 0, 0
			inv := &Context{Args: []string{tc.flag, "-n1", "worker"}, Stdin: strings.NewReader("a b c d"), Env: []string{}, Stderr: io.Discard, Stdout: io.Discard}
			inv.Exec = func(context.Context, string, *Context) int {
				mu.Lock()
				active++
				calls++
				if active > max {
					max = active
				}
				mu.Unlock()
				started <- struct{}{}
				<-release
				mu.Lock()
				active--
				mu.Unlock()
				return 0
			}
			go func() { done <- Run(context.Background(), "xargs", inv) }()
			for i := 0; i < tc.width; i++ {
				xargsWait(t, started)
			}
			if tc.width == 2 {
				select {
				case <-started:
					t.Fatal("exceeded -P2")
				case <-time.After(100 * time.Millisecond):
				}
			}
			// Release all children and await completion before examining counters.
			for i := 0; i < 4; i++ {
				select {
				case release <- struct{}{}:
				case <-time.After(2 * time.Second):
					t.Fatal("child did not finish")
				}
			}
			if code := xargsWait(t, done); code != 0 {
				t.Fatalf("status=%d", code)
			}
			mu.Lock()
			defer mu.Unlock()
			if calls != 4 || max != tc.width {
				t.Fatalf("calls=%d max concurrency=%d", calls, max)
			}
		})
	}
}

func TestXargsParallelStop255Waits(t *testing.T) {
	started := make(chan string, 3)
	fail := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-P2", "-n1", "worker"}, Stdin: strings.NewReader("a b c"), Env: []string{}, Stderr: io.Discard, Stdout: io.Discard}
	inv.Exec = func(ctx context.Context, _ string, c *Context) int {
		started <- c.Args[0]
		if c.Args[0] == "a" {
			<-fail
			return 255
		}
		select {
		case <-release:
			return 0
		case <-ctx.Done():
			return 130
		}
	}
	go func() { done <- Run(context.Background(), "xargs", inv) }()
	xargsWait(t, started)
	xargsWait(t, started)
	close(fail)
	select {
	case code := <-done:
		t.Fatalf("returned %d before running child finished", code)
	case arg := <-started:
		t.Fatalf("launched %q after 255", arg)
	case <-time.After(150 * time.Millisecond):
	}
	release <- struct{}{}
	if code := xargsWait(t, done); code != 124 {
		t.Fatalf("status=%d want 124", code)
	}
	select {
	case arg := <-started:
		t.Fatalf("unexpected launch %q", arg)
	default:
	}
}

func TestXargsParallelOrdinaryFailure(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	inv := &Context{Args: []string{"-P2", "-n1", "worker"}, Stdin: strings.NewReader("a b c d"), Env: []string{}, Stderr: io.Discard, Stdout: io.Discard}
	inv.Exec = func(_ context.Context, _ string, c *Context) int {
		mu.Lock()
		calls++
		mu.Unlock()
		if c.Args[0] == "b" {
			return 7
		}
		return 0
	}
	code := Run(context.Background(), "xargs", inv)
	mu.Lock()
	defer mu.Unlock()
	if code != 123 || calls != 4 {
		t.Fatalf("status=%d calls=%d", code, calls)
	}
}

func TestXargsParallelCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 4)
	done := make(chan int, 1)
	var mu sync.Mutex
	calls := 0
	inv := &Context{Args: []string{"-P2", "-n1", "worker"}, Stdin: strings.NewReader("a b c d"), Env: []string{}, Stderr: io.Discard, Stdout: io.Discard}
	inv.Exec = func(ctx context.Context, _ string, _ *Context) int {
		mu.Lock()
		calls++
		mu.Unlock()
		started <- struct{}{}
		<-ctx.Done()
		return 130
	}
	go func() { done <- Run(ctx, "xargs", inv) }()
	xargsWait(t, started)
	xargsWait(t, started)
	cancel()
	if code := xargsWait(t, done); code != 130 {
		t.Fatalf("status=%d", code)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 {
		t.Fatalf("launched %d children", calls)
	}
}

func TestXargsParallelFailureWhileInputBlocked(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-P2", "-n1", "worker"}, Stdin: r, Env: []string{}, Stderr: io.Discard, Stdout: io.Discard, Exec: func(context.Context, string, *Context) int { return 255 }}
	go func() { done <- Run(context.Background(), "xargs", inv) }()
	go func() { _, _ = io.WriteString(w, "first\n") }()
	if code := xargsWait(t, done); code != 124 {
		t.Fatalf("status=%d", code)
	}
}
