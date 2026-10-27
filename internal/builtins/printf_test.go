package builtins

import "testing"

func TestPrintfString(t *testing.T) {
	r := runBuiltin(t, "printf", "%s", "hello")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello")
}

func TestPrintfStringNewline(t *testing.T) {
	r := runBuiltin(t, "printf", "%s\\n", "hello")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
}

func TestPrintfDecimal(t *testing.T) {
	r := runBuiltin(t, "printf", "%d", "42")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "42")
}

func TestPrintfZeroPad(t *testing.T) {
	r := runBuiltin(t, "printf", "%05d", "42")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "00042")
}

func TestPrintfHex(t *testing.T) {
	r := runBuiltin(t, "printf", "%x", "255")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "ff")
}

func TestPrintfOctal(t *testing.T) {
	r := runBuiltin(t, "printf", "%o", "8")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "10")
}

func TestPrintfPercent(t *testing.T) {
	r := runBuiltin(t, "printf", "100%%")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "100%")
}

func TestPrintfEscapeTab(t *testing.T) {
	r := runBuiltin(t, "printf", "a\\tb")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\tb")
}

func TestPrintfEscapeBackslash(t *testing.T) {
	r := runBuiltin(t, "printf", "a\\\\b")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\\b")
}

func TestPrintfWidthString(t *testing.T) {
	r := runBuiltin(t, "printf", "%10s", "hi")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "        hi")
}

func TestPrintfLeftAlign(t *testing.T) {
	r := runBuiltin(t, "printf", "%-10s|", "hi")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hi        |")
}

func TestPrintfMissingFormat(t *testing.T) {
	r := runBuiltin(t, "printf")
	assertExitCode(t, r, 1)
}

func TestPrintfMultiArgs(t *testing.T) {
	r := runBuiltin(t, "printf", "%s=%d\\n", "x", "5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "x=5\n")
}

func TestPrintfChar(t *testing.T) {
	r := runBuiltin(t, "printf", "%c", "A")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "A")
}

func TestPrintfQuoteChar(t *testing.T) {
	r := runBuiltin(t, "printf", "%d\\n", "'x")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "120\n")
}

func TestPrintfDynamicWidth(t *testing.T) {
	r := runBuiltin(t, "printf", "|%*.*f|\\n", "23", "12", "5.25")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "|         5.250000000000|\n")
}

func TestPrintfDynamicWidthNegative(t *testing.T) {
	r := runBuiltin(t, "printf", "|%*f|\\n", "-23", "5.25")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "|5.250000               |\n")
}

func TestPrintfSizeModifierZ(t *testing.T) {
	r := runBuiltin(t, "printf", "%zd\\n", "-5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "-5\n")
}

func TestPrintfSizeModifierL(t *testing.T) {
	r := runBuiltin(t, "printf", "%ld\\n", "-5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "-5\n")
}

func TestPrintfLeadingWhitespace(t *testing.T) {
	r := runBuiltin(t, "printf", "%d\\n", "   3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestPrintfLeadingPlus(t *testing.T) {
	r := runBuiltin(t, "printf", "%d\\n", "+3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestPrintfBarePercent(t *testing.T) {
	r := runBuiltin(t, "printf", "%", "a")
	assertExitCode(t, r, 1)
}

func TestPrintfInvalidSpec(t *testing.T) {
	r := runBuiltin(t, "printf", "%r", "a")
	assertExitCode(t, r, 1)
}

func TestPrintfLeadingZeroFlag(t *testing.T) {
	r := runBuiltin(t, "printf", "%0*d\\n", "2", "1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "01\n")
}

func TestPrintfBadInput(t *testing.T) {
	r := runBuiltin(t, "printf", "%d\\n", "bad")
	assertStdout(t, r, "0\n")
	assertStderrContains(t, r, "expected a numeric value")
}

func TestPrintfHexFloat(t *testing.T) {
	r := runBuiltin(t, "printf", "%a", "1.5")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "0x")
}

func TestPrintfInfLowercase(t *testing.T) {
	r := runBuiltin(t, "printf", "%e", "5e8123456789012345678")
	assertStdoutContains(t, r, "inf")
}

func TestPrintfEmoji(t *testing.T) {
	r := runBuiltin(t, "printf", "Status: %s \xf0\x9f\x8e\xaf Count: %d\\n", "Success \xf0\x9f\x9a\x80", "42")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "Status: Success \xf0\x9f\x9a\x80 \xf0\x9f\x8e\xaf Count: 42\n")
}

func TestPrintfPrecisionOnChar(t *testing.T) {
	r := runBuiltin(t, "printf", "%5.2c", "q")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid conversion specification")
}
