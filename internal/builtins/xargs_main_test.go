package builtins

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestXargsExecution(t *testing.T) {
	tests := []struct {
		args   []string
		input  string
		want   [][]string
		status int
	}{
		{nil, "", [][]string{{}}, 0},
		{[]string{"-r"}, "", nil, 0},
		{[]string{"-n", "2", "cmd", "fixed"}, "a b c", [][]string{{"fixed", "a", "b"}, {"fixed", "c"}}, 0},
		{[]string{"-0", "-n1", "cmd"}, "a b\x00c\x00", [][]string{{"a b"}, {"c"}}, 0},
		{[]string{"-I", "{}", "cmd", "x{}y"}, "one two\nthree\n", [][]string{{"xone twoy"}, {"xthreey"}}, 0},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, "/"), func(t *testing.T) {
			var got [][]string
			var errout bytes.Buffer
			inv := &Context{Args: tt.args, Stdin: strings.NewReader(tt.input), Stdout: io.Discard, Stderr: &errout, Env: []string{}, Exec: func(_ context.Context, name string, c *Context) int {
				if name == "" {
					t.Error("empty command")
				}
				b, _ := io.ReadAll(c.Stdin)
				if len(b) > 0 || c.StdinTTY {
					t.Error("child stdin not empty")
				}
				got = append(got, append([]string{}, c.Args...))
				return 0
			}}
			status := runXargs(inv)
			if status != tt.status || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("status=%d got=%#v want=%#v stderr=%s", status, got, tt.want, errout.String())
			}
		})
	}
}

func TestXargsStatuses(t *testing.T) {
	for _, tt := range []struct{ child, want, calls int }{{1, 123, 2}, {255, 124, 1}, {130, 125, 1}, {126, 126, 1}, {127, 127, 1}} {
		t.Run(string(rune(tt.child)), func(t *testing.T) {
			calls := 0
			inv := &Context{Args: []string{"-n1", "cmd"}, Stdin: strings.NewReader("a b"), Stdout: io.Discard, Stderr: io.Discard, Env: []string{}, Exec: func(context.Context, string, *Context) int { calls++; return tt.child }}
			if got := runXargs(inv); got != tt.want || calls != tt.calls {
				t.Fatalf("status=%d calls=%d", got, calls)
			}
		})
	}
}

func TestXargsLimitsAndUnsupported(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		input  string
		want   [][]string
		status int
	}{
		{[]string{"-s", "9", "c"}, "aa bb cc", [][]string{{"aa", "bb"}, {"cc"}}, 0},
		{[]string{"-s", "9", "-x", "c"}, "aa bb cc", nil, 1},
		{[]string{"-p"}, "x", nil, 1},
		{[]string{"-o"}, "x", nil, 1},
	} {
		t.Run(strings.Join(tc.args, "/"), func(t *testing.T) {
			var got [][]string
			inv := &Context{Args: tc.args, Stdin: strings.NewReader(tc.input), Stdout: io.Discard, Stderr: io.Discard, Env: []string{}, Exec: func(_ context.Context, _ string, c *Context) int {
				got = append(got, append([]string{}, c.Args...))
				return 0
			}}
			status := runXargs(inv)
			if status != tc.status || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("status=%d got=%#v want=%#v", status, got, tc.want)
			}
		})
	}
}

func TestXargsStreamsBeforeEOF(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	calls := make(chan []string, 1)
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-n1", "cmd"}, Stdin: r, Stdout: io.Discard, Stderr: io.Discard, Env: []string{}, Exec: func(_ context.Context, _ string, c *Context) int { calls <- append([]string(nil), c.Args...); return 0 }}
	go func() { done <- runXargs(inv) }()
	go func() { _, _ = io.WriteString(w, "first\n") }()
	select {
	case args := <-calls:
		if !reflect.DeepEqual(args, []string{"first"}) {
			t.Fatal(args)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first batch waited for EOF")
	}
	w.Close()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatal(code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not finish")
	}
}

func TestXargsStreamingStopsBeforeReadingRemainder(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	done := make(chan int, 1)
	inv := &Context{Args: []string{"-n1", "cmd"}, Stdin: r, Stdout: io.Discard, Stderr: io.Discard, Env: []string{}, Exec: func(context.Context, string, *Context) int { return 255 }}
	go func() { done <- runXargs(inv) }()
	go func() { _, _ = io.WriteString(w, "first\n") }()
	select {
	case code := <-done:
		if code != 124 {
			t.Fatalf("status=%d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waited for more input after child failure")
	}
}
