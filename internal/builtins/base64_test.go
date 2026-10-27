package builtins

import (
	"strings"
	"testing"
)

func TestBase64EncodeStdin(t *testing.T) {
	r := runBuiltinStdin(t, "base64", "hello\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "aGVsbG8K\n")
}

func TestBase64DecodeStdin(t *testing.T) {
	r := runBuiltinStdin(t, "base64", "aGVsbG8K\n", "-d")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
}

func TestBase64RoundTrip(t *testing.T) {
	input := "The quick brown fox jumps over the lazy dog"
	enc := runBuiltinStdin(t, "base64", input)
	assertExitCode(t, enc, 0)
	dec := runBuiltinStdin(t, "base64", enc.stdout, "-d")
	assertExitCode(t, dec, 0)
	assertStdout(t, dec, input)
}

func TestBase64NoWrap(t *testing.T) {
	input := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	r := runBuiltinStdin(t, "base64", input, "-w", "0")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line with -w 0, got %d", len(lines))
	}
}

func TestBase64File(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "in.txt", "test data\n")
	r := runBuiltinIn(t, dir, "base64", "in.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "dGVzdCBkYXRhCg==\n")
}

func TestBase64DecodeInvalid(t *testing.T) {
	r := runBuiltinStdin(t, "base64", "not valid!!!", "-d")
	assertExitCode(t, r, 1)
}

func TestBase64DecodeIgnoreGarbage(t *testing.T) {
	r := runBuiltinStdin(t, "base64", "aGVs\n!!!bG8K\n", "-d", "-i")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
}

func TestBase64WrapBadArg(t *testing.T) {
	r := runBuiltinStdin(t, "base64", "hello", "-w", "b")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid wrap size: 'b'")
}

func TestBase64WrapMissingArg(t *testing.T) {
	r := runBuiltinStdin(t, "base64", "hello", "-w")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "requires an argument")
	assertStderrContains(t, r, "For more information, try '--help'.")
}

func TestBase64TrailingSlash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "data")
	r := runBuiltinIn(t, dir, "base64", "a/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Not a directory")
}

func TestBase64FileNotFound(t *testing.T) {
	r := runBuiltin(t, "base64", "nonexistent")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
}
