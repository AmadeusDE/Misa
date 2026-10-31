package builtins

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestXargsParallelUnlimitedCap(t *testing.T) {
	const total, capProcs = 260, 256
	started := make(chan struct{}, total)
	release := make(chan struct{})
	defer close(release)
	done := make(chan int, 1)
	var mu sync.Mutex
	active, max, calls := 0, 0, 0
	inv := &Context{Args: []string{"-P0", "-n1", "worker"}, Stdin: strings.NewReader(strings.Repeat("x\n", total)), Env: []string{}, Stderr: io.Discard, Stdout: io.Discard}
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
	for i := 0; i < capProcs; i++ {
		xargsWait(t, started)
	}
	select {
	case <-started:
		t.Fatal("-P0 exceeded 256 concurrent children")
	case <-time.After(150 * time.Millisecond):
	}
	for i := 0; i < total; i++ {
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
	if calls != total || max != capProcs {
		t.Fatalf("calls=%d max=%d", calls, max)
	}
}

func TestXargsParallelSerializesOutput(t *testing.T) {
	const count = 8
	var output bytes.Buffer
	started := make(chan struct{}, count)
	release := make(chan struct{})
	defer close(release)
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-P8", "-n1", "worker"}, Stdin: strings.NewReader(strings.Repeat("x\n", count)), Env: []string{}, Stdout: &output, Stderr: &output}
	inv.Exec = func(_ context.Context, _ string, c *Context) int {
		started <- struct{}{}
		<-release
		for i := 0; i < 100; i++ {
			_, _ = io.WriteString(c.Stdout, "O")
			_, _ = io.WriteString(c.Stderr, "E")
		}
		return 0
	}
	go func() { done <- Run(context.Background(), "xargs", inv) }()
	for i := 0; i < count; i++ {
		xargsWait(t, started)
	}
	for i := 0; i < count; i++ {
		release <- struct{}{}
	}
	if code := xargsWait(t, done); code != 0 {
		t.Fatalf("status=%d", code)
	}
	got := output.String()
	if len(got) != count*200 || strings.Count(got, "O") != count*100 || strings.Count(got, "E") != count*100 {
		t.Fatalf("corrupted output: len=%d O=%d E=%d", len(got), strings.Count(got, "O"), strings.Count(got, "E"))
	}
}

func TestXargsParallelFatalPrecedence(t *testing.T) {
	// These callback statuses use the integer executor's established convention:
	// 130 denotes a signal; 126 and 127 denote launch failures.
	cases := []struct {
		name                string
		first, second, want int
	}{
		{"255 over signal", 255, 130, 124}, {"signal over 255", 130, 255, 124},
		{"signal over 126", 130, 126, 125}, {"126 under signal", 126, 130, 125},
		{"126 over 127", 126, 127, 126}, {"127 under 126", 127, 126, 126},
		{"127 over ordinary", 127, 7, 127}, {"ordinary under 127", 7, 127, 127},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan string, 2)
			first := make(chan struct{})
			second := make(chan struct{})
			done := make(chan int, 1)
			defer close(first)
			defer close(second)
			inv := &Context{Args: []string{"-P2", "-n1", "worker"}, Stdin: strings.NewReader("a b"), Env: []string{}, Stdout: io.Discard, Stderr: io.Discard}
			inv.Exec = func(_ context.Context, _ string, c *Context) int {
				started <- c.Args[0]
				if c.Args[0] == "a" {
					<-first
					return tc.first
				}
				<-second
				return tc.second
			}
			go func() { done <- Run(context.Background(), "xargs", inv) }()
			xargsWait(t, started)
			xargsWait(t, started)
			first <- struct{}{}
			// The second child remains running, so even a fatal first result must wait.
			select {
			case code := <-done:
				t.Fatalf("returned %d before second child", code)
			case <-time.After(50 * time.Millisecond):
			}
			second <- struct{}{}
			if code := xargsWait(t, done); code != tc.want {
				t.Fatalf("status=%d want=%d", code, tc.want)
			}
		})
	}
}

func TestXargsParallelCancellationOverridesFatal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan string, 2)
	fail := make(chan struct{})
	defer close(fail)
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-P2", "-n1", "worker"}, Stdin: strings.NewReader("a b"), Env: []string{}, Stdout: io.Discard, Stderr: io.Discard}
	inv.Exec = func(ctx context.Context, _ string, c *Context) int {
		started <- c.Args[0]
		if c.Args[0] == "a" {
			<-fail
			return 255
		}
		<-ctx.Done()
		return 130
	}
	go func() { done <- Run(ctx, "xargs", inv) }()
	xargsWait(t, started)
	xargsWait(t, started)
	fail <- struct{}{}
	select {
	case code := <-done:
		t.Fatalf("returned %d before cancellation", code)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if code := xargsWait(t, done); code != 130 {
		t.Fatalf("status=%d want cancellation status 130", code)
	}
}
