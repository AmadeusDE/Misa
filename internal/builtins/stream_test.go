package builtins

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCopyContextCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "deadline"}[deadline], func(t *testing.T) {
			baseline := runtime.NumGoroutine()
			source, sink := io.Pipe()
			ctx, cancel := context.WithCancel(context.Background())
			if deadline {
				ctx, cancel = context.WithTimeout(context.Background(), 40*time.Millisecond)
			}
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := CopyContext(ctx, io.Discard, source); done <- err }()
			if !deadline {
				time.Sleep(20 * time.Millisecond)
				cancel()
			}
			select {
			case err := <-done:
				want := context.Canceled
				if deadline {
					want = context.DeadlineExceeded
				}
				if !errors.Is(err, want) {
					t.Fatalf("error %v, want %v", err, want)
				}
			case <-time.After(time.Second):
				t.Fatal("copy did not cancel")
			}
			source.Close()
			sink.Close()
			until := time.Now().Add(time.Second)
			for runtime.NumGoroutine() > baseline+2 && time.Now().Before(until) {
				time.Sleep(time.Millisecond)
			}
			if n := runtime.NumGoroutine(); n > baseline+2 {
				t.Fatalf("goroutines: before %d, after %d", baseline, n)
			}
		})
	}
}

func TestCopyContextStreams(t *testing.T) {
	var out bytes.Buffer
	n, err := CopyContext(context.Background(), &out, strings.NewReader("hello"))
	if n != 5 || err != nil || out.String() != "hello" {
		t.Fatalf("copy = %d, %v, %q", n, err, out.String())
	}
	_, err = CopyContext(context.Background(), closedStreamWriter{}, strings.NewReader("hello"))
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error = %v", err)
	}
}

type closedStreamWriter struct{}

func (closedStreamWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestContextWriterBlockedCancellation(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := CopyContext(ctx, w, strings.NewReader("hello")); done <- err }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked write did not cancel")
	}
}

func TestContextStreamsDoNotRetainIdleWorkers(t *testing.T) {
	baseline := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < 32; i++ {
		input := ContextReader(ctx, strings.NewReader("x"))
		if _, err := input.Read(make([]byte, 1)); err != nil {
			t.Fatal(err)
		}
		if _, err := ContextWriter(ctx, io.Discard).Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	until := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > baseline+3 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > baseline+3 {
		t.Fatalf("idle workers: baseline %d now %d", baseline, n)
	}
}
