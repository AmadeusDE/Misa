package builtins

import "testing"

func TestBase32EncodeStdin(t *testing.T) {
	r := runBuiltinStdin(t, "base32", "hello\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "NBSWY3DPBI======\n")
}

func TestBase32DecodeStdin(t *testing.T) {
	r := runBuiltinStdin(t, "base32", "NBSWY3DPBI======\n", "-d")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
}

func TestBase32RoundTrip(t *testing.T) {
	input := "round trip test"
	enc := runBuiltinStdin(t, "base32", input)
	assertExitCode(t, enc, 0)
	dec := runBuiltinStdin(t, "base32", enc.stdout, "-d")
	assertExitCode(t, dec, 0)
	assertStdout(t, dec, input)
}

func TestBase32File(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "in.txt", "test\n")
	r := runBuiltinIn(t, dir, "base32", "in.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "ORSXG5AK\n")
}

func TestBase32DecodeInvalid(t *testing.T) {
	r := runBuiltinStdin(t, "base32", "not valid base32!!!", "-d")
	assertExitCode(t, r, 1)
}

func TestBase32WrapBadArg(t *testing.T) {
	r := runBuiltinStdin(t, "base32", "hello", "-w", "b")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "invalid wrap size: 'b'")
}

func TestBase32WrapMissingArg(t *testing.T) {
	r := runBuiltinStdin(t, "base32", "hello", "--wrap")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "requires an argument")
	assertStderrContains(t, r, "For more information, try '--help'.")
}

func TestBase32TrailingSlash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "data")
	r := runBuiltinIn(t, dir, "base32", "a/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Not a directory")
}
