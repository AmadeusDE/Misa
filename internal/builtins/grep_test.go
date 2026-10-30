package builtins

import (
	"fmt"
	"strings"
	"testing"
)

func TestGrepSelection(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		args  []string
		code  int
		want  string
	}{
		{"basic", "one\ntwo\nthree\n", []string{"t"}, 0, "two\nthree\n"},
		{"no match", "one\n", []string{"x"}, 1, ""},
		{"bre is default", "a+b\naab\n", []string{"a+b"}, 0, "a+b\n"},
		{"ere", "a+b\naab\n", []string{"-E", "a+b"}, 0, "aab\n"},
		{"fixed", "a.b\naxb\n", []string{"-F", "a.b"}, 0, "a.b\n"},
		{"last syntax flag wins", "a+b\naab\n", []string{"-E", "-G", "a+b"}, 0, "a+b\n"},
		{"ignore case", "ABC\nabc\nxyz\n", []string{"-i", "abc"}, 0, "ABC\nabc\n"},
		{"no ignore case", "ABC\nabc\n", []string{"-i", "--no-ignore-case", "abc"}, 0, "abc\n"},
		{"invert", "a\nb\n", []string{"-v", "a"}, 0, "b\n"},
		{"invert all match", "a\n", []string{"-v", "a"}, 1, ""},
		{"word", "foo\nfoobar\nbar foo.\n", []string{"-w", "foo"}, 0, "foo\nbar foo.\n"},
		{"line", "foo\nfoo bar\n", []string{"-x", "foo"}, 0, "foo\n"},
		{"newline separates patterns", "a\nb\nc\n", []string{"a\nc"}, 0, "a\nc\n"},
		{"several -e", "a\nb\nc\n", []string{"-e", "a", "-e", "b"}, 0, "a\nb\n"},
		{"empty pattern", "a\n\n", []string{""}, 0, "a\n\n"},
		{"backref", "abab\nabba\n", []string{`\(ab\)\1`}, 0, "abab\n"},
		{"gnu alternation in bre", "cat\ndog\nemu\n", []string{`cat\|dog`}, 0, "cat\ndog\n"},
		{"count", "a\nb\na\n", []string{"-c", "a"}, 0, "2\n"},
		{"count invert", "a\nb\na\n", []string{"-cv", "a"}, 0, "1\n"},
		{"count zero", "b\n", []string{"-c", "a"}, 1, "0\n"},
		{"max count", "a1\na2\na3\n", []string{"-m", "2", "a"}, 0, "a1\na2\n"},
		{"max count zero", "a\n", []string{"-m", "0", "a"}, 1, ""},
		{"max count with count", "a\na\na\n", []string{"-c", "-m2", "a"}, 0, "2\n"},
		{"line numbers", "a\nb\na\n", []string{"-n", "a"}, 0, "1:a\n3:a\n"},
		{"byte offsets", "ab\ncd\n", []string{"-b", "c"}, 0, "3:cd\n"},
		{"only matching", "abcabc\n", []string{"-o", "b."}, 0, "bc\nbc\n"},
		{"only matching offsets", "xab ab\n", []string{"-ob", "ab"}, 0, "1:ab\n4:ab\n"},
		{"only matching invert prints nothing", "a\nb\n", []string{"-ov", "a"}, 0, ""},
		{"null data", "a\x00b\x00", []string{"-z", "b"}, 0, "b\x00"},
		{"null data multiline record", "x\ny\x00z", []string{"-z", "^y"}, 1, ""},
		{"label", "a\n", []string{"-H", "--label=in", "a"}, 0, "in:a\n"},
		{"quiet", "a\n", []string{"-q", "a"}, 0, ""},
		{"missing final newline", "a", []string{"a"}, 0, "a\n"},
		{"after context", "m\n1\n2\n3\nm\n", []string{"-A1", "m"}, 0, "m\n1\n--\nm\n"},
		{"before context", "1\n2\nm\n", []string{"-B1", "m"}, 0, "2\nm\n"},
		{"context merges", "m\n1\nm\n", []string{"-C1", "m"}, 0, "m\n1\nm\n"},
		{"context marks", "1\nm\n2\n", []string{"-nC1", "m"}, 0, "1-1\n2:m\n3-2\n"},
		{"no group separator", "m\n1\n2\nm\n", []string{"-A0", "--no-group-separator", "m"}, 0, "m\nm\n"},
		{"custom group separator", "m\n1\n2\nm\n", []string{"-A0", "--group-separator=::", "m"}, 0, "m\n::\nm\n"},
		{"many literals", "alpha\nbeta\ngamma\n", []string{"-e", "et", "-e", "mm", "-e", "zz"}, 0, "beta\ngamma\n"},
		{"many literals whole line", "alpha\nbeta\n", []string{"-x", "-e", "beta", "-e", "alp"}, 0, "beta\n"},
		{"many literals only matching", "xabcd\n", []string{"-o", "-e", "ab", "-e", "abcd", "-e", "b"}, 0, "abcd\n"},
		{"color", "abcab\n", []string{"--color=always", "b"}, 0, "a\x1b[01;31m\x1b[Kb\x1b[m\x1b[Kca\x1b[01;31m\x1b[Kb\x1b[m\x1b[K\n"},
		{"color never", "ab\n", []string{"--color=never", "b"}, 0, "ab\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "grep", tt.stdin, tt.args...)
			assertExitCode(t, r, tt.code)
			assertStdout(t, r, tt.want)
		})
	}
}

func TestGrepFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "hit\nmiss\n")
	writeFile(t, dir, "b", "miss\n")
	writeFile(t, dir, "pats", "hit\nzzz\n")
	writeFile(t, dir, "empty", "")
	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"names with several files", []string{"hit", "a", "b"}, 0, "a:hit\n"},
		{"no filename", []string{"-h", "hit", "a", "b"}, 0, "hit\n"},
		{"files with matches", []string{"-l", "hit", "a", "b"}, 0, "a\n"},
		{"files without match", []string{"-L", "hit", "a", "b"}, 0, "b\n"},
		{"files without match none listed", []string{"-L", "i", "a", "b"}, 1, ""},
		{"null after name", []string{"-lZ", "hit", "a"}, 0, "a\x00"},
		{"null after name in lines", []string{"-HZ", "hit", "a"}, 0, "a\x00hit\n"},
		{"count per file", []string{"-c", "hit", "a", "b"}, 0, "a:1\nb:0\n"},
		{"pattern file", []string{"-f", "pats", "a"}, 0, "hit\n"},
		{"empty pattern file", []string{"-f", "empty", "a"}, 1, ""},
		{"missing file", []string{"hit", "a", "nope"}, 2, "a:hit\n"},
		{"missing file silent", []string{"-s", "hit", "nope"}, 2, ""},
		{"quiet match beats error", []string{"-q", "hit", "nope", "a"}, 0, ""},
		{"directory operand", []string{"hit", "."}, 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltinIn(t, dir, "grep", tt.args...)
			assertExitCode(t, r, tt.code)
			assertStdout(t, r, tt.want)
		})
	}
	r := runBuiltinIn(t, dir, "grep", "hit", "nope")
	assertStderrContains(t, r, "grep: nope: No such file or directory")
	r = runBuiltinIn(t, dir, "grep", "hit", ".")
	assertStderrContains(t, r, "grep: .: Is a directory")
}

func TestGrepErrors(t *testing.T) {
	tests := []struct {
		args   []string
		stderr string
	}{
		{[]string{"a["}, "grep: Unmatched [, [^, [:, [., or [="},
		{[]string{"-E", "(a"}, `grep: Unmatched ( or \(`},
		{[]string{}, "Usage: grep"},
		{[]string{"-r", "a"}, "grep: -r: recursive search is not supported yet"},
		{[]string{"-P", "a"}, "grep: -P: Perl regular expressions are not supported"},
		{[]string{"--color=sometimes", "a"}, "grep: invalid argument 'sometimes' for '--color'"},
		{[]string{"-f", "missing"}, "grep: missing: No such file or directory"},
	}
	for _, tt := range tests {
		r := runBuiltinStdin(t, "grep", "a\n", tt.args...)
		assertExitCode(t, r, 2)
		assertStderrContains(t, r, tt.stderr)
	}
}

func TestGrepAliases(t *testing.T) {
	r := runBuiltinStdin(t, "egrep", "aab\na+b\n", "a+b")
	assertStdout(t, r, "aab\n")
	r = runBuiltinStdin(t, "fgrep", "a.b\naxb\n", "a.b")
	assertStdout(t, r, "a.b\n")
}

func TestGrepManyLiteralPatternsAreFast(t *testing.T) {
	dir := t.TempDir()
	var pats, input strings.Builder
	for i := 0; i < 20000; i++ {
		fmt.Fprintf(&pats, "line-%06d-%s\n", i, strings.Repeat("x", 60))
		fmt.Fprintf(&input, "prefix line-%06d-%s suffix\n", i, strings.Repeat("x", 60))
	}
	writeFile(t, dir, "pats", pats.String())
	writeFile(t, dir, "input", input.String())
	r := runBuiltinIn(t, dir, "grep", "-c", "-f", "pats", "input")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "20000\n")
}
