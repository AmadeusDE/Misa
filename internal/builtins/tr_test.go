package builtins

import "testing"

func TestTrTranslate(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "hello", "a-z", "A-Z")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "HELLO")
}

func TestTrDelete(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "hello world", "-d", "lo")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "he wrd")
}

func TestTrSqueeze(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "aabbcc", "-s", "a-z")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abc")
}

func TestTrComplement(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "hello 123 world", "-d", "-c", "a-z")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "helloworld")
}

func TestTrClassUpper(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "Hello World", "[:upper:]", "[:lower:]")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello world")
}

func TestTrClassLower(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "hello", "[:lower:]", "[:upper:]")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "HELLO")
}

func TestTrEscapes(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "a\tb", "\\t", " ")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a b")
}

func TestTrDeleteSqueeze(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "aaabbbccc", "-d", "-s", "a", "b")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "bccc")
}

func TestTrDeleteSqueezeOneMissing(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "aaabbbccc", "-d", "-s", "a")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Two strings must be given")
}

func TestTrMissingOperand(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "input")
	assertExitCode(t, r, 1)
}

func TestTrDigits(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "abc123def456", "-d", "[:digit:]")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "abcdef")
}

func TestTrSingleCharRange(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "abc", "a", "x")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "xbc")
}

func TestTrXdigit(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "0123456789abcdefABCDEFghij", "-d", "[:xdigit:]")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "ghij")
}

func TestTrXdigitTranslate(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "af", "[:xdigit:]", "0")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "00")
}

func TestTrOctalEscape(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "X", "X", "\\015")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\r")
}

func TestTrExtraOperand(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "abc", "a", "b", "c")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "extra operand")
}

func TestTrMissingSet2(t *testing.T) {
	r := runBuiltinStdin(t, "tr", "abc", "a")
	assertExitCode(t, r, 1)
}
