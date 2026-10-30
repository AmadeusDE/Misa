package builtins

import (
	"strings"
	"testing"
)

func hexRun(input string, args ...string) (string, string, int) {
	var out, err strings.Builder
	c := &Context{Args: args, CmdName: "hexdump", Stdin: strings.NewReader(input), Stdout: &out, Stderr: &err}
	code := runHexdump(c)
	return out.String(), err.String(), code
}
func TestHexdumpCore(t *testing.T) {
	cases := []struct {
		in   string
		args []string
		want string
	}{
		{"ABC", nil, "0000000 4241 0043\n0000003\n"},
		{"ABC", []string{"-C"}, "00000000  41 42 43                                          |ABC|\n00000003\n"},
		{"ABC", []string{"-e", `"%04_ax " 3/1 "%02x " "\n"`, "-e", `"%04_Ax\n"`}, "0000 41 42 43\n0003\n"},
		{"abcd", []string{"-s", "1", "-n", "2", "-e", `2/1 "%c"`}, "bc"},
	}
	for _, tc := range cases {
		out, err, code := hexRun(tc.in, tc.args...)
		if code != 0 || out != tc.want {
			t.Errorf("%v: got %q, stderr %q, code %d; want %q", tc.args, out, err, code, tc.want)
		}
	}
}
func TestHexdumpMalformed(t *testing.T) {
	for _, s := range []string{`1/3 "%x"`, `"%q"`, `"%_ax`, `0/1 "%x"`} {
		_, _, code := hexRun("abc", "-e", s)
		if code == 0 {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestHexdumpSqueezeAndAlias(t *testing.T) {
	input := strings.Repeat("a", 48)
	out, err, code := hexRun(input, "-C")
	if code != 0 || err != "" || out != "00000000  61 61 61 61 61 61 61 61  61 61 61 61 61 61 61 61  |aaaaaaaaaaaaaaaa|\n*\n00000030\n" {
		t.Fatalf("output %q err %q code %d", out, err, code)
	}
	out, _, code = hexRun(input, "-C", "-v")
	if code != 0 || strings.Contains(out, "*\n") || strings.Count(out, "|aaaaaaaaaaaaaaaa|") != 3 {
		t.Fatalf("verbose output %q", out)
	}
	var stdout, stderr strings.Builder
	code = runHexdump(&Context{CmdName: "hd", Stdin: strings.NewReader("ABC"), Stdout: &stdout, Stderr: &stderr})
	if code != 0 || !strings.Contains(stdout.String(), "|ABC|") {
		t.Fatalf("hd output %q err %q", stdout.String(), stderr.String())
	}
}
func TestHexdumpFilesAndFormatFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one", "ab")
	writeFile(t, dir, "two", "cd")
	writeFile(t, dir, "fmt", "# comment\n2/1 \"%c\"\n")
	var out, err strings.Builder
	ctx := &Context{Dir: dir, Args: []string{"-f", "fmt", "one", "missing", "two"}, Stdout: &out, Stderr: &err, Stdin: strings.NewReader("")}
	if code := runHexdump(ctx); code != 1 || out.String() != "abcd" || !strings.Contains(err.String(), "hexdump: missing:") {
		t.Fatalf("code %d out %q err %q", code, out.String(), err.String())
	}
}
