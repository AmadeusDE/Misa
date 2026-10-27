package native

import "testing"

func TestPrintf_String(t *testing.T) {
	r := run(t, "printf", "%s\n", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestPrintf_Integer(t *testing.T) {
	r := run(t, "printf", "%d\n", "42")
	wantSuccess(t, r)
	wantOut(t, r, "42\n")
}

func TestPrintf_Hex(t *testing.T) {
	r := run(t, "printf", "%x\n", "255")
	wantSuccess(t, r)
	wantOut(t, r, "ff\n")
}

func TestPrintf_Octal(t *testing.T) {
	r := run(t, "printf", "%o\n", "8")
	wantSuccess(t, r)
	wantOut(t, r, "10\n")
}

func TestPrintf_Padding(t *testing.T) {
	r := run(t, "printf", "%10s", "hi")
	wantSuccess(t, r)
	wantOut(t, r, "        hi")
}

func TestPrintf_LeftAlign(t *testing.T) {
	r := run(t, "printf", "%-10s|", "hi")
	wantSuccess(t, r)
	wantOut(t, r, "hi        |")
}

func TestPrintf_ZeroPad(t *testing.T) {
	r := run(t, "printf", "%05d", "42")
	wantSuccess(t, r)
	wantOut(t, r, "00042")
}

func TestPrintf_Backslash(t *testing.T) {
	r := run(t, "printf", "a\\tb\\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\tb\n")
}

func TestPrintf_OctalEscape(t *testing.T) {
	r := run(t, "printf", "\\101\\102\n")
	wantSuccess(t, r)
	wantOut(t, r, "AB\n")
}

func TestPrintf_MultipleArgs(t *testing.T) {
	r := run(t, "printf", "%s=%s\n", "key", "value")
	wantSuccess(t, r)
	wantOut(t, r, "key=value\n")
}

func TestPrintf_Repeat(t *testing.T) {
	r := run(t, "printf", "%s\n", "a", "b", "c")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\nc\n")
}

func TestPrintf_Char(t *testing.T) {
	r := run(t, "printf", "%c", "A")
	wantSuccess(t, r)
	wantOut(t, r, "A")
}

func TestPrintf_NoArgs(t *testing.T) {
	r := run(t, "printf", "hello")
	wantSuccess(t, r)
	wantOut(t, r, "hello")
}

func TestPrintf_EscapeOctal(t *testing.T) {
	r := run(t, "printf", "\\101")
	wantSuccess(t, r)
	wantOut(t, r, "A")
}

func TestPrintf_Percent(t *testing.T) {
	r := run(t, "printf", "100%%")
	wantSuccess(t, r)
	wantOut(t, r, "100%")
}

func TestPrintf_InvalidHexEscape(t *testing.T) {
	r := run(t, "printf", `one\xvdtwo`)
	wantFail(t, r)
	wantOut(t, r, "one")
	wantErrContains(t, r, "hexadecimal")
}

func TestPrintf_ShellQuote(t *testing.T) {
	r := run(t, "printf", "%q", "\x01'\x01")
	wantSuccess(t, r)
	wantOut(t, r, "''$'\\001'\\'''$'\\001'")
}

func TestPrintf_VerticalTabAndOctalPercentB(t *testing.T) {
	r1 := run(t, "printf", `one\vtwo\n`)
	wantSuccess(t, r1)
	wantOut(t, r1, "one\vtwo\n")

	r2 := run(t, "printf", `\0007%b`, `\0007`)
	wantSuccess(t, r2)
	wantOut(t, r2, "\x007\a")
}
