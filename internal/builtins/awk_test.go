package builtins

import (
	"strings"
	"testing"
)

func TestAwkProgramFromArgs(t *testing.T) {
	r := runBuiltinStdin(t, "awk", "a b\nc d\n", "{print $2, NF}")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b 2\nd 2\n")
}

func TestAwkFieldSeparatorOption(t *testing.T) {
	r := runBuiltinStdin(t, "awk", "a:b\n", "-F:", "{print $2}")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\n")
	r = runBuiltinStdin(t, "awk", "a\tb c\n", "-F", `\t`, "{print $2}")
	assertStdout(t, r, "b c\n")
}

func TestAwkAssignOption(t *testing.T) {
	r := runBuiltin(t, "awk", "-v", "x=2", "-v", "y=3", "BEGIN{print x*y}")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "6\n")
}

func TestAwkProgramFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one.awk", "BEGIN { n = 1 }\n")
	writeFile(t, dir, "two.awk", "END { print n, NR }\n")
	writeFile(t, dir, "in.txt", "x\ny\n")
	r := runBuiltinIn(t, dir, "awk", "-f", "one.awk", "-f", "two.awk", "in.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1 2\n")
}

func TestAwkMissingProgramFile(t *testing.T) {
	r := runBuiltin(t, "awk", "-f", "nope.awk")
	assertExitCode(t, r, 2)
}

func TestAwkUsage(t *testing.T) {
	r := runBuiltin(t, "awk")
	assertExitCode(t, r, 2)
}

func TestAwkOperandsAfterProgram(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "1\n")
	writeFile(t, dir, "b", "2\n")
	r := runBuiltinIn(t, dir, "awk", "--", "{s += $1} END {print s}", "a", "b")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n")
}

func TestAwkExitStatus(t *testing.T) {
	r := runBuiltin(t, "awk", "BEGIN { exit 7 }")
	assertExitCode(t, r, 7)
}

func TestAwkSyntaxError(t *testing.T) {
	r := runBuiltin(t, "awk", "BEGIN {")
	assertExitCode(t, r, 2)
}

func TestAwkUnsupportedNamesFeature(t *testing.T) {
	r := runBuiltin(t, "awk", `BEGIN { x = gensub(/a/, "b", "g", "aaa") }`)
	assertExitCode(t, r, 2)
	if !strings.Contains(r.stderr, "gensub") {
		t.Fatalf("stderr %q does not name gensub", r.stderr)
	}
}

func TestAwkByteMode(t *testing.T) {
	r := runBuiltinStdin(t, "awk", "あ\n", "-b", "{print length($0)}")
	assertStdout(t, r, "3\n")
	r = runBuiltinStdin(t, "awk", "あ\n", "{print length($0)}")
	assertStdout(t, r, "1\n")
}
