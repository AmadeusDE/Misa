package builtins

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFindExecSemicolonInjection(t *testing.T) {
	dir := t.TempDir()
	name := "odd ; $(bad)\nfile"
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
		t.Fatal(err)
	}
	var got []string
	c := &Context{Dir: dir, Args: []string{name, "-exec", "capture", "pre{}post", "{}", ";"}, Env: []string{"PATH=/bin", "X=y"}, Exec: func(_ context.Context, cmd string, sub *Context) int {
		got = append([]string{cmd}, sub.Args...)
		if sub.Dir != dir || !reflect.DeepEqual(sub.Env, []string{"PATH=/bin", "X=y"}) {
			t.Errorf("context: %#v", sub)
		}
		return 0
	}}
	if status := Run(context.Background(), "find", c); status != 0 {
		t.Fatalf("status %d", status)
	}
	if !reflect.DeepEqual(got, []string{"capture", "pre" + name + "post", name}) {
		t.Fatalf("args %#v", got)
	}
}

func TestFindExecBatchFlushAndFailure(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var calls [][]string
	c := &Context{Dir: dir, Args: []string{".", "-type", "f", "-exec", "capture", "{}", "+"}, Exec: func(_ context.Context, _ string, sub *Context) int {
		calls = append(calls, append([]string(nil), sub.Args...))
		return 1
	}}
	if status := Run(context.Background(), "find", c); status == 0 {
		t.Fatal("batch failure lost")
	}
	if !reflect.DeepEqual(calls, [][]string{{"a", "b"}}) && !reflect.DeepEqual(calls, [][]string{{"./a", "./b"}}) {
		t.Fatalf("calls %#v", calls)
	}
}

func TestFindExecMalformed(t *testing.T) {
	for _, args := range [][]string{{"-exec", ";"}, {"-exec", "echo", "{}"}, {"-exec", "echo", "x{}", "+"}, {"-exec", "echo", "{}", "{}", "+"}, {"-ok", "echo", "{}", ";"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			p := findParser{ctx: &Context{}, args: args}
			if _, err := p.parse(); err == nil {
				t.Fatalf("accepted %q", args)
			}
		})
	}
}

func TestFindExecTruthAndPath(t *testing.T) {
	for _, tc := range []struct {
		name        string
		env         []string
		command     string
		code        int
		truth, fail bool
	}{
		{"false", []string{"PATH=/bin"}, "external", 1, false, false},
		{"success", []string{"PATH=/bin"}, "external", 0, true, false},
		{"insecure", []string{"PATH=/bin:."}, "external", 0, false, true},
		{"builtin", []string{"PATH=:"}, "true", 0, true, false},
		{"explicit-path", []string{"PATH=:"}, "./external", 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &Context{Dir: t.TempDir(), Env: tc.env, Stderr: &strings.Builder{}, Exec: func(context.Context, string, *Context) int { return tc.code }}
			p := findParser{ctx: ctx, args: []string{"-execdir", tc.command, "{}", ";"}}
			node, err := p.parse()
			if err != nil {
				t.Fatal(err)
			}
			if got := node(&findEntry{ctx: ctx, path: "foo", full: filepath.Join(ctx.Dir, "foo")}); got != tc.truth {
				t.Fatalf("truth %v", got)
			}
			if got := p.flushExec(); got != tc.fail {
				t.Fatalf("failure %v", got)
			}
		})
	}
}

func TestFindExecBudget(t *testing.T) {
	var batches [][]string
	ctx := &Context{Dir: t.TempDir(), Env: []string{}, Exec: func(_ context.Context, _ string, sub *Context) int {
		batches = append(batches, append([]string(nil), sub.Args...))
		return 0
	}}
	p := findParser{ctx: ctx, args: []string{"-exec", "capture", "{}", "+"}}
	node, err := p.parse()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		node(&findEntry{ctx: ctx, path: strings.Repeat("x", 9000), full: "/x"})
	}
	if p.flushExec() || len(batches) != 2 || len(batches[0]) != 3 || len(batches[1]) != 1 {
		t.Fatalf("batches %#v", batches)
	}
}

func TestFindExecStreamsCancellationAndQuit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	in := strings.NewReader("input")
	out := &strings.Builder{}
	errout := &strings.Builder{}
	calls := 0
	ctx := &Context{Dir: dir, Stdin: in, Stdout: out, Stderr: errout, Env: []string{"X=y"}, Args: []string{"a", "-exec", "capture", "{}", "+", "-quit"}, Exec: func(c context.Context, _ string, sub *Context) int {
		calls++
		if sub.Stdin != in || sub.Stdout != out || sub.Stderr != errout || !reflect.DeepEqual(sub.Env, []string{"X=y"}) {
			t.Error("streams or environment changed")
		}
		return 0
	}}
	if status := Run(context.Background(), "find", ctx); status != 0 || calls != 1 {
		t.Fatalf("status %d calls %d", status, calls)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	ctx.Args = []string{".", "-exec", "capture", "{}", ";"}
	if status := Run(canceled, "find", ctx); status == 0 || calls != 0 {
		t.Fatalf("cancellation status %d calls %d", status, calls)
	}
}

func TestFindExecBuiltinFallback(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	ctx := &Context{Dir: dir, Stdout: &out, Args: []string{".", "-maxdepth", "0", "-exec", "echo", "x{}y", ";"}}
	if status := Run(context.Background(), "find", ctx); status != 0 || out.String() != "x.y\n" {
		t.Fatalf("status %d output %q", status, out.String())
	}
}

func TestFindExecBatchContinuesAfterFailureAndOversize(t *testing.T) {
	calls := 0
	stderr := &strings.Builder{}
	ctx := &Context{Dir: t.TempDir(), Env: []string{}, Stderr: stderr, Exec: func(_ context.Context, _ string, sub *Context) int {
		calls++
		if calls == 1 {
			return 1
		}
		return 0
	}}
	for _, d := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(ctx.Dir, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	p := findParser{ctx: ctx, args: []string{"-execdir", "capture", "{}", "+"}}
	node, err := p.parse()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a/first", "b/second"} {
		node(&findEntry{ctx: ctx, path: n, full: filepath.Join(ctx.Dir, n)})
	}
	if !p.flushExec() || calls != 2 {
		t.Fatalf("calls %d", calls)
	}
	p = findParser{ctx: ctx, args: []string{"-exec", "capture", "{}", "+"}}
	node, err = p.parse()
	if err != nil {
		t.Fatal(err)
	}
	node(&findEntry{ctx: ctx, path: strings.Repeat("x", findExecBudget), full: "/x"})
	node(&findEntry{ctx: ctx, path: "small", full: "/small"})
	if !p.flushExec() || calls != 3 || !strings.Contains(stderr.String(), "budget") {
		t.Fatalf("calls %d stderr %q", calls, stderr.String())
	}
}
