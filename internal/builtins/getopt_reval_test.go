package builtins

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// runScript runs src in an mvdan/sh interpreter with the Misa getopt builtin
// wired in as the "getopt" command; other commands use the default handler.
// It returns the captured stdout/stderr and the final exit status.
func runScript(t *testing.T, src string) (stdout, stderr string, code int) {
	t.Helper()
	file, err := syntax.NewParser().Parse(strings.NewReader(src), "")
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	var outBuf, errBuf bytes.Buffer
	runner, err := interp.New(
		interp.Env(expand.ListEnviron()),
		interp.StdIO(nil, &outBuf, &errBuf),
		interp.ExecHandlers(func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return func(ctx context.Context, args []string) error {
				if args[0] != "getopt" {
					return next(ctx, args)
				}
				hc := interp.HandlerCtx(ctx)
				code := runGetopt(&Context{
					Dir:     hc.Dir,
					Args:    args[1:],
					CmdName: "getopt",
					Stdout:  hc.Stdout,
					Stderr:  hc.Stderr,
					Stdin:   hc.Stdin,
				})
				if code != 0 {
					return interp.ExitStatus(uint8(code))
				}
				return nil
			}
		}),
	)
	if err != nil {
		t.Fatalf("interp.New: %v", err)
	}
	if err := runner.Run(context.Background(), file); err != nil {
		var es interp.ExitStatus
		if !errors.As(err, &es) {
			t.Fatalf("run %q: %v", src, err)
		}
		return outBuf.String(), errBuf.String(), int(es)
	}
	return outBuf.String(), errBuf.String(), 0
}

// revalScript is the documented way to use getopt from a shell: the quoted
// output is re-parsed by eval, so every option, its argument and every operand
// survives unchanged (and the trailing "--" becomes a positional parameter).
func revalScript(call string) string {
	return "eval set -- \"$(" + call + ")\"\nprintf '<%s>\\n' \"$@\"\n"
}

func expectedArgs(args ...string) string {
	var b strings.Builder
	for _, a := range args {
		b.WriteString("<" + a + ">\n")
	}
	return b.String()
}

// TestGetoptRevalRoundTrip proves that eval set -- "$(getopt ...)" gives back
// the argument list the user passed: options (with their values) first, then
// the "--" separator, then the operands in their original order.
func TestGetoptRevalRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		call string
		want []string
	}{
		{
			name: "spaces",
			call: `getopt -o a: -- -a 'b c' 'd e'`,
			want: []string{"-a", "b c", "--", "d e"},
		},
		{
			name: "single quote",
			call: `getopt -o a -- "it's"`,
			want: []string{"--", "it's"},
		},
		{
			name: "double quote",
			call: `getopt -o a -- 'say "hi" now'`,
			want: []string{"--", `say "hi" now`},
		},
		{
			name: "backslashes",
			call: `getopt -o a -- 'a\b\\c\ d'`,
			want: []string{"--", `a\b\\c\ d`},
		},
		{
			name: "dollar signs",
			call: `getopt -o a -- '$HOME $(id) ` + "`x`" + ` ${y}'`,
			want: []string{"--", "$HOME $(id) `x` ${y}"},
		},
		{
			name: "newline inside an argument",
			call: `getopt -o a -- $'x\ny'`,
			want: []string{"--", "x\ny"},
		},
		{
			name: "empty argument",
			call: `getopt -o a -- '' ''`,
			want: []string{"--", "", ""},
		},
		{
			name: "literal double dash operand",
			call: `getopt -o a -- -- -- 'x y'`,
			want: []string{"--", "--", "x y"},
		},
		{
			name: "short option with a value",
			call: `getopt -o ab: -- -b 'v w' -a 'x'`,
			want: []string{"-b", "v w", "-a", "--", "x"},
		},
		{
			name: "required long option",
			call: `getopt -o a -l one:,two:: -- --one=a --two`,
			want: []string{"--one", "a", "--two", "", "--"},
		},
		{
			name: "optional long option with a value",
			call: `getopt -o a -l one:,two:: -- --two='o p' --one 'r'`,
			want: []string{"--two", "o p", "--one", "r", "--"},
		},
		{
			name: "long abbreviation",
			call: `getopt -o a -l one:,two: -- --on=v --tw 'w'`,
			want: []string{"--one", "v", "--two", "w", "--"},
		},
		{
			name: "multiple operands",
			call: `getopt -o a -- x -a 'y z' w`,
			want: []string{"-a", "--", "x", "y z", "w"},
		},
		{
			name: "unquoted mode",
			call: `getopt -u -o a: -- -a b c`,
			want: []string{"-a", "b", "--", "c"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runScript(t, revalScript(tc.call))
			if code != 0 {
				t.Errorf("exit status = %d, want 0 (stderr: %s)", code, stderr)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if want := expectedArgs(tc.want...); stdout != want {
				t.Errorf("printf output = %q, want %q", stdout, want)
			}
		})
	}
}

// TestGetoptRevalCount checks $# after the round trip: the options, the "--"
// separator and the operands all become positional parameters.
func TestGetoptRevalCount(t *testing.T) {
	src := "eval set -- \"$(getopt -o a: -- -a 'b c' '')\"\nprintf '%d\\n' \"$#\"\nprintf '<%s>\\n' \"$@\"\n"
	stdout, stderr, code := runScript(t, src)
	if code != 0 {
		t.Fatalf("exit status = %d, want 0 (stderr: %s)", code, stderr)
	}
	want := "4\n" + expectedArgs("-a", "b c", "--", "")
	if stdout != want {
		t.Errorf("output = %q, want %q", stdout, want)
	}
}

// TestGetoptRevalErrorStatus checks that a bad caller option fails the shell
// command while the line is still printed.
func TestGetoptRevalErrorStatus(t *testing.T) {
	src := "eval set -- \"$(getopt -n prog -o a -- -a -q x)\"\nprintf '%d\\n' \"$#\"\nprintf '<%s>\\n' \"$@\"\n"
	stdout, stderr, code := runScript(t, src)
	if code != 0 {
		t.Errorf("exit status = %d, want 0 (eval swallows getopt's status)", code)
	}
	if want := "prog: invalid option 'q'\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if want := "3\n" + expectedArgs("-a", "--", "x"); stdout != want {
		t.Errorf("output = %q, want %q", stdout, want)
	}
}
