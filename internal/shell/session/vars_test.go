package session

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestAssignmentOnlyStatus(t *testing.T) {
	cases := []struct {
		script string
		want   int
	}{
		{"x=1", 0},
		{"false; x=1", 0},
		{"x=$(exit 3)", 3},
		{"x=$(echo hi)", 0},
		{"x=1 y=$(exit 4) z=2", 4},
		{"x=$(exit 3) y=$(true)", 0},
		{"false; x=$(true) y=$(exit 5)", 5},
		{"x=1; x=2", 0},
	}
	for _, c := range cases {
		t.Run(c.script, func(t *testing.T) {
			s, _, _ := newTestSession(t, t.TempDir(), nil)
			if r := s.Run(context.Background(), c.script); r.Status != c.want || r.Err != nil {
				t.Fatalf("%s: %+v, want status %d", c.script, r, c.want)
			}
		})
	}
}

func TestTemporaryCommandEnvironment(t *testing.T) {
	root := t.TempDir()
	home := root + "/home"
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	var seen []Invocation
	var out, errb bytes.Buffer
	s, err := New(Options{
		Dir: root, Env: []string{"KEEP=yes", "HOME=" + root}, Stdout: &out, Stderr: &errb,
		Exec: func(_ context.Context, i Invocation) int {
			seen = append(seen, i)
			return 0
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run := func(src string) Result {
		t.Helper()
		r := s.Run(context.Background(), src)
		if r.Err != nil {
			t.Fatalf("%s: %+v", src, r)
		}
		return r
	}
	run("FOO=bar cmd")
	if len(seen) != 1 || !hasEnv(seen[0].Env, "FOO=bar") || !hasEnv(seen[0].Env, "KEEP=yes") {
		t.Fatalf("invocation %+v", seen)
	}
	seen = nil
	run("FOO=bar; FOO=baz cmd")
	if len(seen) != 1 || !hasEnv(seen[0].Env, "FOO=baz") {
		t.Fatalf("prefix must win: %+v", seen)
	}
	run("printf '%s' \"[$FOO]\"")
	if got := out.String(); got != "[bar]" {
		t.Fatalf("temporary assignment persisted: %q", got)
	}
	out.Reset()
	// A prefix assignment also applies to interpreter builtins.
	run("HOME=" + home + " cd")
	if got := s.Dir(); got != home {
		t.Fatalf("dir %q, want %q (stderr %q)", got, home, errb.String())
	}
	run("printf '%s' \"[$HOME]\"")
	if got := out.String(); got != "["+root+"]" {
		t.Fatalf("temporary HOME persisted: %q", got)
	}
	// Assignment-only commands and unexported variables never reach commands.
	run("LOCAL=1")
	seen = nil
	run("env")
	if len(seen) != 1 || hasEnv(seen[0].Env, "LOCAL=1") {
		t.Fatalf("assignment-only command exported: %+v", seen)
	}
}

func TestExportAndUnset(t *testing.T) {
	s, out, errb := newTestSession(t, t.TempDir(), []string{"KEEP=yes"})
	run := func(src string) Result {
		t.Helper()
		r := s.Run(context.Background(), src)
		if r.Err != nil {
			t.Fatalf("%s: %+v", src, r)
		}
		return r
	}
	run("export E=1")
	if !hasEnv(s.Env(), "E=1") || !s.Vars()["E"].Exported {
		t.Fatalf("export failed: %v", s.Env())
	}
	run("unset E")
	if hasEnv(s.Env(), "E=1") || s.Vars()["E"].IsSet() {
		t.Fatalf("unset failed: %v", s.Env())
	}
	run("export G=1; unset -v G")
	if hasEnv(s.Env(), "G=1") {
		t.Fatalf("unset -v failed: %v", s.Env())
	}
	// A variable created inside the shell stays unexported until export.
	run("PRIVATE=1")
	if hasEnv(s.Env(), "PRIVATE=1") {
		t.Fatal("unexported variable appeared in the environment")
	}
	// Unsigned values are written so that the output can be re-evaluated.
	out.Reset()
	run("export Q='a b'; export")
	listing := out.String()
	if !strings.Contains(listing, "export Q='a b'\n") || !strings.Contains(listing, "export KEEP=yes\n") {
		t.Fatalf("export listing %q", listing)
	}
	out.Reset()
	run("export -p")
	if !strings.Contains(out.String(), "export Q='a b'\n") {
		t.Fatalf("export -p listing %q", out.String())
	}
	// Unsupported options fail instead of being ignored.
	errb.Reset()
	if r := run("export -Z Q"); r.Status != 2 || !strings.Contains(errb.String(), "invalid option") {
		t.Fatalf("%+v stderr %q", r, errb.String())
	}
	// Documented limitation: an assignment prefix before the special builtin
	// export or readonly is not parsed as a declaration and fails.
	errb.Reset()
	if r := run("FOO=bar export FOO"); r.Status != 2 || errb.Len() == 0 {
		t.Fatalf("%+v stderr %q", r, errb.String())
	}
}

func TestReadonly(t *testing.T) {
	s, out, errb := newTestSession(t, t.TempDir(), nil)
	run := func(src string) Result {
		t.Helper()
		return s.Run(context.Background(), src)
	}
	errb.Reset()
	if r := run("readonly R=1; R=2"); r.Status != 1 || r.Err != nil {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(errb.String(), "readonly variable") {
		t.Fatalf("stderr %q", errb.String())
	}
	if got := s.Vars()["R"].Str; got != "1" {
		t.Fatalf("readonly value changed: %q", got)
	}
	// unset reports the failure once and keeps the variable.
	errb.Reset()
	if r := run("unset R"); r.Status != 1 || r.Err != nil {
		t.Fatalf("%+v", r)
	}
	if got := strings.Count(errb.String(), "readonly variable"); got != 1 {
		t.Fatalf("stderr %q", errb.String())
	}
	if !s.Vars()["R"].IsSet() || s.Vars()["R"].Str != "1" {
		t.Fatal(s.Vars())
	}
	// The attribute survives submissions and is not exported.
	if r := run("R=3"); r.Status != 1 || s.Vars()["R"].Str != "1" {
		t.Fatalf("%+v %v", r, s.Vars()["R"])
	}
	if hasEnv(s.Env(), "R=1") {
		t.Fatal("readonly variable exported")
	}
	// readonly marks a variable that does not exist yet.
	errb.Reset()
	if r := run("readonly LATER; LATER=1"); r.Status != 1 || !strings.Contains(errb.String(), "readonly variable") {
		t.Fatalf("%+v stderr %q", r, errb.String())
	}
	out.Reset()
	run("readonly")
	if !strings.Contains(out.String(), "readonly R=1\n") || !strings.Contains(out.String(), "readonly LATER\n") {
		t.Fatalf("readonly listing %q", out.String())
	}
	out.Reset()
	run("readonly -p")
	if !strings.Contains(out.String(), "readonly R=1\n") {
		t.Fatalf("readonly -p listing %q", out.String())
	}
}

func hasEnv(env []string, want string) bool { return indexEnv(env, want) >= 0 }

func indexEnv(env []string, entry string) int {
	for i, e := range env {
		if e == entry {
			return i
		}
	}
	return -1
}
