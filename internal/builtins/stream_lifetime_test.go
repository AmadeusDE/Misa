package builtins

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

type lifetimeStream struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *lifetimeStream) Read([]byte) (int, error) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	return 0, io.EOF
}
func (s *lifetimeStream) Write([]byte) (int, error) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	return 0, io.ErrClosedPipe
}

func TestStreamCommandWorkerLifetimes(t *testing.T) {
	cases := map[string][]string{
		"cat": nil, "tee": nil, "tail": nil, "head": nil, "wc": nil, "cut": {"-b", "1"},
		"md5sum": nil, "sha1sum": nil, "sha224sum": nil, "sha256sum": nil, "sha384sum": nil, "sha512sum": nil, "sha3sum": nil, "b2sum": nil, "cksum": nil, "sum": nil,
		"base32": nil, "base64": nil, "basenc": {"--base64"}, "xxd": nil,
		"cmp": {"-", "other"}, "comm": {"-", "-"}, "diff": {"-", "other"}, "du": {"--files0-from=-"},
		"expand": nil, "unexpand": nil, "fold": nil, "fmt": nil, "nl": nil, "numfmt": {"--to=si"}, "paste": nil, "rev": nil, "sort": nil, "split": nil, "strings": nil, "tac": nil, "tr": {"a", "b"}, "tsort": nil, "uudecode": nil, "uuencode": {"file"},
		"yes": nil, "seq": {"100000000000000000"}, "shuf": {"-r", "-e", "x"}, "printf": {"%s", "x"},
		"cp": {"-i", "other", "dest"}, "mv": {"-i", "other", "dest"}, "mcookie": {"-f", "-"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			baseline := runtime.NumGoroutine()
			source := &lifetimeStream{started: make(chan struct{}), release: make(chan struct{})}
			defer close(source.release)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dir := t.TempDir()
			for _, file := range []string{"other", "dest"} {
				if err := os.WriteFile(filepath.Join(dir, file), []byte("data\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var output io.Writer = io.Discard
			switch name {
			case "yes", "seq", "shuf", "printf":
				output = source
			}
			done := make(chan int, 1)
			go func() {
				done <- Run(ctx, name, &Context{Dir: dir, Args: args, Stdin: source, Stdout: output, Stderr: io.Discard})
			}()
			select {
			case <-source.started:
			case code := <-done:
				t.Fatalf("exited before blocking: %d", code)
			case <-time.After(time.Second):
				t.Fatal("stream never reached")
			}
			cancel()
			select {
			case code := <-done:
				if code != 130 {
					t.Fatalf("status = %d", code)
				}
			case <-time.After(time.Second):
				t.Fatal("command did not cancel")
			}
			// Caller owns the stream. Unblocking it must release the one pending worker.
			source.release <- struct{}{}
			until := time.Now().Add(time.Second)
			for runtime.NumGoroutine() > baseline+1 && time.Now().Before(until) {
				time.Sleep(time.Millisecond)
			}
			if n := runtime.NumGoroutine(); n > baseline+1 {
				t.Fatalf("goroutines before %d after %d", baseline, n)
			}
		})
	}
}

func TestEmptyAndClosedStandardInput(t *testing.T) {
	for _, name := range []string{"cat", "tail"} {
		for _, closed := range []bool{false, true} {
			r, w := io.Pipe()
			if closed {
				r.Close()
			} else {
				w.Close()
			}
			code := Run(context.Background(), name, &Context{Stdin: r, Stdout: io.Discard, Stderr: io.Discard})
			r.Close()
			w.Close()
			if !closed && code != 0 {
				t.Fatalf("%s empty stdin status %d", name, code)
			}
		}
	}
}
