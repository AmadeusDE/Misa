package builtins

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type f05FailWriter struct{}

func (f05FailWriter) Write([]byte) (int, error) { return 0, errors.New("closed output") }

type f05SignalReader struct {
	started chan struct{}
	release chan struct{}
}

func (r *f05SignalReader) Read([]byte) (int, error) { close(r.started); <-r.release; return 0, io.EOF }

func TestF05CopyStreamingCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*Context) int
		args []string
	}{
		{"cat", runCat, nil}, {"cat-formatted", runCat, []string{"-n"}}, {"tee", runTee, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := &f05SignalReader{make(chan struct{}), make(chan struct{})}
			defer close(r.release)
			done := make(chan int, 1)
			go func() {
				done <- tc.run(&Context{Ctx: c, Args: tc.args, Stdin: r, Stdout: io.Discard, Stderr: io.Discard})
			}()
			select {
			case <-r.started:
			case <-time.After(time.Second):
				t.Fatal("read not started")
			}
			cancel()
			select {
			case code := <-done:
				if code != 130 {
					t.Fatalf("status %d", code)
				}
			case <-time.After(time.Second):
				t.Fatal("did not cancel")
			}
		})
	}
}

func TestF05CatOutputError(t *testing.T) {
	for _, args := range [][]string{nil, {"-n"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code := runCat(&Context{Args: args, Stdin: strings.NewReader("one\ntwo\n"), Stdout: f05FailWriter{}, Stderr: io.Discard})
			if code == 0 {
				t.Fatal("output failure reported success")
			}
		})
	}
}

func TestF05TeeStopsOnStdoutError(t *testing.T) {
	dir := t.TempDir()
	code := runTee(&Context{Dir: dir, Args: []string{"out"}, Stdin: strings.NewReader("data"), Stdout: f05FailWriter{}, Stderr: io.Discard})
	if code == 0 {
		t.Fatal("output failure reported success")
	}
	b, err := os.ReadFile(filepath.Join(dir, "out"))
	if err != nil || len(b) != 0 {
		t.Fatalf("output file %q, %v", b, err)
	}
}

func TestF05CpCancellation(t *testing.T) {
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "out")
	c, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- runCp(&Context{Ctx: c, Dir: dir, Args: []string{"/dev/zero", dst}, Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
	}()
	deadline := time.After(time.Second)
	for {
		if _, err := os.Stat(dst); err == nil {
			break
		}
		select {
		case code := <-done:
			t.Fatalf("cp returned early: %d", code)
		case <-deadline:
			t.Fatal("copy did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case code := <-done:
		if code != 130 {
			t.Fatalf("status %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("copy did not cancel")
	}
}

func TestF05CatZeroCancellation(t *testing.T) {
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip(err)
	}
	for _, args := range [][]string{{"/dev/zero"}, {"-n", "/dev/zero"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			c, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan int, 1)
			go func() { done <- runCat(&Context{Ctx: c, Args: args, Stdout: io.Discard, Stderr: io.Discard}) }()
			time.Sleep(10 * time.Millisecond)
			cancel()
			select {
			case code := <-done:
				if code != 130 {
					t.Fatalf("status %d", code)
				}
			case <-time.After(time.Second):
				t.Fatal("cat did not cancel")
			}
		})
	}
}

func TestF05CatFormattedLongLineAndBrokenOutput(t *testing.T) {
	line := strings.Repeat("x", 12000)
	var out strings.Builder
	if code := runCat(&Context{Args: []string{"-n"}, Stdin: strings.NewReader(line + "\nnext\n"), Stdout: &out, Stderr: io.Discard}); code != 0 {
		t.Fatalf("status %d", code)
	}
	if want := "     1\t" + line + "\n     2\tnext\n"; out.String() != want {
		t.Fatal("long line formatting changed")
	}
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip(err)
	}
	done := make(chan int, 1)
	c, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- runCat(&Context{Ctx: c, Args: []string{"-n", "/dev/zero"}, Stdout: f05FailWriter{}, Stderr: io.Discard})
	}()
	select {
	case code := <-done:
		if code == 0 {
			t.Fatal("output failure reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("cat did not stop on broken output")
	}
}
