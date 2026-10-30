package builtins

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestInvocationEnvironment(t *testing.T) {
	t.Setenv("MISA_F02_ENV", "ambient")
	for _, tc := range []struct {
		env   []string
		want  string
		found bool
	}{
		{nil, "ambient", true}, {[]string{}, "", false}, {[]string{"MISA_F02_ENV=local"}, "local", true}, {[]string{"MISA_F02_ENV="}, "", true},
	} {
		c := &Context{Env: tc.env}
		got, found := c.LookupEnv("MISA_F02_ENV")
		if got != tc.want || found != tc.found || c.Getenv("MISA_F02_ENV") != tc.want {
			t.Fatalf("env %v: got %q, %v", tc.env, got, found)
		}
		if tc.env != nil && !reflect.DeepEqual(c.Environ(), tc.env) {
			t.Fatalf("Environ: %v", c.Environ())
		}
	}
}

func TestInvocationRun(t *testing.T) {
	var out, errout bytes.Buffer
	wantDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"original"}
	env := []string{"X=y"}
	inv := &Context{Dir: ".", Args: args, Env: env, Stdout: &out, Stderr: &errout, Stdin: bytes.NewBufferString("input"), CmdName: "alias"}
	original := *inv
	Register(Command{Name: "f02-invocation-test", Run: func(c *Context) int {
		if c.Dir != wantDir || c.CmdName != "alias" || c.CancellationContext() == nil {
			t.Errorf("bad normalization: %+v", c)
		}
		b, _ := io.ReadAll(c.Stdin)
		c.Stdout.Write(b)
		io.WriteString(c.Stderr, "error")
		c.Args[0] = "changed"
		c.Env[0] = "changed"
		c.Dir = "changed"
		return 42
	}})
	defer delete(registry, "f02-invocation-test")
	if got := Run(nil, "f02-invocation-test", inv); got != 42 {
		t.Fatal(got)
	}
	if out.String() != "input" || errout.String() != "error" {
		t.Fatalf("streams: %q %q", out.String(), errout.String())
	}
	if !reflect.DeepEqual(*inv, original) || args[0] != "original" || env[0] != "X=y" {
		t.Fatal("caller mutated")
	}
	Register(Command{Name: "f02-defaults-test", Run: func(c *Context) int {
		if c.CmdName != "f02-defaults-test" || c.Stdin == nil || c.Stdout == nil || c.Stderr == nil || c.Ctx == nil {
			t.Errorf("bad defaults: %+v", c)
		}
		b, _ := io.ReadAll(c.Stdin)
		if len(b) != 0 {
			t.Error("stdin not empty")
		}
		return 0
	}})
	defer delete(registry, "f02-defaults-test")
	if Run(nil, "f02-defaults-test", nil) != 0 || Run(nil, "f02-unknown", nil) != ExitNotFound {
		t.Fatal("unexpected status")
	}
}

func TestInvocationContextAndSleep(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	inv := &Context{Ctx: canceled, Args: []string{"10"}}
	if got := Run(nil, "sleep", inv); got != ExitCanceled {
		t.Fatalf("cancel: %d", got)
	}
	deadline, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if got := Run(deadline, "sleep", inv); got != ExitTimeout {
		t.Fatalf("deadline: %d", got)
	}
}

func TestInvocationExitStatus(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{nil, ExitSuccess}, {context.Canceled, ExitCanceled}, {context.DeadlineExceeded, ExitTimeout},
		{exec.ErrNotFound, ExitNotFound}, {os.ErrNotExist, ExitNotFound}, {os.ErrPermission, ExitCannotExecute}, {errors.New("other"), ExitFailure},
	} {
		if got := ExitStatus(tc.err); got != tc.want {
			t.Errorf("%v: %d want %d", tc.err, got, tc.want)
		}
	}
}
