package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

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
		{[]string{"y/ab/c/"}, "`y' command"},
		{[]string{}, "Usage: sed"},
	}
	for _, tt := range tests {
		r := runBuiltinStdin(t, "sed", "a\n", tt.args...)
		assertExitCode(t, r, 1)
		assertStderrContains(t, r, tt.want)
	}
}

func TestSedCommands(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		args  []string
		want  string
	}{
		{"append", "a\nb\n", []string{"1a x"}, "a\nx\nb\n"},
		{"append backslash form", "a\n", []string{"a\\\nx\\\ny"}, "a\nx\ny\n"},
		{"append same-line backslash", "a\n", []string{`a\tx`}, "a\ntx\n"},
		{"append escapes", "a\n", []string{`a x\ty\nz`}, "a\nx\ty\nz\n"},
		{"insert", "a\nb\n", []string{"2i x"}, "a\nx\nb\n"},
		{"change", "a\nb\n", []string{"1c x"}, "x\nb\n"},
		{"change range prints once", "1\n2\n3\n4\n", []string{"2,3c x"}, "1\nx\n4\n"},
		{"change negated", "1\n2\n", []string{"1!c x"}, "1\nx\n"},
		{"append after missing newline", "a", []string{"a x"}, "a\nx\n"},
		{"empty append adds missing newline", "a", []string{"a\\"}, "a\n"},
		{"next", "1\n2\n3\n", []string{"-n", "n;p"}, "2\n"},
		{"next prints", "1\n2\n3\n4\n", []string{"n;d"}, "1\n3\n"},
		{"next at end keeps autoprint", "1\n2\n3\n", []string{"n;d"}, "1\n3\n"},
		{"join", "1\n2\n3\n", []string{"N;s/\\n/+/"}, "1+2\n3\n"},
		{"join at end prints", "1\n2\n3\n", []string{"$!N;s/\\n/+/"}, "1+2\n3\n"},
		{"join all", "1\n2\n3\n", []string{":a;N;$!ba;s/\\n/,/g"}, "1,2,3\n"},
		{"delete first line restarts", "1\n2\n3\n", []string{"-n", "$!N;P;D"}, "1\n2\n3\n"},
		{"print first line", "1\n2\n", []string{"-n", "N;P"}, "1\n"},
		{"hold and get", "1\n2\n", []string{"-n", "1h;2{g;p}"}, "1\n"},
		{"reverse lines", "1\n2\n3\n", []string{"-n", "1!G;h;$p"}, "3\n2\n1\n"},
		{"exchange", "1\n2\n", []string{"x"}, "\n1\n"},
		{"append hold", "1\n2\n", []string{"-n", "H;${x;p}"}, "\n1\n2\n"},
		{"zap", "abc\n", []string{"z;s/^$/empty/"}, "empty\n"},
		{"branch to label", "a\nb\n", []string{"-n", "/a/bx;p;:x"}, "b\n"},
		{"branch to end", "a\nb\n", []string{"/a/b;s/./X/"}, "a\nX\n"},
		{"test branch", "aa\nb\n", []string{":x;s/a/c/;tx"}, "cc\nb\n"},
		{"test resets per line", "a\nb\n", []string{"-n", "s/a/A/;ty;p;d;:y;s/$/!/p"}, "A!\nb\n"},
		{"test not", "a\nb\n", []string{"s/a/A/;Tz;s/$/!/;:z"}, "A!\nb\n"},
		{"transliterate", "hello\n", []string{"y/elo/ELO/"}, "hELLO\n"},
		{"transliterate utf8", "aé\n", []string{"y/aé/éa/"}, "éa\n"},
		{"transliterate newline", "a\nb\n", []string{"N;y/\\n/ /"}, "a b\n"},
		{"list", "a\tb\x01é\\\n", []string{"-n", "l"}, "a\\tb\\001\\303\\251\\\\$\n"},
		{"list wraps", "abcdefghij\n", []string{"-n", "l 5"}, "abcd\\\nefgh\\\nij$\n"},
		{"list no wrap", "abcdefghij\n", []string{"-n", "l 0"}, "abcdefghij$\n"},
		{"list option width", "abcdefghij\n", []string{"-l", "5", "-n", "l"}, "abcd\\\nefgh\\\nij$\n"},
		{"filename", "a\n", []string{"F"}, "-\na\n"},
		{"quit flushes append", "a\nb\n", []string{"a x\nq"}, "a\nx\n"},
		{"write stdout", "a\n", []string{"w /dev/stdout"}, "a\na\n"},
		{"substitute write stdout", "a\nb\n", []string{"-n", "s/a/x/w /dev/stdout"}, "x\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "sed", tt.stdin, tt.args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tt.want)
		})
	}
}

func TestSedFileCommands(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "in", "1\n2\n3\n")
	writeFile(t, dir, "extra", "x\ny\n")
	tests := []struct {
		name string
		args []string
		want string
		file string
		body string
	}{
		{"read file", []string{"2r extra", "in"}, "1\n2\nx\ny\n3\n", "", ""},
		{"read missing file", []string{"2r nope", "in"}, "1\n2\n3\n", "", ""},
		{"read lines", []string{"R extra", "in"}, "1\nx\n2\ny\n3\n", "", ""},
		{"write", []string{"-n", "2w out", "in"}, "", "out", "2\n"},
		{"write shared handle", []string{"-n", "-e", "1w out", "-e", "3w out", "in"}, "", "out", "1\n3\n"},
		{"write created even unused", []string{"-n", "9w out", "in"}, "", "out", ""},
		{"write first line", []string{"-n", "N;W out", "in"}, "", "out", "1\n"},
		{"substitute write", []string{"-n", "s/2/two/w out", "in"}, "", "out", "two\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltinIn(t, dir, "sed", tt.args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tt.want)
			if tt.file != "" {
				data, err := os.ReadFile(filepath.Join(dir, tt.file))
				if err != nil || string(data) != tt.body {
					t.Errorf("%s = %q, %v; want %q", tt.file, data, err, tt.body)
				}
				os.Remove(filepath.Join(dir, tt.file))
			}
		})
	}
}
