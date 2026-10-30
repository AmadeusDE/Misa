// Test cases adapted from uutils coreutils od's tests/by-util/test_od.rs
// (commit fee61ff5d4f87e0f912a52e639942b5f7ad58663, MIT). The binary fixtures
// are generated here; no opaque files are stored.
package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// odAlphaOut is the octal dump of "abcdefghijklmnopqrstuvwxyz\n" read as
// little-endian 16-bit values.
const odAlphaOut = "0000000 061141 062143 063145 064147 065151 066153 067155 070157\n" +
	"0000020 071161 072163 073165 074167 075171 000012\n" +
	"0000033\n"

func runOdStdin(t *testing.T, stdin string, args ...string) runResult {
	t.Helper()
	return runBuiltinStdin(t, "od", stdin, args...)
}

func TestOdDefaultFormat(t *testing.T) {
	input := "abcdefghijklmnopqrstuvwxyz\n"
	for _, args := range [][]string{
		{"--endian=little"},
		{"--endian=littl"},
		{"--endian=l"},
		{"--endian=little", "-t", "o2"},
	} {
		r := runOdStdin(t, input, args...)
		assertExitCode(t, r, 0)
		if r.stdout != odAlphaOut {
			t.Errorf("od %v stdout = %q, want %q", args, r.stdout, odAlphaOut)
		}
		if r.stderr != "" {
			t.Errorf("od %v stderr = %q, want empty", args, r.stderr)
		}
	}
}

func TestOdFilesAndConcatenation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one", "abcdefghijklmnop")
	writeFile(t, dir, "two", "qrstuvwxyz\n")
	r := runBuiltinIn(t, dir, "od", "--endian=little", "one", "two")
	assertExitCode(t, r, 0)
	if r.stdout != odAlphaOut {
		t.Errorf("stdout = %q, want %q", r.stdout, odAlphaOut)
	}
}

func TestOdStdInTheMiddle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one", "abcdefg")
	writeFile(t, dir, "three", "qrstuvwxyz\n")
	cmd, _ := Lookup("od")
	var stdout, stderr strings.Builder
	ctx := &Context{
		Dir:    dir,
		Args:   []string{"--endian=little", "one", "-", "three"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader("hijklmnop"),
	}
	if code := cmd.Run(ctx); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != odAlphaOut {
		t.Errorf("stdout = %q, want %q", stdout.String(), odAlphaOut)
	}
}

func TestOdMissingFileContinues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "ab")
	r := runBuiltinIn(t, dir, "od", "--endian=little", "a", "missing", "a")
	assertExitCode(t, r, 1)
	if want := "od: missing: No such file or directory\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
	// The dump of both readable operands is still printed, and od suppresses the
	// trailing offset once an operand failed.
	if want := "0000000 061141 061141\n"; r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestOdDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "od", "sub")
	assertExitCode(t, r, 1)
	if want := "od: sub: Is a directory\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestOdMultipleFormats(t *testing.T) {
	r := runOdStdin(t, "abcdefghijklmnopqrstuvwxyz\n", "-c", "-b")
	assertExitCode(t, r, 0)
	want := "0000000   a   b   c   d   e   f   g   h   i   j   k   l   m   n   o   p\n" +
		"        141 142 143 144 145 146 147 150 151 152 153 154 155 156 157 160\n" +
		"0000020   q   r   s   t   u   v   w   x   y   z  \\n\n" +
		"        161 162 163 164 165 166 167 170 171 172 012\n" +
		"0000033\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestOdTypeStrings(t *testing.T) {
	// two little-endian bytes: 0x01 0x23
	const input = "\x01\x23"
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-An", "-t", "x1"}, " 01 23\n"},
		{[]string{"-An", "-t", "x2"}, " 2301\n"},
		{[]string{"-An", "-t", "x4"}, " 00002301\n"},
		{[]string{"-An", "-t", "o1"}, " 001 043\n"},
		{[]string{"-An", "-t", "o2"}, " 021401\n"},
		{[]string{"-An", "-t", "o4"}, " 00000021401\n"},
		{[]string{"-An", "-t", "u1"}, "   1  35\n"},
		{[]string{"-An", "-t", "u2"}, "  8961\n"},
		{[]string{"-An", "-t", "d2"}, "   8961\n"},
		{[]string{"-An", "-t", "a"}, " soh   #\n"},
		{[]string{"-An", "-t", "c"}, " 001   #\n"},
		{[]string{"-An", "-t", "cz"}, " 001   #" + strings.Repeat(" ", 58) + ">.#<\n"},
		{[]string{"-An", "-t", "x1z", "-t", "a"},
			"  01  23" + strings.Repeat(" ", 58) + ">.#<\n soh   #\n"},
	}
	for _, tc := range cases {
		r := runOdStdin(t, input, append([]string{"--endian=little"}, tc.args...)...)
		assertExitCode(t, r, 0)
		if r.stdout != tc.want {
			t.Errorf("od %v stdout = %q, want %q", tc.args, r.stdout, tc.want)
		}
	}
}

func TestOdTraditionalSelectors(t *testing.T) {
	// 0x0102 little-endian: -b oct1, -o oct2, -s dec16s, -d dec16u, -x hex2,
	// -i dec32s, -l dec64s, -f float32, -e float64.
	input := "\x02\x01\x00\x00\x00\x00\x00\x00"
	cases := []struct {
		flag string
		want string
	}{
		{"-b", "0000000 002 001 000 000 000 000 000 000\n0000010\n"},
		{"-o", "0000000 000402 000000 000000 000000\n0000010\n"},
		{"-c", "0000000 002 001  \\0  \\0  \\0  \\0  \\0  \\0\n0000010\n"},
		{"-d", "0000000   258     0     0     0\n0000010\n"},
		{"-s", "0000000    258      0      0      0\n0000010\n"},
		{"-x", "0000000 0102 0000 0000 0000\n0000010\n"},
		{"-i", "0000000         258           0\n0000010\n"},
		{"-l", "0000000                  258\n0000010\n"},
		{"-a", "0000000 stx soh nul nul nul nul nul nul\n0000010\n"},
	}
	for _, tc := range cases {
		r := runOdStdin(t, input, "--endian=little", tc.flag)
		assertExitCode(t, r, 0)
		if r.stdout != tc.want {
			t.Errorf("od %s stdout = %q, want %q", tc.flag, r.stdout, tc.want)
		}
	}
}

func TestOdSkipBytes(t *testing.T) {
	r := runOdStdin(t, "abcdefghijklmnopq", "-c", "--skip-bytes=5")
	assertExitCode(t, r, 0)
	want := "0000005   f   g   h   i   j   k   l   m   n   o   p   q\n0000021\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}

	// -j with a hexadecimal argument.
	r = runOdStdin(t, "abcdefghijklmnopq", "-c", "--skip-bytes=0xB")
	assertExitCode(t, r, 0)
	if want := "0000013   l   m   n   o   p   q\n0000021\n"; r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestOdSkipAcrossFiles(t *testing.T) {
	cases := []struct {
		files []string
		skip  string
		want  string
	}{
		{[]string{"a"}, "1", ""},
		{[]string{"a", "b"}, "2", ""},
		{[]string{"a", "b", "c"}, "3", ""},
		{[]string{"a", "b", "c", "d"}, "3", "   d\n"},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		var names []string
		for i, content := range tc.files {
			name := string(rune('g' + i))
			writeFile(t, dir, name, content)
			names = append(names, name)
		}
		r := runBuiltinIn(t, dir, "od", append([]string{"-c", "-j", tc.skip, "-An"}, names...)...)
		assertExitCode(t, r, 0)
		if r.stdout != tc.want {
			t.Errorf("od -j %s over %v stdout = %q, want %q", tc.skip, tc.files, r.stdout, tc.want)
		}
	}
}

func TestOdSkipPastEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "abc")
	writeFile(t, dir, "b", "de")
	r := runBuiltinIn(t, dir, "od", "-j6", "a", "b")
	assertExitCode(t, r, 1)
	if want := "od: cannot skip past end of combined input\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}

	// A regular file shorter than the skip is an error too, with no offset line.
	r = runBuiltinIn(t, dir, "od", "-j10", "b")
	assertExitCode(t, r, 1)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}

	// stdin can only be read, but skipping past its end is still an error.
	r = runOdStdin(t, "12345", "--skip-bytes=10")
	if r.code == 0 {
		t.Error("exit = 0, want failure")
	}
}

func TestOdReadBytes(t *testing.T) {
	r := runOdStdin(t, "abcdefghijklmnopqrstuvwxyz\n12345678", "--endian=little", "--read-bytes=27")
	assertExitCode(t, r, 0)
	if r.stdout != odAlphaOut {
		t.Errorf("stdout = %q, want %q", r.stdout, odAlphaOut)
	}
}

func TestOdWidth(t *testing.T) {
	r := runOdStdin(t, "\x00\x00\x00\x00\x00\x00\x00\x00", "-w4", "-v")
	assertExitCode(t, r, 0)
	want := "0000000 000000 000000\n0000004 000000 000000\n0000010\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}

	// A width that is not a multiple of the type size is corrected with a warning.
	r = runOdStdin(t, "\x00\x00\x00\x00", "-w5", "-v")
	assertExitCode(t, r, 0)
	want = "0000000 000000\n0000002 000000\n0000004\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
	if wantWarn := "od: warning: invalid width 5; using 2 instead\n"; r.stderr != wantWarn {
		t.Errorf("stderr = %q, want %q", r.stderr, wantWarn)
	}

	// -w without a value means 32 bytes per line.
	r = runOdStdin(t, strings.Repeat("\x00", 40), "-w")
	assertExitCode(t, r, 0)
	line := "0000000" + strings.Repeat(" 000000", 16) + "\n"
	want = line + "0000040" + strings.Repeat(" 000000", 4) + "\n0000050\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}

	for _, tc := range []struct{ args, stderr string }{
		{"-w0", "od: invalid -w argument '0'\n"},
		{"-w-1", "od: invalid -w argument '-1'\n"},
		{"-ww", "od: invalid -w argument 'w'\n"},
	} {
		r := runOdStdin(t, "", tc.args)
		assertExitCode(t, r, 1)
		if r.stderr != tc.stderr {
			t.Errorf("od %s stderr = %q, want %q", tc.args, r.stderr, tc.stderr)
		}
	}
}

func TestOdSuppressDuplicates(t *testing.T) {
	input := strings.Repeat("\x00", 16) + "\x01" + strings.Repeat("\x00", 16) + strings.Repeat("\x00", 8)
	want := "0000000 00000000000\n" +
		"         0000  0000\n" +
		"*\n" +
		"0000020 00000000001\n" +
		"         0001  0000\n" +
		"0000024 00000000000\n" +
		"         0000  0000\n" +
		"*\n" +
		"0000050 00000000000\n" +
		"         0000\n" +
		"0000051\n"
	r := runOdStdin(t, input, "-w4", "-O", "-x")
	assertExitCode(t, r, 0)
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}

	// -v prints every repeated line.
	r = runOdStdin(t, input, "-w4", "-O", "-x", "-v")
	assertExitCode(t, r, 0)
	if strings.Contains(r.stdout, "*") {
		t.Errorf("-v output contains a suppression marker: %q", r.stdout)
	}
}

func TestOdAddressRadix(t *testing.T) {
	cases := []struct{ args, want string }{
		{"-Ax", "000000 00000000\n000004\n"},
		{"-Ad", "0000000 00000000\n0000004\n"},
		{"-An", " 00000000\n"},
		{"-Anone", " 00000000\n"},
	}
	for _, tc := range cases {
		r := runOdStdin(t, strings.Repeat("\x00", 4), "-X", tc.args)
		assertExitCode(t, r, 0)
		if r.stdout != tc.want {
			t.Errorf("od -X %s stdout = %q, want %q", tc.args, r.stdout, tc.want)
		}
	}

	for _, args := range [][]string{{"-Ab"}, {"-A", ""}} {
		r := runOdStdin(t, "", args...)
		assertExitCode(t, r, 1)
		if r.stderr == "" {
			t.Errorf("od %v: want a diagnostic on stderr", args)
		}
	}
	r := runOdStdin(t, "", "-A", "")
	if want := "od: Radix cannot be empty, and must be one of [o, d, x, n]\n"; r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

func TestOdBigEndian(t *testing.T) {
	input := "\xc0\x00\x00\x00\x00\x00\x00\x00"
	r := runOdStdin(t, input, "--endian=big", "-F", "-f", "-X", "-x")
	assertExitCode(t, r, 0)
	want := "0000000                              -2\n" +
		"                     -2               0\n" +
		"               c0000000        00000000\n" +
		"           c000    0000    0000    0000\n" +
		"0000010\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestOdShortFinalBlock(t *testing.T) {
	// Three bytes with a 4-byte type: the last value is zero-filled, and the
	// final offset counts only the real bytes.
	r := runOdStdin(t, "\x01\x02\x03", "--endian=little", "-An", "-t", "x4")
	assertExitCode(t, r, 0)
	if want := " 00030201\n"; r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
	r = runOdStdin(t, "\x01\x02\x03", "--endian=little", "-t", "x4")
	assertExitCode(t, r, 0)
	if want := "0000000 00030201\n0000003\n"; r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestOdFloatFormats(t *testing.T) {
	// -f: binary32 values -1.2345679, 12345678, -9.876543e+37, -0, NaN, 1e-40.
	input := "\x52\x06\x9e\xbf" + "\x4e\x61\x3c\x4b" + "\x0f\x9b\x94\xfe" +
		"\x00\x00\x00\x80" + "\xff\xff\xff\x7f" + "\xc2\x16\x01\x00"
	r := runOdStdin(t, input, "--endian=little", "-f")
	assertExitCode(t, r, 0)
	want := "0000000      -1.2345679        12345678   -9.876543e+37              -0\n" +
		"0000020             NaN           1e-40\n" +
		"0000030\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}

	// -F: binary64 values 12345678912345678, 0, -2.2250738585072014e-308, -2.
	input = "\x27\x6b\x0a\x2f\x2a\xee\x45\x43" + "\x00\x00\x00\x00\x00\x00\x00\x00" +
		"\x00\x00\x00\x00\x00\x00\x10\x80" + "\x00\x00\x00\x00\x00\x00\x00\xc0"
	r = runOdStdin(t, input, "--endian=little", "-F")
	assertExitCode(t, r, 0)
	want = "0000000        12345678912345678                        0\n" +
		"0000020 -2.2250738585072014e-308                       -2\n" +
		"0000040\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
}

func TestOdHalfAndBFloat(t *testing.T) {
	// 1.0, 0, -0, inf, -inf, NaN, -6.1035156e-5 as little-endian binary16.
	half := "\x00\x3c" + "\x00\x00" + "\x00\x80" + "\x00\x7c" + "\x00\xfc" + "\x00\xfe" + "\x00\x84"
	want := "0000000               1               0              -0             inf\n" +
		"0000010            -inf             NaN   -6.1035156e-5\n" +
		"0000016\n"
	for _, flag := range []string{"-tf2", "-tfH"} {
		r := runOdStdin(t, half, "--endian=little", flag, "-w8")
		assertExitCode(t, r, 0)
		if r.stdout != want {
			t.Errorf("od %s stdout = %q, want %q", flag, r.stdout, want)
		}
	}

	// bfloat16 1.0, 0, -0, inf, -inf, NaN, -6.1035156e-5.
	bfloat := "\x80\x3f" + "\x00\x00" + "\x00\x80" + "\x80\x7f" + "\x80\xff" + "\xc0\x7f" + "\x80\xb8"
	r := runOdStdin(t, bfloat, "--endian=little", "-tfB", "-w8")
	assertExitCode(t, r, 0)
	if r.stdout != want {
		t.Errorf("od -tfB stdout = %q, want %q", r.stdout, want)
	}
}

func TestOdAsciiDump(t *testing.T) {
	input := "\x00\x01\x0a\x0d\x10\x1f\x20\x61\x62\x63\x7d\x7e\x7f\x80\x90\xa0" +
		"\xb0\xc0\xd0\xe0\xf0\xff"
	want := "0000000  00  01  0a  0d  10  1f  20  61  62  63  7d  7e  7f  80  90  a0" +
		"  >...... abc}~....<\n" +
		"        nul soh  nl  cr dle  us  sp   a   b   c   }   ~ del nul dle  sp\n" +
		"         \\0 001  \\n  \\r 020 037       a   b   c   }   ~ 177 200 220 240" +
		"  >...... abc}~....<\n" +
		"0000020  b0  c0  d0  e0  f0  ff" + strings.Repeat(" ", 42) + ">......<\n" +
		"          0   @   P   `   p del\n" +
		"        260 300 320 340 360 377" + strings.Repeat(" ", 42) + ">......<\n" +
		"0000026\n"
	r := runOdStdin(t, input, "-tx1zacz")
	assertExitCode(t, r, 0)
	if r.stdout != want {
		t.Errorf("stdout = %q\nwant %q", r.stdout, want)
	}
}

func TestOdInvalidByteCounts(t *testing.T) {
	for _, opt := range []string{"--read-bytes", "--skip-bytes", "--width"} {
		for _, tc := range []struct{ value, want string }{
			{"x", "od: invalid " + opt + " argument 'x'\n"},
			{"1fb4t", "od: invalid suffix in " + opt + " argument '1fb4t'\n"},
			{"1Y", odSizeTooLargeOrSuffix(opt)},
		} {
			r := runOdStdin(t, "", opt+"="+tc.value, "file")
			assertExitCode(t, r, 1)
			if r.stderr != tc.want {
				t.Errorf("od %s=%s stderr = %q, want %q", opt, tc.value, r.stderr, tc.want)
			}
		}
	}

	// A hyphen-leading count belongs to the option, not to the file list.
	for _, opt := range []string{"-N", "-j"} {
		r := runOdStdin(t, "", opt, "-1")
		assertExitCode(t, r, 1)
		if want := "od: invalid " + opt + " argument '-1'\n"; r.stderr != want {
			t.Errorf("od %s -1 stderr = %q, want %q", opt, r.stderr, want)
		}
	}
}

func TestOdUnsupportedOptions(t *testing.T) {
	cases := []struct {
		args   []string
		stderr string
	}{
		{[]string{"--definitely-invalid"}, "od: unknown option '--definitely-invalid'\n"},
		{[]string{"-Q"}, "od: unknown option '-Q'\n"},
		{[]string{"-S0"}, "od: unknown option '-S'\n"},
		{[]string{"--traditional"}, "od: unknown option '--traditional'\n"},
		{[]string{"-t", "f16"}, "od: invalid size 16 in type string 'f16'\n"},
		{[]string{"-t", "g"}, "od: invalid character 'g' in type string 'g'\n"},
		{[]string{"-t", "x3"}, "od: invalid size 3 in type string 'x3'\n"},
	}
	for _, tc := range cases {
		r := runOdStdin(t, "abc", tc.args...)
		assertExitCode(t, r, 1)
		if r.stderr != tc.stderr {
			t.Errorf("od %v stderr = %q, want %q", tc.args, r.stderr, tc.stderr)
		}
		if r.stdout != "" {
			t.Errorf("od %v stdout = %q, want empty", tc.args, r.stdout)
		}
	}
}

func TestOdHelpAndVersion(t *testing.T) {
	r := runOdStdin(t, "", "--help")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "--address-radix") || !strings.Contains(r.stdout, "--endian") {
		t.Errorf("--help output = %q", r.stdout)
	}
	r = runOdStdin(t, "", "-h")
	assertExitCode(t, r, 0)
	if !strings.HasPrefix(r.stdout, "Usage:") {
		t.Errorf("-h output = %q", r.stdout)
	}
	r = runOdStdin(t, "", "--version")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.stdout, "0.1.0") {
		t.Errorf("--version output = %q", r.stdout)
	}
}

func TestOdEmptyInput(t *testing.T) {
	r := runOdStdin(t, "")
	assertExitCode(t, r, 0)
	if want := "0000000\n"; r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}
	r = runOdStdin(t, "", "-An")
	assertExitCode(t, r, 0)
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
}

func TestOdParseWidth(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		err  odSizeErr
	}{
		{"4", 4, odSizeOK},
		{"010", 10, odSizeOK},
		{"08", 8, odSizeOK},
		{"00", 0, odSizeOK},
		{"", 0, odSizeInvalid},
		{"abc", 0, odSizeInvalid},
		{"-4", 0, odSizeInvalid},
		{"1K", 0, odSizeSuffix},
		{"0x8", 0, odSizeSuffix},
		{"18446744073709551616", 0, odSizeTooLarge},
	}
	for _, tc := range cases {
		got, err := odParseWidth(tc.in)
		if got != tc.want || err != tc.err {
			t.Errorf("odParseWidth(%q) = %d, %v; want %d, %v", tc.in, got, err, tc.want, tc.err)
		}
	}
}

func TestOdParseByteCount(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		err  odSizeErr
	}{
		{"5", 5, odSizeOK},
		{"010", 8, odSizeOK},        // octal
		{"010b", 8 * 512, odSizeOK}, // blocks
		{"010k", 8 << 10, odSizeOK},
		{"0xf", 15, odSizeOK},
		{"0x1b", 27, odSizeOK},
		{"0x10k", 16 << 10, odSizeOK},
		{"1K", 1 << 10, odSizeOK},
		{"1KB", 1000, odSizeOK},
		{"1m", 1 << 20, odSizeOK},
		{"x", 0, odSizeInvalid},
		{"1fb4t", 0, odSizeSuffix},
		{"1Y", 0, odSizeTooLarge},
	}
	for _, tc := range cases {
		got, err := odParseByteCount(tc.in)
		if got != tc.want || err != tc.err {
			t.Errorf("odParseByteCount(%q) = %d, %v; want %d, %v", tc.in, got, err, tc.want, tc.err)
		}
	}
}

// odSizeTooLargeOrSuffix is the diagnostic for a "1Y" byte count: -j and -N
// reject the Y unit as too large, while -w only accepts plain digits and
// reports the unit as an invalid suffix.
func odSizeTooLargeOrSuffix(opt string) string {
	if opt == "--width" {
		return "od: invalid suffix in --width argument '1Y'\n"
	}
	return "od: " + opt + " argument '1Y' too large\n"
}
