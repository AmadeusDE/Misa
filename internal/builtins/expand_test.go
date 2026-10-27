package builtins

import "testing"

func TestExpandBasic(t *testing.T) {
	r := runBuiltinStdin(t, "expand", "a\tb\tc\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a       b       c\n")
}

func TestExpandCustomTab(t *testing.T) {
	r := runBuiltinStdin(t, "expand", "a\tb\n", "-t", "4")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a   b\n")
}

func TestExpandFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tabs.txt", "x\ty\n")
	r := runBuiltinIn(t, dir, "expand", "tabs.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "x       y\n")
}

func TestExpandNoTabs(t *testing.T) {
	r := runBuiltinStdin(t, "expand", "no tabs here\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "no tabs here\n")
}

func TestExpandMultipleTabs(t *testing.T) {
	r := runBuiltinStdin(t, "expand", "\t\tindented\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "                indented\n")
}

func TestUnexpandBasic(t *testing.T) {
	r := runBuiltinStdin(t, "unexpand", "        hello\n")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "\t")
	assertStdoutContains(t, r, "hello")
}

func TestUnexpandCustomTab(t *testing.T) {
	r := runBuiltinStdin(t, "unexpand", "    hello\n", "-t", "4")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\thello\n")
}

func TestUnexpandFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "spaces.txt", "        x\n")
	r := runBuiltinIn(t, dir, "unexpand", "spaces.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "\t")
}

func TestExpandUnexpandRoundTrip(t *testing.T) {
	input := "\thello\t\tworld\n"
	expanded := runBuiltinStdin(t, "expand", input)
	assertExitCode(t, expanded, 0)
	unexpanded := runBuiltinStdin(t, "unexpand", expanded.stdout, "-a")
	assertExitCode(t, unexpanded, 0)
	assertStdout(t, unexpanded, input)
}

func TestUnexpandTabAbsorption(t *testing.T) {
	// Space before tab should be absorbed into the tab
	r := runBuiltinStdin(t, "unexpand", "123 \t 45678\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "123\t 45678\n")
}

func TestUnexpandFirstOnly(t *testing.T) {
	r := runBuiltinStdin(t, "unexpand", "    hello    world\n", "-t", "4", "-f")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\thello    world\n")
}

func TestUnexpandAllSpaces(t *testing.T) {
	r := runBuiltinStdin(t, "unexpand", "        hello        world\n", "-a")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\thello\t     world\n")
}

func TestUnexpandPreserveTabs(t *testing.T) {
	// Tabs in input should be preserved even without -a
	r := runBuiltinStdin(t, "unexpand", "\thello\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\thello\n")
}

func TestExpandNumericShorthand(t *testing.T) {
	r := runBuiltinStdin(t, "expand", "a\tb\n", "-4")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a   b\n")
}

func TestUnexpandNumericShorthand(t *testing.T) {
	r := runBuiltinStdin(t, "unexpand", "    hello\n", "-4")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\thello\n")
}

func TestExpandErrorMessage(t *testing.T) {
	r := runBuiltin(t, "expand", "nonexistent_file_xyz")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
}
