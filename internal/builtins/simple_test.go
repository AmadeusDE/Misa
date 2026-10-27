package builtins

import "testing"

func TestEchoEscapeOrdering(t *testing.T) {
	r := runBuiltin(t, "echo", "-E", "-e", "\\na")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\na\n")
}

func TestEchoDisableEscape(t *testing.T) {
	r := runBuiltin(t, "echo", "-e", "-E", "\\na")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\\na\n")
}

func TestPrintenvNull(t *testing.T) {
	r := runBuiltin(t, "printenv", "-0", "PATH")
	assertExitCode(t, r, 0)
	if len(r.stdout) == 0 || r.stdout[len(r.stdout)-1] != 0 {
		t.Errorf("expected null-terminated output, got %q", r.stdout)
	}
}

func TestPrintenvInvalidOption(t *testing.T) {
	r := runBuiltin(t, "printenv", "--bogus")
	assertExitCode(t, r, 2)
}

func TestTrueHelp(t *testing.T) {
	r := runBuiltin(t, "true", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage")
}

func TestTrueVersion(t *testing.T) {
	r := runBuiltin(t, "true", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Misa")
}

func TestFalseAlwaysFails(t *testing.T) {
	r := runBuiltin(t, "false", "--help")
	assertExitCode(t, r, 1)
	assertStdoutContains(t, r, "false")
}

func TestFalseNoArgs(t *testing.T) {
	r := runBuiltin(t, "false")
	assertExitCode(t, r, 1)
}

func TestYesInvalidOption(t *testing.T) {
	r := runBuiltin(t, "yes", "--definitely-invalid")
	assertExitCode(t, r, 1)
}

func TestYesHelp(t *testing.T) {
	r := runBuiltin(t, "yes", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Usage")
}
