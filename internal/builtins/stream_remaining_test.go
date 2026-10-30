package builtins

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStreamCommandsCanceled(t *testing.T) {
	cases := map[string][]string{
		"cmp": {"-", "other"}, "comm": {"-", "-"}, "diff": {"-", "other"},
		"du": {"--files0-from=-"}, "expand": nil, "unexpand": nil, "fold": nil,
		"fmt": nil, "nl": nil, "numfmt": {"--to=si"}, "paste": nil, "rev": nil,
		"sort": nil, "split": nil, "strings": nil, "tac": nil, "tr": {"a", "b"},
		"tsort": nil, "uudecode": nil, "uuencode": {"file"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			c, cancel := context.WithCancel(context.Background())
			cancel()
			code := Run(c, name, &Context{Dir: t.TempDir(), Args: args, Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
			if code != 130 {
				t.Errorf("canceled status = %d, want 130", code)
			}
		})
	}
}

func TestRemainingStreamsBlockedInput(t *testing.T) {
	cases := map[string][]string{
		"cmp": {"-", "other"}, "comm": {"-", "-"}, "diff": {"-", "other"},
		"du": {"--files0-from=-"}, "expand": nil, "unexpand": nil, "fold": nil,
		"fmt": nil, "nl": nil, "numfmt": {"--to=si"}, "paste": nil, "rev": nil,
		"sort": nil, "split": nil, "strings": nil, "tac": nil, "tr": {"a", "b"},
		"tsort": nil, "uudecode": nil, "uuencode": {"file"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			r, w := io.Pipe()
			defer r.Close()
			defer w.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			done := make(chan int, 1)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "other"), []byte("data\n"), 0600); err != nil {
				t.Fatal(err)
			}
			go func() {
				done <- Run(ctx, name, &Context{Dir: dir, Args: args, Stdin: r, Stdout: io.Discard, Stderr: io.Discard})
			}()
			select {
			case code := <-done:
				if code != 124 {
					t.Errorf("status %d, want 124", code)
				}
			case <-time.After(time.Second):
				t.Fatal("did not stop after deadline")
			}
		})
	}
}

type remainingInfiniteReader struct{}

func (remainingInfiniteReader) Read(p []byte) (int, error) { return copy(p, "a\n\n"), nil }

type remainingFailedWriter struct{}

func (remainingFailedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRemainingStreamsBrokenOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"comm", []string{"-", "-"}}, {"expand", nil}, {"fmt", nil}, {"nl", nil}, {"strings", []string{"-n", "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan int, 1)
			dir := t.TempDir()
			go func() {
				done <- Run(ctx, tc.name, &Context{Dir: dir, Args: tc.args, Stdin: remainingInfiniteReader{}, Stdout: remainingFailedWriter{}, Stderr: io.Discard})
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("did not stop on output error")
				cancel()
				<-done
			}
		})
	}
}
