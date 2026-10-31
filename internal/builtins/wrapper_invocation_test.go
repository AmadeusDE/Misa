package builtins

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestEnvInjectedInvocation(t *testing.T) {
	t.Setenv("MISA_WRAPPER_GLOBAL", "unchanged")
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errout bytes.Buffer
	in := strings.NewReader("input")
	dir := t.TempDir()
	ctx := &Context{Dir: dir, Args: []string{"-u", "DROP", "-C", "child", "ADD=value", "tool", "arg"}, CmdName: "env", Width: 81, Height: 22, StdinTTY: true, StdoutTTY: true, StderrTTY: true, Stdin: in, Stdout: &out, Stderr: &errout, Env: []string{"KEEP=yes", "DROP=x"}, Ctx: cancelCtx}
	called := false
	ctx.Exec = func(c context.Context, name string, nested *Context) int {
		called = true
		if c != cancelCtx || nested.Ctx != cancelCtx || name != "tool" || nested.CmdName != "tool" {
			t.Errorf("wrong invocation context or name: %q %#v", name, nested)
		}
		if nested.Dir != dir+"/child" || !reflect.DeepEqual(nested.Args, []string{"arg"}) {
			t.Errorf("wrong dir or args: %#v", nested)
		}
		if nested.Stdin != in || nested.Stdout != &out || nested.Stderr != &errout || nested.Width != 81 || nested.Height != 22 || !nested.StdinTTY || !nested.StdoutTTY || !nested.StderrTTY || nested.Exec == nil {
			t.Error("invocation metadata not preserved")
		}
		env := map[string]bool{}
		for _, e := range nested.Env {
			env[e] = true
		}
		if !reflect.DeepEqual(env, map[string]bool{"KEEP=yes": true, "ADD=value": true}) {
			t.Errorf("environment: %v", nested.Env)
		}
		if got := os.Getenv("MISA_WRAPPER_GLOBAL"); got != "unchanged" {
			t.Errorf("process environment changed: %q", got)
		}
		return 42
	}
	if status := runEnv(ctx); status != 42 || !called {
		t.Fatalf("status=%d called=%v stderr=%s", status, called, &errout)
	}
	if got := ctx.Args; !reflect.DeepEqual(got, []string{"-u", "DROP", "-C", "child", "ADD=value", "tool", "arg"}) {
		t.Errorf("parent args changed: %v", got)
	}
}

func TestEnvExplicitEmptyInvocation(t *testing.T) {
	t.Setenv("MISA_WRAPPER_EMPTY", "inherited")
	var out, errout bytes.Buffer
	ctx := &Context{Args: []string{"-S", "tool ${MISA_WRAPPER_EMPTY}"}, Env: []string{}, Stdout: &out, Stderr: &errout}
	called := false
	ctx.Exec = func(_ context.Context, name string, nested *Context) int {
		called = true
		if name != "tool" || !reflect.DeepEqual(nested.Args, []string{""}) || nested.Env == nil || len(nested.Env) != 0 {
			t.Errorf("name=%q args=%#v env=%#v", name, nested.Args, nested.Env)
		}
		return 0
	}
	if status := runEnv(ctx); status != 0 || !called {
		t.Fatalf("status=%d called=%v stderr=%s", status, called, &errout)
	}
	out.Reset()
	ctx.Args = nil
	if status := runEnv(ctx); status != 0 || out.Len() != 0 {
		t.Fatalf("status=%d output=%q", status, out.String())
	}
}

func TestEnvInjectedIgnoreAndInheritance(t *testing.T) {
	t.Setenv("MISA_WRAPPER_INHERITED", "host")
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"inherit", []string{"tool"}, true},
		{"ignore", []string{"-i", "tool"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			ctx := &Context{Args: tc.args, Stdout: &out, Stderr: &errout}
			called := false
			ctx.Exec = func(_ context.Context, _ string, nested *Context) int {
				called = true
				if nested.Env == nil {
					t.Error("nested environment must be explicit")
				}
				found := false
				for _, e := range nested.Env {
					if e == "MISA_WRAPPER_INHERITED=host" {
						found = true
					}
				}
				if found != tc.want {
					t.Errorf("inherited variable found=%v want %v", found, tc.want)
				}
				return 0
			}
			if status := runEnv(ctx); status != 0 || !called {
				t.Fatalf("status=%d called=%v", status, called)
			}
		})
	}
}

func TestEnvInjectedCancellation(t *testing.T) {
	cancellation, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errout bytes.Buffer
	ctx := &Context{Args: []string{"tool"}, Ctx: cancellation, Stdout: &out, Stderr: &errout}
	ctx.Exec = func(c context.Context, _ string, nested *Context) int {
		if c.Err() != context.Canceled || nested.CancellationContext().Err() != context.Canceled {
			t.Error("cancellation not propagated")
		}
		return 19
	}
	if status := runEnv(ctx); status != 19 {
		t.Fatalf("status=%d", status)
	}
}
