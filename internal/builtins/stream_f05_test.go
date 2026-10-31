package builtins

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestF05BlockedInputCancellation(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"head", nil}, {"wc", nil}, {"cut", []string{"-b", "1"}},
		{"md5sum", nil}, {"sha224sum", nil}, {"sha384sum", nil}, {"sha3sum", nil},
		{"b2sum", nil}, {"cksum", nil}, {"sum", nil}, {"base32", nil}, {"base64", nil},
		{"basenc", []string{"--base64"}}, {"xxd", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, ok := Lookup(tc.name)
			if !ok {
				t.Fatal("not registered")
			}
			r, w := io.Pipe()
			defer r.Close()
			defer w.Close()
			c, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan int, 1)
			go func() {
				done <- cmd.Run(&Context{Ctx: c, Args: tc.args, Stdin: r, Stdout: io.Discard, Stderr: io.Discard})
			}()
			time.Sleep(10 * time.Millisecond)
			cancel()
			select {
			case code := <-done:
				if code != 130 {
					t.Errorf("got %d, want 130", code)
				}
			case <-time.After(time.Second):
				t.Fatal("blocked after cancellation")
			}
		})
	}
}

type f05Endless struct{}

func (f05Endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	if len(p) > 0 {
		p[len(p)-1] = '\n'
	}
	return len(p), nil
}

type f05ClosedWriter struct{}

func (f05ClosedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestF05EndlessInputCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"wc", nil}, {"cut", []string{"-b", "1"}}, {"md5sum", nil}, {"sha3sum", nil}, {"base64", nil}, {"xxd", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, _ := Lookup(tc.name)
			c, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			done := make(chan int, 1)
			go func() {
				done <- cmd.Run(&Context{Ctx: c, Args: tc.args, Stdin: f05Endless{}, Stdout: io.Discard, Stderr: io.Discard})
			}()
			select {
			case code := <-done:
				if code != 124 {
					t.Errorf("got %d, want 124", code)
				}
			case <-time.After(time.Second):
				t.Fatal("endless input did not stop")
			}
		})
	}
}

func TestF05ClosedOutputStopsLineStreams(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{{"head", []string{"-n", "100000000"}}, {"cut", []string{"-b", "1"}}} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, _ := Lookup(tc.name)
			c, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan int, 1)
			go func() {
				done <- cmd.Run(&Context{Ctx: c, Args: tc.args, Stdin: f05Endless{}, Stdout: f05ClosedWriter{}, Stderr: io.Discard})
			}()
			select {
			case code := <-done:
				if code == 0 {
					t.Error("expected write failure")
				}
			case <-time.After(time.Second):
				t.Fatal("closed output did not stop input")
			}
		})
	}
}

func TestF05XXDSeekUsesAbsoluteOffset(t *testing.T) {
	input := strings.NewReader("abcdef")
	_, _ = input.Seek(4, io.SeekStart)
	var out bytes.Buffer
	code := runXXD(&Context{Ctx: context.Background(), Args: []string{"-s", "1", "-l", "1", "-p"}, Stdin: input, Stdout: &out, Stderr: io.Discard})
	if code != 0 || out.String() != "62\n" {
		t.Fatalf("code %d output %q", code, out.String())
	}
}
