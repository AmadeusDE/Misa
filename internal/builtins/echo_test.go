package builtins

import "testing"

func TestEchoBasic(t *testing.T) {
	r := runBuiltin(t, "echo", "hello", "world")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello world\n")
}

func TestEchoNoNewline(t *testing.T) {
	r := runBuiltin(t, "echo", "-n", "hello")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello")
}

func TestEchoEmpty(t *testing.T) {
	r := runBuiltin(t, "echo")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\n")
}

func TestEchoEscapes(t *testing.T) {
	r := runBuiltin(t, "echo", "-e", `hello\tworld\n`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\tworld\n\n")
}

func TestEchoEscapeC(t *testing.T) {
	r := runBuiltin(t, "echo", "-e", `hello\cworld`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello")
}

func TestEchoOctalBare(t *testing.T) {
	r := runBuiltin(t, "echo", "-ne", `\41z`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "!z")
}

func TestEchoOctalLeadingZero(t *testing.T) {
	r := runBuiltin(t, "echo", "-ne", `\0041z`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "!z")
}

func TestEchoOctalFourDigit(t *testing.T) {
	r := runBuiltin(t, "echo", "-ne", `\00041z`)
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\x04"+"1z")
}

func TestTrue(t *testing.T) {
	r := runBuiltin(t, "true")
	assertExitCode(t, r, 0)
}

func TestFalse(t *testing.T) {
	r := runBuiltin(t, "false")
	assertExitCode(t, r, 1)
}

func TestPrintenvAll(t *testing.T) {
	t.Setenv("MISA_TEST_VAR", "hello123")
	r := runBuiltin(t, "printenv")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "MISA_TEST_VAR=hello123")
}

func TestPrintenvSingle(t *testing.T) {
	t.Setenv("MISA_TEST_VAR", "value42")
	r := runBuiltin(t, "printenv", "MISA_TEST_VAR")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "value42\n")
}

func TestPrintenvMissing(t *testing.T) {
	r := runBuiltin(t, "printenv", "MISA_NONEXISTENT_VAR_12345")
	assertExitCode(t, r, 1)
}
