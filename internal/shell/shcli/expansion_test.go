package shcli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestS08ExpansionCLI(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.txt", "a.txt", ".hidden.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// The separate count retains the distinction between no arguments and one empty argument.
	const prelude = `argv() { printf '%s|' "$#"; for arg do printf '<%s>' "$arg"; done; }; `
	tests := []struct {
		name, src, want, stderr string
		args                    []string
		status                  int
	}{
		{name: "empty quoted", src: `x=; argv "$x" ''`, want: `2|<><>`},
		{name: "empty unquoted", src: `x=; argv $x`, want: `0|`},
		{name: "quote concatenation", src: `x='a b'; argv pre"$x"post`, want: `1|<prea bpost>`},
		{name: "default splitting", src: `x=' a  b '; argv $x`, want: `2|<a><b>`},
		{name: "IFS delimiters", src: `IFS=:; x='a::b'; argv $x`, want: `3|<a><><b>`},
		{name: "empty IFS", src: `IFS=; x='a b'; argv $x`, want: `1|<a b>`},
		{name: "quoted positional", src: `argv "$@"`, args: []string{"a b", "", "c"}, want: `3|<a b><><c>`},
		{name: "zero quoted positional", src: `argv "$@"`, want: `0|`},
		{name: "glob", src: `argv *.txt 'no*' .*.txt`, want: `4|<a.txt><b.txt><no*><.hidden.txt>`},
		{name: "expanded pattern", src: `p='*.txt'; argv $p "$p" $(printf '%s' '*.txt')`, want: `5|<a.txt><b.txt><*.txt><a.txt><b.txt>`},
		{name: "escaped glob", src: `argv \*.txt`, want: `1|<*.txt>`},
		{name: "noglob", src: `set -f; argv *.txt`, want: `1|<*.txt>`},
		{name: "parameter defaults", src: `x=; argv "${x:-default}" "${x-default}" "${missing:-fallback}"`, want: `3|<default><><fallback>`},
		{name: "parameter removal", src: `x=abcabc; argv "${x#a*c}" "${x%%b*}"`, want: `2|<abc><a>`},
		{name: "arithmetic", src: `x=4; argv "$((x * 2 + 1))"`, want: `1|<9>`},
		{name: "command substitution", src: `argv "$(printf 'a\nb\n\n')"`, want: "1|<a\nb>"},
		{name: "substitution failure status", src: `x=$(no_such_command); printf '%s' "$?"`, want: `127`, stderr: "no_such_command: exec: \"no_such_command\": executable file not found in $PATH\n"},
		{name: "parameter error", src: `argv "${missing:?required}"; printf unreachable`, status: 1, stderr: "missing: required\n"},
		{name: "arithmetic error", src: `argv "$((1 / 0))"; printf unreachable`, status: 1, stderr: "division by zero\n"},
		{name: "nounset error", src: `set -u; argv "$missing"; printf unreachable`, status: 1, stderr: "missing: unbound variable\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-c", prelude + tt.src, "test-shell"}, tt.args...)
			status, out, stderr := run(t, "sh", args, Options{Dir: dir, Env: []string{"PATH=", "HOME=" + dir}})
			if status != tt.status || out != tt.want || stderr != tt.stderr {
				t.Errorf("status %d stdout %q stderr %q; want status %d stdout %q stderr %q", status, out, stderr, tt.status, tt.want, tt.stderr)
			}
		})
	}
}
