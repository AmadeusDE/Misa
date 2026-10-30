package builtins

import "testing"

func TestSedScripts(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		args  []string
		code  int
		want  string
	}{
		{"cat", "a\nb\n", []string{""}, 0, "a\nb\n"},
		{"keeps missing newline", "a\nb", []string{""}, 0, "a\nb"},
		{"quiet", "a\nb\n", []string{"-n", "2p"}, 0, "b\n"},
		{"hash n quiet", "a\nb\n", []string{"#n\n2p"}, 0, "b\n"},
		{"last line", "a\nb\nc", []string{"-n", "$p"}, 0, "c"},
		{"range", "1\n2\n3\n4\n", []string{"-n", "2,3p"}, 0, "2\n3\n"},
		{"backwards range is one line", "1\n2\n3\n", []string{"-n", "2,1p"}, 0, "2\n"},
		{"regex range", "a\nstart\nb\nend\nc\n", []string{"-n", "/start/,/end/p"}, 0, "start\nb\nend\n"},
		{"regex end not tried on start line", "x\nx\ny\n", []string{"-n", "/x/,/x/p"}, 0, "x\nx\n"},
		{"zero address", "x\nx\ny\n", []string{"-n", "0,/x/p"}, 0, "x\n"},
		{"plus", "1\n2\n3\n4\n", []string{"-n", "/2/,+1p"}, 0, "2\n3\n"},
		{"multiple", "1\n2\n3\n4\n5\n", []string{"-n", "2,~4p"}, 0, "2\n3\n4\n"},
		{"step", "1\n2\n3\n4\n5\n", []string{"-n", "1~2p"}, 0, "1\n3\n5\n"},
		{"negate", "a\nb\n", []string{"-n", "1!p"}, 0, "b\n"},
		{"block", "a\nb\nc\n", []string{"-n", "2{p;p}"}, 0, "b\nb\n"},
		{"negated block", "a\nb\n", []string{"-n", "1!{p}"}, 0, "b\n"},
		{"nested blocks", "a\nb\nc\n", []string{"-n", "1,2{/b/{p}}"}, 0, "b\n"},
		{"delete", "a\nb\nc\n", []string{"2d"}, 0, "a\nc\n"},
		{"quit", "a\nb\n", []string{"q"}, 0, "a\n"},
		{"quit code", "a\nb\n", []string{"q5"}, 5, "a\n"},
		{"quit silently", "a\nb\n", []string{"2Q"}, 0, "a\n"},
		{"line number", "a\nb\n", []string{"-n", "$="}, 0, "2\n"},
		{"comment", "a\n", []string{"p # print"}, 0, "a\na\n"},
		{"substitute", "hello\n", []string{"s/l/L/"}, 0, "heLlo\n"},
		{"global", "hello\n", []string{"s/l/L/g"}, 0, "heLLo\n"},
		{"nth", "aaaa\n", []string{"s/a/b/3"}, 0, "aaba\n"},
		{"nth and global", "aaaa\n", []string{"s/a/b/2g"}, 0, "abbb\n"},
		{"ampersand", "ab\n", []string{"s/b/[&]/"}, 0, "a[b]\n"},
		{"escaped ampersand", "ab\n", []string{`s/b/\&/`}, 0, "a&\n"},
		{"groups", "john smith\n", []string{`s/\(.*\) \(.*\)/\2, \1/`}, 0, "smith, john\n"},
		{"ere groups", "john smith\n", []string{"-E", `s/(.*) (.*)/\2 \1/`}, 0, "smith john\n"},
		{"r is ere", "aab\n", []string{"-r", `s/a+/X/`}, 0, "Xb\n"},
		{"ignore case flag", "Hello\n", []string{"s/hello/bye/I"}, 0, "bye\n"},
		{"print flag", "a\nb\n", []string{"-n", "s/a/x/p"}, 0, "x\n"},
		{"newline in replacement", "ab\n", []string{`s/a/&\n/`}, 0, "a\nb\n"},
		{"other delimiter", "a/b\n", []string{"s|/|-|"}, 0, "a-b\n"},
		{"escaped delimiter", "a|b\n", []string{`s|a\|b|X|`}, 0, "X\n"},
		{"empty regex reuses last", "foo bar\n", []string{"/foo/s//baz/"}, 0, "baz bar\n"},
		{"case conversion", "hello world\n", []string{`s/\(hello\) \(w\)/\U\1\E \u\2/`}, 0, "HELLO World\n"},
		{"lower until end", "ABC\n", []string{`s/.*/\L&/`}, 0, "abc\n"},
		{"empty matches", "abc\n", []string{"s/x*/-/g"}, 0, "-a-b-c-\n"},
		{"null data", "a\x00b\x00", []string{"-z", "s/^/>/"}, 0, ">a\x00>b\x00"},
		{"several -e", "a\n", []string{"-e", "s/a/b/", "-e", "s/b/c/"}, 0, "c\n"},
		{"semicolons and spaces", "a\n", []string{" 1p ; s/a/b/ ; p "}, 0, "a\nb\nb\n"},
		{"dollar inside range", "1\n2\n3\n", []string{"-n", "2,$p"}, 0, "2\n3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "sed", tt.stdin, tt.args...)
			assertExitCode(t, r, tt.code)
			assertStdout(t, r, tt.want)
		})
	}
}

func TestSedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one", "1\n2\n")
	writeFile(t, dir, "two", "3\n4\n")
	writeFile(t, dir, "script", "s/1/one/\n$s/4/four/\n")
	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"continuous", []string{"-n", "$p", "one", "two"}, 0, "4\n"},
		{"separate", []string{"-sn", "1p", "one", "two"}, 0, "1\n3\n"},
		{"separate last", []string{"-sn", "$p", "one", "two"}, 0, "2\n4\n"},
		{"script file", []string{"-f", "script", "one", "two"}, 0, "one\n2\n3\nfour\n"},
		{"missing input", []string{"p", "nope", "one"}, 2, "1\n1\n2\n2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltinIn(t, dir, "sed", tt.args...)
			assertExitCode(t, r, tt.code)
			assertStdout(t, r, tt.want)
		})
	}
}

func TestSedErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"s/a/b"}, "sed: -e expression #1, char 6: unterminated `s' command"},
		{[]string{"k"}, "sed: -e expression #1, char 1: unknown command: `k'"},
		{[]string{"s/a/\\1/"}, "invalid reference \\1 on `s' command's RHS"},
		{[]string{"-e", "p", "-e", "s/a/b/gg"}, "sed: -e expression #2, char"},
		{[]string{"{p"}, "unmatched `{'"},
		{[]string{"}"}, "unexpected `}'"},
		{[]string{"1,2q"}, "command only uses one address"},
		{[]string{"s/[/x/"}, "unterminated `s' command"},
		{[]string{"/a/M p"}, "the M modifier is not supported"},
		{[]string{"-i", "p"}, "sed: -i is not supported yet"},
		{[]string{"y/ab/c/"}, "`y' command"},
		{[]string{}, "Usage: sed"},
	}
	for _, tt := range tests {
		r := runBuiltinStdin(t, "sed", "a\n", tt.args...)
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, tt.want)
	}
}
