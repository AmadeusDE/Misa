package builtins

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type f05Writer struct {
	cancel context.CancelFunc
	fail   bool
}

func (w f05Writer) Write(p []byte) (int, error) {
	if w.cancel != nil {
		w.cancel()
	}
	if w.fail {
		return 0, errors.New("broken output")
	}
	return len(p), nil
}

func TestF05StreamingCancellation(t *testing.T) {
	cases := []struct {
		name string
		run  func(*Context) int
		args []string
	}{
		{"yes", runYes, nil}, {"seq", runSeq, []string{"1000000"}}, {"seq-big", runSeq, []string{"100000000000000000"}},
		{"shuf", runShuf, []string{"-r", "-e", "x"}}, {"printf", runPrintf, []string{strings.Repeat("x", 10000)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, cancel := context.WithCancel(context.Background())
			defer cancel()
			inv := &Context{Ctx: c, Args: tc.args, Stdin: strings.NewReader(""), Stdout: f05Writer{cancel: cancel}, Stderr: io.Discard}
			done := make(chan int, 1)
			go func() { done <- tc.run(inv) }()
			select {
			case status := <-done:
				if status != 130 {
					t.Errorf("status = %d, want 130", status)
				}
			case <-time.After(time.Second):
				t.Fatal("did not stop on cancellation")
			}
		})
	}
}

func TestF05StreamingWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Context) int
		args []string
	}{
		{"yes", runYes, nil}, {"seq-big", runSeq, []string{"100000000000000000"}}, {"printf", runPrintf, []string{strings.Repeat("x", 10000)}}, {"shuf", runShuf, []string{"-r", "-e", "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan int, 1)
			c, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				done <- tc.run(&Context{Ctx: c, Args: tc.args, Stdin: strings.NewReader(""), Stdout: f05Writer{fail: true}, Stderr: io.Discard})
			}()
			select {
			case status := <-done:
				if status != 1 {
					t.Errorf("status = %d, want 1", status)
				}
			case <-time.After(time.Second):
				t.Fatal("did not stop on write error")
			}
		})
	}
}

func TestF05TailFollowDeadline(t *testing.T) {
	file := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(file, []byte("hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-f", "-F"} {
		t.Run(flag, func(t *testing.T) {
			c, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			done := make(chan int, 1)
			go func() {
				done <- runTail(&Context{Ctx: c, Args: []string{flag, file}, Stdout: io.Discard, Stderr: io.Discard})
			}()
			select {
			case status := <-done:
				if status != 124 {
					t.Errorf("status = %d, want 124", status)
				}
			case <-time.After(time.Second):
				t.Fatal("tail did not stop")
			}
		})
	}
}

func TestF05TailAndShufBlockedInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Context) int
	}{{"tail", runTail}, {"shuf", runShuf}} {
		t.Run(tc.name, func(t *testing.T) {
			r, w := io.Pipe()
			defer r.Close()
			defer w.Close()
			c, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			done := make(chan int, 1)
			go func() { done <- tc.run(&Context{Ctx: c, Stdin: r, Stdout: io.Discard, Stderr: io.Discard}) }()
			select {
			case status := <-done:
				if status != 124 {
					t.Errorf("status = %d, want 124", status)
				}
			case <-time.After(time.Second):
				t.Fatal("blocked on input")
			}
		})
	}
}

func TestF05TailWriteFailure(t *testing.T) {
	inv := &Context{Stdin: strings.NewReader("one\ntwo\n"), Stdout: f05Writer{fail: true}, Stderr: io.Discard}
	if got := runTail(inv); got != 1 {
		t.Errorf("status = %d, want 1", got)
	}
}

func TestF05YesPreservesBufferLimit(t *testing.T) {
	var out bytes.Buffer
	if got := runYes(&Context{Stdout: &out, Stderr: io.Discard}); got != 0 {
		t.Fatalf("status = %d", got)
	}
	if got := strings.Count(out.String(), "y\n"); got != 10000 {
		t.Errorf("lines = %d, want 10000", got)
	}
}
