//go:build unix

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/shell/session"
	"github.com/AmadeusDE/Misa/internal/shell/sessionbuiltins"
)

func TestProcessHelper(t *testing.T) {
	if os.Getenv("MISA_WORKER_HELPER") == "hold" {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}
	if os.Getenv("MISA_WORKER_HELPER") != "1" {
		return
	}
	if path := os.Getenv("MISA_WORKER_PIDFILE"); path != "" {
		_ = os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
	}
	f := os.NewFile(3, "control")
	opts := sessionbuiltins.Configure(&session.Options{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	originalExec := opts.Exec
	opts.Exec = func(ctx context.Context, inv session.Invocation) int {
		if inv.Name == "fixtureenv" {
			found := false
			for _, entry := range inv.Env {
				if entry == "y=yes" {
					found = true
				}
				if strings.HasPrefix(entry, "x=") {
					return 1
				}
			}
			if found {
				return 0
			}
			return 1
		}
		return originalExec(ctx, inv)
	}
	if os.Getenv("MISA_WORKER_CRASH") == "1" {
		opts.Exec = func(ctx context.Context, inv session.Invocation) int {
			if os.Getenv("MISA_NO_DESCENDANT") == "1" {
				os.Exit(37)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
			cmd.Env = []string{"MISA_WORKER_HELPER=hold"}
			cmd.ExtraFiles = []*os.File{f}
			if cmd.Start() != nil {
				os.Exit(38)
			}
			os.Exit(37)
			return 0
		}
	}
	err := Serve(context.Background(), f, Options{Session: *opts})
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestProcessPersistence(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Skip(e)
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, e := StartProcess(ctx, ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1"}, Dir: dir})
	if errors.Is(e, os.ErrPermission) {
		t.Skip(e)
	}
	if e != nil {
		t.Fatal(e)
	}
	defer c.Shutdown(context.Background())
	for _, s := range []string{"x=ok; export y=yes; f() { test \"$x:$y\" = ok:yes; }; cd /", "f && fixtureenv && test \"$PWD\" = /"} {
		r, e := c.Run(ctx, s)
		if e != nil || r.Status != 0 {
			t.Fatalf("%q: %+v %v", s, r, e)
		}
	}
}

func TestProcessDeathWithInheritedSocket(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Skip(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, e := StartProcess(ctx, ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1", "MISA_WORKER_CRASH=1"}})
	if errors.Is(e, os.ErrPermission) {
		t.Skip(e)
	}
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.Run(ctx, "crashfixture")
	var exit *ExitError
	if !errors.As(e, &exit) || exit.ExitCode() != 37 {
		t.Fatalf("expected status 37, got %v", e)
	}
	_ = c.Shutdown(context.Background())
}

func BenchmarkProcessStartup(b *testing.B) {
	exe, e := os.Executable()
	if e != nil {
		b.Skip(e)
	}
	opts := ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1"}}
	for b.Loop() {
		c, e := StartProcess(context.Background(), opts)
		if errors.Is(e, os.ErrPermission) {
			b.Skip(e)
		}
		if e != nil {
			b.Fatal(e)
		}
		if e = c.Shutdown(context.Background()); e != nil {
			b.Fatal(e)
		}
	}
}

func BenchmarkProcessRun(b *testing.B) {
	exe, e := os.Executable()
	if e != nil {
		b.Skip(e)
	}
	c, e := StartProcess(context.Background(), ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1"}})
	if errors.Is(e, os.ErrPermission) {
		b.Skip(e)
	}
	if e != nil {
		b.Fatal(e)
	}
	defer c.Shutdown(context.Background())
	b.ResetTimer()
	for b.Loop() {
		r, e := c.Run(context.Background(), "true")
		if e != nil || r.Status != 0 {
			b.Fatalf("%+v %v", r, e)
		}
	}
}

type stalledWriter struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (w *stalledWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}
func TestProcessBlockedOutputDoesNotPreventReaping(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Skip(e)
	}
	w := &stalledWriter{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(w.release)
	c, e := StartProcess(context.Background(), ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1"}, Stdout: w, ShutdownTimeout: 3 * time.Second})
	if errors.Is(e, os.ErrPermission) {
		t.Skip(e)
	}
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Run(context.Background(), "echo hello"); e != nil {
		t.Fatal(e)
	}
	select {
	case <-w.entered:
	case <-time.After(time.Second):
		t.Fatal("no output")
	}
	start := time.Now()
	if e = c.Shutdown(context.Background()); e != nil {
		t.Fatal(e)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("blocked shutdown")
	}
}

func TestProcessImmediateExitStatus(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Skip(e)
	}
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		c, e := StartProcess(ctx, ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1", "MISA_WORKER_CRASH=1", "MISA_NO_DESCENDANT=1"}})
		if errors.Is(e, os.ErrPermission) {
			cancel()
			t.Skip(e)
		}
		if e != nil {
			cancel()
			t.Fatal(e)
		}
		_, e = c.Run(ctx, "crashfixture")
		var exit *ExitError
		if !errors.As(e, &exit) || exit.ExitCode() != 37 {
			cancel()
			t.Fatalf("run %d: expected status 37, got %v", i, e)
		}
		_ = c.Shutdown(ctx)
		cancel()
	}
}

func TestProcessExternalFixture(t *testing.T) {
	if os.Getenv("MISA_EXTERNAL_FIXTURE") != "1" {
		return
	}
	time.Sleep(10 * time.Second)
	os.Exit(0)
}
func TestProcessCancelExternal(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	c, err := StartProcess(context.Background(), ProcessOptions{Executable: exe, Args: []string{"-test.run=^TestProcessHelper$"}, Env: []string{"MISA_WORKER_HELPER=1", "MISA_EXTERNAL_FIXTURE=1"}})
	if errors.Is(err, os.ErrPermission) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer c.Shutdown(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = c.Run(ctx, fmt.Sprintf("'%s' -test.run=^TestProcessExternalFixture$", strings.ReplaceAll(exe, "'", "'\\''")))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	r, err := c.Run(ctx2, "true")
	if err != nil || r.Status != 0 {
		t.Fatalf("recovery %+v %v", r, err)
	}
}
