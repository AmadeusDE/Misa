package builtins

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// pwgenConstReader always returns the same byte.
type pwgenConstReader struct{ b byte }

func (r pwgenConstReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.b
	}
	return len(p), nil
}

// pwgenCycleReader returns the bytes 0..255 over and over.
type pwgenCycleReader struct{ next byte }

func (r *pwgenCycleReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.next
		r.next++
	}
	return len(p), nil
}

// pwgenErrReader fails every read.
type pwgenErrReader struct{}

func (pwgenErrReader) Read(p []byte) (int, error) { return 0, errors.New("randomness is on fire") }

// pwgenShortReader returns n filler bytes and then fails.
type pwgenShortReader struct{ left int }

func (r *pwgenShortReader) Read(p []byte) (int, error) {
	if r.left <= 0 {
		return 0, errors.New("randomness is on fire")
	}
	n := len(p)
	if n > r.left {
		n = r.left
	}
	for i := 0; i < n; i++ {
		p[i] = byte('a')
	}
	r.left -= n
	return n, nil
}

// withPwgenRand swaps the package-level randomness source for the test.
func withPwgenRand(t *testing.T, r io.Reader) {
	t.Helper()
	old := pwgenRand
	pwgenRand = r
	t.Cleanup(func() { pwgenRand = old })
}

// runPwgenCtx runs pwgen against a context built by the test (for Width/tty-less
// layout cases that runBuiltin cannot express).
func runPwgenCtx(t *testing.T, width, height int, args ...string) runResult {
	t.Helper()
	cmd, ok := Lookup("pwgen")
	if !ok {
		t.Fatal("builtin \"pwgen\" not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Dir:    t.TempDir(),
		Args:   args,
		Width:  width,
		Height: height,
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	}
	code := cmd.Run(ctx)
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func isAlnumByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func TestPwgenDefault(t *testing.T) {
	withPwgenRand(t, &pwgenCycleReader{})

	r := runBuiltin(t, "pwgen")
	assertExitCode(t, r, 0)

	if got := strings.Count(r.stdout, "\n"); got != 1 {
		t.Fatalf("stdout = %q, want a single line", r.stdout)
	}
	if !strings.HasSuffix(r.stdout, "\n") {
		t.Fatalf("stdout = %q, want a trailing newline", r.stdout)
	}
	line := strings.TrimSuffix(r.stdout, "\n")
	if len(line) != 8 {
		t.Fatalf("password %q has length %d, want 8", line, len(line))
	}
	for i := 0; i < len(line); i++ {
		if !isAlnumByte(line[i]) {
			t.Fatalf("password %q contains non-alphanumeric byte %q", line, line[i])
		}
	}
}

func TestPwgenDeterministic(t *testing.T) {
	seq := []byte{7, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62}
	first := func() string {
		withPwgenRand(t, bytes.NewReader(seq))
		r := runBuiltin(t, "pwgen", "8", "2")
		assertExitCode(t, r, 0)
		return r.stdout
	}()
	second := func() string {
		withPwgenRand(t, bytes.NewReader(seq))
		r := runBuiltin(t, "pwgen", "8", "2")
		assertExitCode(t, r, 0)
		return r.stdout
	}()
	if first != second {
		t.Fatalf("same random bytes produced %q and %q", first, second)
	}
	if lines := strings.Split(strings.TrimSuffix(first, "\n"), "\n"); len(lines) != 2 {
		t.Fatalf("stdout = %q, want two lines", first)
	}
}

func TestPwgenConstantReader(t *testing.T) {
	// Byte 0 is index 0 of the default [0-9A-Za-z] set, so every character is '0'.
	withPwgenRand(t, pwgenConstReader{0})
	r := runBuiltin(t, "pwgen")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "00000000\n")

	// With -y, index 0 is '!'.
	withPwgenRand(t, pwgenConstReader{0})
	r = runBuiltin(t, "pwgen", "-y")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "!!!!!!!!\n")
}

func TestPwgenCharacterSets(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		reject  string
		wantAny string
	}{
		{name: "no numerals", args: []string{"-0", "200"}, reject: "0123456789"},
		{name: "no capitals", args: []string{"-A", "200"}, reject: "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{name: "ambiguous", args: []string{"-B", "200"}, reject: "0O1lI8B5S2ZD'`.,"},
		{name: "no vowels", args: []string{"-v", "200"}, reject: "aeiouAEIOU"},
		{name: "remove chars", args: []string{"-r", "abcXYZ019", "200"}, reject: "abcXYZ019"},
		{name: "numerals no-op", args: []string{"-n", "200"}, wantAny: "0123456789"},
		{name: "capitalize no-op", args: []string{"-c", "200"}, wantAny: "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{name: "secure no-op", args: []string{"-s", "200"}, wantAny: "0123456789"},
		{name: "symbols", args: []string{"-y", "200"}, wantAny: "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withPwgenRand(t, &pwgenCycleReader{})
			r := runBuiltin(t, "pwgen", tc.args...)
			assertExitCode(t, r, 0)
			body := strings.TrimSuffix(r.stdout, "\n")
			if len(body) != 200 {
				t.Fatalf("got %d characters in %q, want 200", len(body), body)
			}
			for i := 0; i < len(body); i++ {
				if strings.IndexByte(tc.reject, body[i]) >= 0 {
					t.Fatalf("character %q at %d is in the rejected set %q", body[i], i, tc.reject)
				}
			}
			if tc.wantAny != "" {
				found := false
				for i := 0; i < len(body); i++ {
					if strings.IndexByte(tc.wantAny, body[i]) >= 0 {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%q contains none of %q", body, tc.wantAny)
				}
			}
		})
	}
}

func TestPwgenSymbolsOnly(t *testing.T) {
	// -y keeps the whole printable range, but the default set has no punctuation.
	withPwgenRand(t, &pwgenCycleReader{})
	r := runBuiltin(t, "pwgen", "200")
	assertExitCode(t, r, 0)
	for i := 0; i < len(r.stdout)-1; i++ {
		if !isAlnumByte(r.stdout[i]) {
			t.Fatalf("default output %q contains punctuation %q", r.stdout, r.stdout[i])
		}
	}
}

func TestPwgenLengthCount(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		lines  int
		length int
	}{
		{name: "length and count", args: []string{"5", "3"}, lines: 3, length: 5},
		{name: "default count", args: []string{"12"}, lines: 1, length: 12},
		{name: "max length", args: []string{"4096", "1"}, lines: 1, length: 4096},
		{name: "count zero", args: []string{"4", "0"}, lines: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withPwgenRand(t, &pwgenCycleReader{})
			r := runBuiltin(t, "pwgen", tc.args...)
			assertExitCode(t, r, 0)
			if tc.lines == 0 {
				assertStdout(t, r, "")
				return
			}
			lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
			if len(lines) != tc.lines {
				t.Fatalf("stdout = %q, want %d lines", r.stdout, tc.lines)
			}
			for _, line := range lines {
				if len(line) != tc.length {
					t.Fatalf("line %q has length %d, want %d", line, len(line), tc.length)
				}
			}
		})
	}
}

func TestPwgenColumns(t *testing.T) {
	// The cycle reader yields 0,1,2,... so the first passwords are 0123, 4567,
	// 89AB, CDEF, GHIJ. At width 20 a fourth password on a row would not fit.
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "columns wrap",
			args: []string{"-C", "4", "5"},
			want: "0123 4567 89AB CDEF\nGHIJ\n",
		},
		{
			name: "one per line",
			args: []string{"-1", "4", "3"},
			want: "0123\n4567\n89AB\n",
		},
		{
			name: "columns exact fit",
			args: []string{"-C", "5", "3"},
			want: "01234 56789 ABCDE\n",
		},
		{
			name: "columns single",
			args: []string{"-C", "4", "1"},
			want: "0123\n",
		},
		{
			// A password as wide as the screen ends its own line every time.
			name: "columns full width",
			args: []string{"-C", "20", "2"},
			want: "0123456789ABCDEFGHIJ\nKLMNOPQRSTUVWXYZabcd\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withPwgenRand(t, &pwgenCycleReader{})
			r := runPwgenCtx(t, 20, 24, tc.args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tc.want)
		})
	}
}

func TestPwgenErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "capitalize and no-capitalize", args: []string{"-c", "-A"}, want: "pwgen: -c and -A are mutually exclusive\n"},
		{name: "numerals and no-numerals", args: []string{"-n", "-0"}, want: "pwgen: -n and -0 are mutually exclusive\n"},
		{name: "columns and one per line", args: []string{"-C", "-1"}, want: "pwgen: -C and -1 are mutually exclusive\n"},
		{name: "unknown option", args: []string{"-z"}, want: "pwgen: unknown option -z\n"},
		{name: "length zero", args: []string{"0"}, want: "pwgen: bad length\n"},
		{name: "length not a number", args: []string{"abc"}, want: "pwgen: bad length\n"},
		{name: "length too large", args: []string{"5000"}, want: "pwgen: bad length\n"},
		{name: "length negative", args: []string{"-5"}, want: "pwgen: unknown option -5\n"},
		{name: "count not a number", args: []string{"4", "abc"}, want: "pwgen: bad count\n"},
		{name: "too many arguments", args: []string{"1", "2", "3"}, want: "pwgen: too many arguments\n"},
		{
			name: "empty character set",
			args: []string{"-r", "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"},
			want: "pwgen: no characters available\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withPwgenRand(t, &pwgenCycleReader{})
			r := runBuiltin(t, "pwgen", tc.args...)
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")
			if r.stderr != tc.want {
				t.Errorf("stderr = %q, want %q", r.stderr, tc.want)
			}
		})
	}
}

func TestPwgenReadFailure(t *testing.T) {
	tests := []struct {
		name string
		r    io.Reader
	}{
		{name: "immediate error", r: pwgenErrReader{}},
		{name: "error after some bytes", r: &pwgenShortReader{left: 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withPwgenRand(t, tc.r)
			r := runBuiltin(t, "pwgen", "16", "4")
			assertExitCode(t, r, 1)
			if r.stderr != "pwgen: cannot read random bytes\n" {
				t.Errorf("stderr = %q, want %q", r.stderr, "pwgen: cannot read random bytes\n")
			}
		})
	}
}

func TestPwgenHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		r := runBuiltin(t, "pwgen", arg)
		assertExitCode(t, r, 0)
		assertStdoutContains(t, r, "usage: pwgen")
		assertStdoutContains(t, r, "-r CHARS")
	}
}

// TestPwgenUniform proves the character choice is rejection sampled instead of
// taken modulo 256: with a reader cycling 0..255 the accepted bytes of one cycle
// hit every character of the set exactly the same number of times.
func TestPwgenUniform(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		chars string
		count int
	}{
		{name: "all printable", args: []string{"-y", "188"}, chars: pwgenPrintableRange(), count: 2},
		{name: "alphanumeric", args: []string{"248"}, chars: "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", count: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withPwgenRand(t, &pwgenCycleReader{})
			r := runBuiltin(t, "pwgen", tc.args...)
			assertExitCode(t, r, 0)
			body := strings.TrimSuffix(r.stdout, "\n")
			if len(body) != len(tc.chars)*tc.count {
				t.Fatalf("got %d characters, want %d", len(body), len(tc.chars)*tc.count)
			}
			seen := map[byte]int{}
			for i := 0; i < len(body); i++ {
				seen[body[i]]++
			}
			if len(seen) != len(tc.chars) {
				t.Fatalf("output uses %d distinct characters, want %d: %q", len(seen), len(tc.chars), body)
			}
			for i := 0; i < len(tc.chars); i++ {
				if got := seen[tc.chars[i]]; got != tc.count {
					t.Errorf("character %q appeared %d times, want %d", tc.chars[i], got, tc.count)
				}
			}
		})
	}
}

func pwgenPrintableRange() string {
	var b strings.Builder
	for c := byte(33); c <= 126; c++ {
		b.WriteByte(c)
	}
	return b.String()
}
