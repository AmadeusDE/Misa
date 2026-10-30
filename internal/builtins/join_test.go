// Test cases adapted from uutils/coreutils tests/by-util/test_join.rs (MIT),
// pinned commit fee61ff5d4f87e0f912a52e639942b5f7ad58663. See docs/CREDITS.md.

package builtins

import (
	"bytes"
	"strings"
	"testing"
)

func TestJoinCases(t *testing.T) {
	const f1 = "1\n2\n3\n5\n8\n"
	const f2 = "1 a\n2 b\n3 c\n4 d\n5 e\n6 f\n7 g\n8 h\n9 i\n"
	const basic = "1 a\n2 b\n3 c\n5 e\n8 h\n"
	tests := []struct {
		name, a, b string
		opts       []string
		out        string
	}{
		{"default", f1, f2, nil, basic},
		{"empty both", "", "", nil, ""}, {"empty first", "", f2, nil, ""}, {"empty second", f1, "", nil, ""},
		{"shared field", "x 1\ny 2\n", "p 1\nq 2\n", []string{"-j", "2"}, "1 x p\n2 y q\n"},
		{"separate fields", "x 1\ny 2\n", "1 p\n2 q\n", []string{"-1", "2", "-2", "1"}, "1 x p\n2 y q\n"},
		{"a1", "a 1\nb 2\n", "b x\nc y\n", []string{"-a", "1"}, "a 1\nb 2 x\n"},
		{"a2", "a 1\nb 2\n", "b x\nc y\n", []string{"-a", "2"}, "b 2 x\nc y\n"},
		{"both a", "a 1\nb 2\n", "b x\nc y\n", []string{"-a", "1", "-a", "2"}, "a 1\nb 2 x\nc y\n"},
		{"v1", "a 1\nb 2\n", "b x\nc y\n", []string{"-v", "1"}, "a 1\n"},
		{"v2", "a 1\nb 2\n", "b x\nc y\n", []string{"-v", "2"}, "c y\n"},
		{"a and v", "a 1\nb 2\n", "b x\nc y\n", []string{"-a", "1", "-v", "2"}, "a 1\nc y\n"},
		{"format", f1, f2, []string{"-o", "1.1 2.2"}, basic},
		{"format key", f1, f2, []string{"-o", "0 2.2"}, basic},
		{"format repeated", f1, f2, []string{"-o", "2.2", "-o", "1.1"}, "a 1\nb 2\nc 3\ne 5\nh 8\n"},
		{"auto", "1 x y\n2 p\n", "1 a\n2 b\n", []string{"-o", "auto"}, "1 x y a\n2 p  b\n"},
		{"auto repeated", f1, f2, []string{"-o", "auto", "-o", "auto"}, basic},
		{"auto mixed", f1, f2, []string{"-o", "auto", "-o", "2.2 1.1"}, "a 1\nb 2\nc 3\ne 5\nh 8\n"},
		{"missing filler", "1 a\n2 b\n", "1 c\n", []string{"-a1", "-e", "d", "-e", "d", "-o", "1.1,1.2,2.2"}, "1 a c\n2 b d\n"},
		{"empty filler", "a,,b\n", "a,,b\n", []string{"-t", ",", "-e", "EMPTY", "-o", "0,1.1,1.2,1.3"}, "a,a,EMPTY,b\n"},
		{"empty no filler", "a,,b\n", "a,,b\n", []string{"-t", ",", "-o", "0,1.1,1.2,1.3"}, "a,a,,b\n"},
		{"empty string filler", "a,,b\n", "a,,b\n", []string{"-t", ",", "-e", "", "-o", "0,1.1,1.2,1.3"}, "a,a,,b\n"},
		{"comma", "a,1\n", "a,2\n", []string{"-t", ","}, "a,1,2\n"},
		{"semicolon", "a;1\n", "a;2\n", []string{"-t", ";"}, "a;1;2\n"},
		{"whole line", "1 a\n1 b\n8 h\n", "1 a\n8 h\n", []string{"-t", ""}, "1 a\n8 h\n"},
		{"nul separator", "a\x001\n", "a\x002\n", []string{"-t", `\0`}, "a\x001\x002\n"},
		{"unicode separator", "a§1\n", "a§2\n", []string{"-t", "§"}, "a§1§2\n"},
		{"emoji separator", "a🗿b\n", "u🗿b\n", []string{"-t", "🗿", "-j", "2"}, "b🗿a🗿u\n"},
		{"blanks", " a  ,,,b\n", " a  ,c ", nil, "a ,,,b ,c \n"},
		{"blanks comma", " a  ,,,b\n", " a  ,c ", []string{"-t", ","}, " a  ,,,b,c \n"},
		{"ignore case", "B x\nb y\n", "b z\n", []string{"-i"}, "B x z\nb y z\n"},
		{"ignore case long", "B x\n", "b z\n", []string{"--ignore-case"}, "B x z\n"},
		{"fold order", "svcabc mallory\nsvc_backup mallory\n", "SVC_BACKUP\n", []string{"-i", "-v", "1"}, "svcabc mallory\n"},
		{"cartesian", "a 1\na 2\n", "a x\na y\n", nil, "a 1 x\na 1 y\na 2 x\na 2 y\n"},
		{"header", "key left\na 1\n", "id right\na 2\n", []string{"--header"}, "key left right\na 1 2\n"},
		{"header auto", "key left extra\na 1\n", "id right\na 2\n", []string{"--header", "-o", "auto"}, "key left extra right\na 1  2\n"},
		{"header alone", "key left\n", "", []string{"--header"}, "key left\n"},
		{"zero records", "a 1\x00b 2\x00", "a x\x00b y\x00", []string{"-z"}, "a 1 x\x00b 2 y\x00"},
		{"zero records long", "a 1\x00", "a x\x00", []string{"--zero-terminated"}, "a 1 x\x00"},
		{"missing key comma", "x,\ny\nbob,1002\n", "x,\ny\nbob,1002\n", []string{"-t", ",", "-j", "2"}, ",x,x\n,x,y\n,y,x\n,y,y\n1002,bob,bob\n"},
		{"missing key blanks", "a \nb\nbob 1002\n", "a \nb\nbob 1002\n", []string{"-j", "2"}, " a a\n a b\n b a\n b b\n1002 bob bob\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "a", tt.a)
			writeFile(t, dir, "b", tt.b)
			args := append(append([]string{}, tt.opts...), "a", "b")
			r := runBuiltinIn(t, dir, "join", args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tt.out)
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestJoinErrors(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"missing", nil, "join: missing operand\nTry 'join --help' for more information.\n"},
		{"one", []string{"x"}, "join: missing operand after 'x'\nTry 'join --help' for more information.\n"},
		{"extra", []string{"x", "y", "z"}, "join: extra operand 'z'\nTry 'join --help' for more information.\n"},
		{"stdin twice", []string{"-", "-"}, "join: both files cannot be standard input\n"},
		{"incompatible 1", []string{"-j", "3", "-1", "5"}, "join: incompatible join fields 3, 5\n"},
		{"incompatible 2", []string{"-j", "3", "-2", "5"}, "join: incompatible join fields 3, 5\n"},
		{"empty format", []string{"-o", ""}, "join: invalid file number in field spec: ''\n"},
		{"bad format", []string{"-o", "1.2,2.x"}, "join: invalid field number: 'x'\n"},
		{"conflicting filler", []string{"-e", "a", "-e", "b"}, "join: conflicting empty-field replacement strings\n"},
		{"multi tab", []string{"-t", "ab"}, "join: multi-character tab 'ab'\n"},
		{"invalid utf8 tab", []string{"-t", "\xff\xff"}, "join: non-UTF-8 multi-byte tab\n"},
		{"bad a", []string{"-a", "3"}, "join: invalid file number: '3'\n"},
		{"bad v", []string{"-v", "3"}, "join: invalid file number: '3'\n"},
		{"bad j", []string{"-j", "0"}, "join: invalid field number: '0'\n"},
		{"bad 1", []string{"-1", "-1"}, "join: invalid field number: '-1'\n"},
		{"bad 2", []string{"-2", "0"}, "join: invalid field number: '0'\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltin(t, "join", tt.args...)
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			if r.stderr != tt.stderr {
				t.Errorf("stderr = %q, want %q", r.stderr, tt.stderr)
			}
		})
	}
	for _, args := range [][]string{{"-n"}, {"--definitely-invalid"}, {"-t"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			r := runBuiltin(t, "join", args...)
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			if r.stderr == "" {
				t.Error("missing diagnostic")
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "a", "")
		r := runBuiltinIn(t, dir, "join", "a", "missing-file")
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "missing-file")
		if !strings.Contains(strings.ToLower(r.stderr), "no such file") {
			t.Errorf("stderr = %q", r.stderr)
		}
	})
	t.Run("help", func(t *testing.T) {
		// The shared helper leaves CmdName empty; production dispatch supplies it.
		cmd, ok := Lookup("join")
		if !ok {
			t.Fatal("join not registered")
		}
		var out, err bytes.Buffer
		code := cmd.Run(&Context{Dir: t.TempDir(), CmdName: "join", Args: []string{"--help"}, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &err})
		r := runResult{code: code, stdout: out.String(), stderr: err.String()}
		assertExitCode(t, r, 0)
		for _, s := range []string{"join", "--header", "--check-order", "--ignore-case"} {
			assertStdoutContains(t, r, s)
		}
		if r.stderr != "" {
			t.Errorf("stderr = %q", r.stderr)
		}
	})
}

func TestJoinStdin(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "data", "a x\n")
	for _, args := range [][]string{{"-", p}, {p, "-"}} {
		r := runBuiltinStdin(t, "join", "a x\n", args...)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "a x x\n")
		if r.stderr != "" {
			t.Errorf("stderr = %q", r.stderr)
		}
	}
}

func TestJoinOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "2 c 1 cd\n3 d 2 de\n5 e 3 ef\n7 f 4 fg\n11 g 5 gh\n")
	writeFile(t, dir, "b", "1\n2\n5\n3\n6\n4\n")
	tests := []struct {
		name     string
		opts     []string
		code     int
		out, err string
	}{
		{"default", nil, 1, "2 c 1 cd\n5 e 3 ef\n", "join: b:4: is not sorted: 3\njoin: a:5: is not sorted: 11 g 5 gh\njoin: input is not in sorted order\n"},
		{"check", []string{"--check-order"}, 1, "2 c 1 cd\n", "join: b:4: is not sorted: 3\n"},
		{"nocheck", []string{"--nocheck-order"}, 0, "2 c 1 cd\n5 e 3 ef\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append(append([]string{}, tt.opts...), "a", "b")
			r := runBuiltinIn(t, dir, "join", args...)
			assertExitCode(t, r, tt.code)
			assertStdout(t, r, tt.out)
			if r.stderr != tt.err {
				t.Errorf("stderr = %q, want %q", r.stderr, tt.err)
			}
		})
	}
}
