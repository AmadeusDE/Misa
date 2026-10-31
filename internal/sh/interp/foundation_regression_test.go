package interp

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/sh/expand"
	"github.com/AmadeusDE/Misa/internal/sh/syntax"
)

func foundationParse(t *testing.T, src string) *syntax.File {
	t.Helper()
	f, err := syntax.NewParser(syntax.Variant(syntax.LangPOSIX)).Parse(strings.NewReader(src), "foundation.sh")
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return f
}

var foundationExternalError = errors.New("external execution forbidden")

func foundationRunner(t *testing.T, out, stderr *bytes.Buffer) *Runner {
	t.Helper()
	dir := t.TempDir()
	r, err := New(Dir(dir), Env(expand.ListEnviron("PATH=/nonexistent", "HOME="+dir, "LC_ALL=C")),
		StdIO(strings.NewReader(""), out, stderr),
		ExecHandler(func(context.Context, []string) error { return foundationExternalError }))
	if err != nil {
		t.Fatalf("create runner: %v", err)
	}
	return r
}

func TestFoundationPOSIXPrintReparse(t *testing.T) {
	f := foundationParse(t, "x='a b'; printf '%s\\n' \"$x\"\n")
	var printed bytes.Buffer
	if err := syntax.NewPrinter().Print(&printed, f); err != nil {
		t.Fatalf("print AST: %v", err)
	}
	foundationParse(t, printed.String())
}

func TestFoundationExecution(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"quotes", `x='a b'; printf '<%s>\n' "$x" '' "${x%% *}"`, "<a b>\n<>\n<a>\n"},
		{"lists-pipeline", `false && printf bad; true || printf bad; printf 'abc\n' | read x; printf 'ok\n'`, "ok\n"},
		{"function", `f() { printf '[%s]\n' "$1"; }; f 'x y'`, "[x y]\n"},
		{"heredoc", "x=world\nread line <<EOF\nhello $x\nEOF\nprintf '%s\\n' \"$line\"\n", "hello world\n"},
		{"subshell-isolation", `x=outer; (x=inner); printf '%s\n' "$x"`, "outer\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			r := foundationRunner(t, &out, &stderr)
			err := r.Run(context.Background(), foundationParse(t, tc.src))
			if err != nil || out.String() != tc.want || stderr.Len() != 0 {
				t.Fatalf("Run: stdout=%q (want %q), stderr=%q (want empty), error=%v (want nil)", out.String(), tc.want, stderr.String(), err)
			}
		})
	}
}

func TestFoundationRunnerState(t *testing.T) {
	var out1, err1, out2, err2 bytes.Buffer
	r1 := foundationRunner(t, &out1, &err1)
	r2 := foundationRunner(t, &out2, &err2)
	if err := r1.Run(context.Background(), foundationParse(t, `x=first`)); err != nil {
		t.Fatalf("set variable: %v", err)
	}
	for _, tc := range []struct {
		r           *Runner
		out, stderr *bytes.Buffer
		want        string
	}{{r1, &out1, &err1, "first\n"}, {r2, &out2, &err2, "unset\n"}} {
		err := tc.r.Run(context.Background(), foundationParse(t, `printf '%s\n' "${x-unset}"`))
		if err != nil || tc.out.String() != tc.want || tc.stderr.Len() != 0 {
			t.Errorf("runner state: stdout=%q (want %q), stderr=%q, error=%v", tc.out.String(), tc.want, tc.stderr.String(), err)
		}
	}
}

func TestFoundationPreCancellation(t *testing.T) {
	var out, stderr bytes.Buffer
	r := foundationRunner(t, &out, &stderr)
	f := foundationParse(t, `while :; do :; done`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx, f) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("pre-canceled Run: got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pre-canceled Run did not return")
	}
}

func TestFoundationExternalRejected(t *testing.T) {
	var out, stderr bytes.Buffer
	r := foundationRunner(t, &out, &stderr)
	err := r.Run(context.Background(), foundationParse(t, `not_a_builtin`))
	if !errors.Is(err, foundationExternalError) {
		t.Fatalf("external command: got error %v, want %v", err, foundationExternalError)
	}
	if out.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("external command: stdout=%q stderr=%q, want empty", out.String(), stderr.String())
	}
}
