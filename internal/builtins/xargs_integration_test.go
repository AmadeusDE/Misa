package builtins

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These tests exercise the registered command rather than its parsing internals.
func TestXargsIntegrationBatches(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		args        []string
		want        [][]string
	}{
		{"default", "one 'two three' four\\ five\n", []string{"worker", "fixed"}, [][]string{{"fixed", "one", "two three", "four five"}}},
		{"max args", "a b c d e", []string{"-n", "2", "worker", "fixed"}, [][]string{{"fixed", "a", "b"}, {"fixed", "c", "d"}, {"fixed", "e"}}},
		{"replace", "red blue\ngreen\n", []string{"-I", "{}", "worker", "pre{}post", "{}"}, [][]string{{"prered bluepost", "red blue"}, {"pregreenpost", "green"}}},
		{"empty default", "", []string{"worker", "fixed"}, [][]string{{"fixed"}}},
		{"empty no run", " \n\t", []string{"-r", "worker"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls [][]string
			var stderr bytes.Buffer
			inv := &Context{Args: tc.args, Stdin: strings.NewReader(tc.input), Stderr: &stderr}
			inv.Exec = func(_ context.Context, name string, sub *Context) int {
				if name != "worker" {
					t.Errorf("command = %q", name)
				}
				calls = append(calls, append([]string{}, sub.Args...))
				return 0
			}
			if code := Run(context.Background(), "xargs", inv); code != 0 || !reflect.DeepEqual(calls, tc.want) {
				t.Fatalf("status=%d calls=%#v want=%#v stderr=%q", code, calls, tc.want, stderr.String())
			}
		})
	}
}

func TestXargsIntegrationExitAggregation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		statuses    []int
		want, calls int
	}{
		{"success", []int{0, 0, 0}, 0, 3},
		{"ordinary failure continues", []int{7, 0, 1}, 123, 3},
		{"255 stops", []int{0, 255, 0}, 124, 2},
		{"not executable stops", []int{126, 0, 0}, 126, 1},
		{"not found stops", []int{127, 0, 0}, 127, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var stderr bytes.Buffer
			inv := &Context{Args: []string{"-n", "1", "worker"}, Stdin: strings.NewReader("a b c"), Stderr: &stderr}
			inv.Exec = func(_ context.Context, _ string, _ *Context) int {
				if calls >= len(tc.statuses) {
					t.Errorf("unexpected extra invocation %d", calls+1)
					calls++
					return 0
				}
				s := tc.statuses[calls]
				calls++
				return s
			}
			code := Run(context.Background(), "xargs", inv)
			if code != tc.want || calls != tc.calls {
				t.Fatalf("status=%d calls=%d want=%d,%d stderr=%q", code, calls, tc.want, tc.calls, stderr.String())
			}
		})
	}
}

func TestXargsIntegrationInvocation(t *testing.T) {
	dir := t.TempDir()
	env := []string{"PATH=", "MISA_XARGS_TEST=local"}
	var stderr bytes.Buffer
	called := false
	inv := &Context{Dir: dir, Env: env, Args: []string{"worker", "fixed"}, Stdin: strings.NewReader("item"), Stderr: &stderr}
	inv.Exec = func(_ context.Context, name string, sub *Context) int {
		called = true
		if name != "worker" || sub.Dir != dir || !reflect.DeepEqual(sub.Env, env) || !reflect.DeepEqual(sub.Args, []string{"fixed", "item"}) {
			t.Errorf("name=%q dir=%q env=%#v args=%#v", name, sub.Dir, sub.Env, sub.Args)
		}
		return 0
	}
	if code := Run(context.Background(), "xargs", inv); code != 0 || !called {
		t.Fatalf("status=%d called=%v stderr=%q", code, called, stderr.String())
	}
}

func TestXargsIntegrationExternalHelper(t *testing.T) {
	if os.Getenv("MISA_XARGS_CHILD") != "yes" {
		return
	}
	if status := os.Getenv("MISA_XARGS_EXIT"); status != "" {
		n, err := strconv.Atoi(status)
		if err != nil {
			os.Exit(90)
		}
		os.Exit(n)
	}
	dir, err := os.Getwd()
	if err != nil {
		os.Exit(90)
	}
	fmt.Fprintf(os.Stdout, "%s\n%s\n%s\n", dir, os.Getenv("MISA_XARGS_VALUE"), strings.Join(os.Args[len(os.Args)-2:], ","))
	os.Exit(0)
}

func TestXargsIntegrationExternal(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// Resolve symlinks for platforms whose temporary directory is symlinked.
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	inv := &Context{Dir: dir, Env: []string{"MISA_XARGS_CHILD=yes", "MISA_XARGS_VALUE=local"}, Args: []string{exe, "-test.run=^TestXargsIntegrationExternalHelper$"}, Stdin: strings.NewReader("one two"), Stdout: &out, Stderr: &stderr}
	if code := Run(context.Background(), "xargs", inv); code != 0 || out.String() != dir+"\nlocal\none,two\n" {
		t.Fatalf("status=%d out=%q stderr=%q", code, out.String(), stderr.String())
	}
}

func TestXargsIntegrationMissingCommand(t *testing.T) {
	var stderr bytes.Buffer
	inv := &Context{Dir: t.TempDir(), Env: []string{"PATH="}, Args: []string{"misa-xargs-no-such-command-9c312"}, Stdin: strings.NewReader("x"), Stderr: &stderr}
	if code := Run(context.Background(), "xargs", inv); code != 127 {
		t.Fatalf("status=%d stderr=%q", code, stderr.String())
	}
}

func TestXargsIntegrationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 1)
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-n", "1", "worker"}, Stdin: strings.NewReader("a b")}
	inv.Exec = func(ctx context.Context, _ string, _ *Context) int {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return 130
	}
	go func() { done <- Run(ctx, "xargs", inv) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("child did not start")
	}
	cancel()
	select {
	case code := <-done:
		if code != 130 {
			t.Fatalf("status=%d want 130", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("xargs did not stop on cancellation")
	}
}

// A producer may wait for the first result before sending more input. The
// reader must dispatch a full batch without waiting for the producer to close.
func TestXargsIntegrationLivePipeline(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := make(chan []string, 2)
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-n", "1", "worker"}, Stdin: reader}
	inv.Exec = func(_ context.Context, _ string, sub *Context) int {
		calls <- append([]string(nil), sub.Args...)
		return 0
	}
	go func() { done <- Run(ctx, "xargs", inv) }()
	go func() { _, _ = io.WriteString(writer, "first\n") }()
	select {
	case got := <-calls:
		if !reflect.DeepEqual(got, []string{"first"}) {
			t.Fatalf("first batch: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("first batch did not run before input EOF")
	}
	if _, err := io.WriteString(writer, "second\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-calls:
		if !reflect.DeepEqual(got, []string{"second"}) {
			t.Fatalf("second batch: %#v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second batch did not run")
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("status=%d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("xargs did not finish")
	}
}

// Direct external launches distinguish ordinary high exits from signal deaths
// and launch errors. Injected integer-only executors retain their legacy mapping.
func TestXargsIntegrationExternalHighExitStatuses(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ status, want int }{{126, 123}, {127, 123}, {130, 123}, {200, 123}, {255, 124}} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			var stderr bytes.Buffer
			inv := &Context{Dir: t.TempDir(), Env: []string{"MISA_XARGS_CHILD=yes", fmt.Sprintf("MISA_XARGS_EXIT=%d", tc.status)}, Args: []string{exe, "-test.run=^TestXargsIntegrationExternalHelper$"}, Stdin: strings.NewReader("item"), Stderr: &stderr}
			if got := Run(context.Background(), "xargs", inv); got != tc.want {
				t.Errorf("child exit=%d: xargs status=%d want=%d stderr=%q", tc.status, got, tc.want, stderr.String())
			}
		})
	}
}
