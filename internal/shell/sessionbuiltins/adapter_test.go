package sessionbuiltins

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
	"github.com/AmadeusDE/Misa/internal/shell/session"
)

func TestInvocationAndNestedExec(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "marker")
	dir := t.TempDir()
	var out, stderr bytes.Buffer
	builtins.Register(builtins.Command{Name: "sessionbuiltins-test-probe", Run: func(c *builtins.Context) int {
		if c.CmdName != "sessionbuiltins-test-probe" || c.Dir != dir || !reflect.DeepEqual(c.Args, []string{"one", "two"}) {
			t.Errorf("invocation: %+v", c)
		}
		if got := c.CancellationContext().Value(contextKey{}); got != "marker" {
			t.Errorf("context marker = %v", got)
		}
		if got := c.Environ(); !reflect.DeepEqual(got, []string{"LOCAL=inside"}) {
			t.Errorf("environment = %q", got)
		}
		b, err := io.ReadAll(c.Stdin)
		if err != nil || string(b) != "input" {
			t.Errorf("stdin = %q, %v", b, err)
		}
		fmt.Fprint(c.Stderr, "diagnostic")
		if c.Exec == nil {
			t.Error("missing nested executor")
			return 99
		}
		return c.Exec(c.CancellationContext(), "printenv", &builtins.Context{Dir: c.Dir, Env: c.Env, Args: []string{"LOCAL"}, Stdout: c.Stdout, Stderr: c.Stderr})
	}})
	status := Run(ctx, session.Invocation{Name: "sessionbuiltins-test-probe", Args: []string{"one", "two"}, Dir: dir, Env: []string{"LOCAL=inside"}, Stdin: strings.NewReader("input"), Stdout: &out, Stderr: &stderr})
	if status != 0 || out.String() != "inside\n" || stderr.String() != "diagnostic" {
		t.Fatalf("status %d, stdout %q, stderr %q", status, out.String(), stderr.String())
	}
}

func TestStatusAndEmptyEnvironment(t *testing.T) {
	var out, stderr bytes.Buffer
	inv := session.Invocation{Name: "printenv", Args: []string{"SESSIONBUILTINS_ABSENT"}, Dir: t.TempDir(), Env: []string{}, Stdout: &out, Stderr: &stderr}
	if status := Run(context.Background(), inv); status != 1 || out.Len() != 0 {
		t.Fatalf("status %d, stdout %q, stderr %q", status, out.String(), stderr.String())
	}
	inv.Name = "false"
	inv.Args = nil
	if status := Run(context.Background(), inv); status != 1 {
		t.Fatalf("false status %d", status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if status := Run(ctx, inv); status != 130 {
		t.Fatalf("canceled status %d", status)
	}
}

func TestSessionIntegration(t *testing.T) {
	var out, stderr bytes.Buffer
	dir := t.TempDir()
	builtins.Register(builtins.Command{Name: "sessionbuiltins-integration-probe", Run: func(c *builtins.Context) int {
		if c.Dir != dir || !reflect.DeepEqual(c.Args, []string{"a b"}) || c.Getenv("LOCAL") != "session-value" {
			t.Errorf("session invocation: %+v", c)
		}
		input, err := io.ReadAll(c.Stdin)
		if err != nil || string(input) != "session input" {
			t.Errorf("session stdin %q, %v", input, err)
		}
		fmt.Fprint(c.Stderr, "probe stderr")
		return c.Exec(c.CancellationContext(), "printenv", &builtins.Context{Dir: c.Dir, Env: c.Env, Args: []string{"LOCAL"}, Stdout: c.Stdout})
	}})
	s, err := session.New(session.Options{Dir: dir, Env: []string{"LOCAL=session-value"}, Stdin: strings.NewReader("session input"), Stdout: &out, Stderr: &stderr, Exec: Run})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := s.Run(context.Background(), "sessionbuiltins-integration-probe 'a b'")
	if r.Err != nil || r.Status != 0 || out.String() != "session-value\n" {
		t.Fatalf("result %+v, stdout %q, stderr %q", r, out.String(), stderr.String())
	}
	if stderr.String() != "probe stderr" {
		t.Fatalf("stderr %q", stderr.String())
	}
	r = s.Run(context.Background(), "printenv MISSING_SESSIONBUILTINS_TEST")
	if r.Err != nil || r.Status != 1 {
		t.Fatalf("missing variable: %+v", r)
	}
}
