package session

import (
	"bytes"
	"context"
	"slices"
	"testing"
)

type expansionCase struct {
	name, src, out, stderr string
	status                 int
}

func runExpansionCases(t *testing.T, cases []expansionCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var out, stderr bytes.Buffer
			s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Stdout: &out, Stderr: &stderr})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := s.Run(context.Background(), tc.src)
			if out.String() != tc.out || stderr.String() != tc.stderr || r.Status != tc.status || r.Err != nil {
				t.Errorf("source %q: stdout=%q stderr=%q status=%d err=%v; want stdout=%q stderr=%q status=%d err=nil", tc.src, out.String(), stderr.String(), r.Status, r.Err, tc.out, tc.stderr, tc.status)
			}
		})
	}
}

func TestParameterExpansionOperators(t *testing.T) {
	states := []struct{ name, prefix string }{{"unset", "unset x; "}, {"empty", "x=''; "}, {"set", "x=abcabc; "}}
	ops := []struct {
		op   string
		want [3]string
	}{
		{"-word", [3]string{"word", "", "abcabc"}}, {":-word", [3]string{"word", "word", "abcabc"}},
		{"=word", [3]string{"word", "", "abcabc"}}, {":=word", [3]string{"word", "word", "abcabc"}},
		{"+word", [3]string{"", "word", "word"}}, {":+word", [3]string{"", "", "word"}},
		{"%a*", [3]string{"", "", "abc"}}, {"%%a*", [3]string{"", "", ""}},
		{"#*c", [3]string{"", "", "abc"}}, {"##*c", [3]string{"", "", ""}},
	}
	var cases []expansionCase
	for _, op := range ops {
		for i, state := range states {
			cases = append(cases, expansionCase{name: state.name + "/" + op.op, src: state.prefix + `printf '<%s>' "${x` + op.op + `}"`, out: "<" + op.want[i] + ">"})
		}
	}
	cases = append(cases, expansionCase{name: "length", src: `unset x; printf '%s:' "${#x}"; x=''; printf '%s:' "${#x}"; x=abcabc; printf '%s' "${#x}"`, out: "0:0:6"},
		expansionCase{name: "assign persists", src: `unset x; printf '%s:' "${x=one}" "$x"; x=''; printf '%s:' "${x:=two}" "$x"`, out: "one:one:two:two:"},
	)
	for _, op := range []string{"?", ":?"} {
		for _, state := range states {
			fail := state.name == "unset" || (op == ":?" && state.name == "empty")
			c := expansionCase{name: state.name + "/" + op, src: state.prefix + `printf '<%s>' "${x` + op + `problem}"`, out: "<abcabc>"}
			if state.name == "empty" {
				c.out = "<>"
			}
			if fail {
				c.out = ""
				c.stderr = "x: problem\n"
				c.status = 1
			}
			cases = append(cases, c)
		}
	}
	runExpansionCases(t, cases)
}

func TestParameterExpansionLazyWords(t *testing.T) {
	runExpansionCases(t, []expansionCase{
		{name: "unused defaults", src: `x=yes; unset y; printf '%s:%s:%s' "${x:-${y:=bad}}" "${x-${y:=bad}}" "${y-unset}"`, out: "yes:yes:unset"},
		{name: "unused alternatives", src: `unset x y; printf '%s:%s:%s' "${x+${y:=bad}}" "${x:+${y:=bad}}" "${y-unset}"`, out: "::unset"},
		{name: "unused error and assignment words", src: `x=yes; unset y; printf '%s:%s:%s' "${x?${y:=bad}}" "${x=${y:=bad}}" "${y-unset}"`, out: "yes:yes:unset"},
		{name: "unused substitution", src: `x=yes; printf '%s' "${x:-$(printf ignored >&2)}"`, out: "yes"},
		{name: "used word", src: `unset x y; printf '%s:%s' "${x:-${y:=made}}" "$y"`, out: "made:made"},
	})
}

func TestParameterOperatorWordQuotes(t *testing.T) {
	cases := []struct {
		name, src string
		args      []string
	}{
		{"single quoted default", `unset x; probe ${x:-'a b'}`, []string{"a b"}},
		{"double quoted default", `unset x; probe ${x:-"a b"}`, []string{"a b"}},
		{"quoted pattern", `x=abc; probe "${x%"b*"}"`, []string{"abc"}},
		{"unquoted default splits", `unset x; probe ${x:-a b}`, []string{"a", "b"}},
		{"empty quoted default", `unset x; probe ${x:-''}`, []string{""}},
		{"quoted alternate", `x=yes; probe ${x:+'a b'}`, []string{"a b"}},
		{"quoted assignment", `unset x; probe ${x:="a b"} "$x"`, []string{"a b", "a b"}},
		{"single substitution", `unset x; probe ${x:=$(printf 'a b')} "$x"`, []string{"a", "b", "a b"}},
		{"substitution splits", `unset x; probe ${x:-$(printf 'a b')}`, []string{"a", "b"}},
		{"quoted substitution", `unset x; probe ${x:-"$(printf 'a b')"}`, []string{"a b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var out, stderr bytes.Buffer
			var got []string
			s, err := New(Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}, Stdout: &out, Stderr: &stderr, Exec: func(_ context.Context, inv Invocation) int { got = append(got, inv.Args...); return 0 }})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := s.Run(context.Background(), tc.src)
			if !slices.Equal(got, tc.args) || r.Status != 0 || r.Err != nil || out.Len() != 0 || stderr.Len() != 0 {
				t.Errorf("args=%q status=%d err=%v stdout=%q stderr=%q; want args=%q status=0 err=nil empty output", got, r.Status, r.Err, out.String(), stderr.String(), tc.args)
			}
		})
	}
}
