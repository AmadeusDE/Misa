package builtins

import (
	"strings"
	"testing"
)

func TestTsortRegistered(t *testing.T) {
	_, ok := Lookup("tsort")
	if !ok {
		t.Fatal("tsort not registered")
	}
}

func TestTsortSimple(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "a b\nb c\n")
	assertExitCode(t, r, 0)
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	pos := map[string]int{}
	for i, l := range lines {
		pos[l] = i
	}
	if pos["a"] > pos["b"] || pos["b"] > pos["c"] {
		t.Errorf("wrong ordering: %v", lines)
	}
}

func TestTsortSelfEdge(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "a a\n")
	assertExitCode(t, r, 0)
	if strings.TrimSpace(r.stdout) != "a" {
		t.Errorf("expected 'a', got %q", r.stdout)
	}
}

func TestTsortCycle(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "a b\nb c\nc d\nc b\n")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "a\nb\nc\nd\n")
	assertStderrContains(t, r, "input contains a loop")
	assertStderrContains(t, r, "tsort: b")
	assertStderrContains(t, r, "tsort: c")
}

func TestTsortTwoCycles(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "a b\nb c\nc b\nb d\nd b\n")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "a\nb\nd\nc\n")
}

func TestTsortOddTokens(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "a\n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "odd number of tokens")
}

func TestTsortEmpty(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "")
	assertExitCode(t, r, 0)
}

func TestTsortFloatingNodes(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "d d\nc c\na a\nb b\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\nc\nd\n")
}

func TestTsortLinearBranch(t *testing.T) {
	r := runBuiltinStdin(t, "tsort", "a b\nb c\nc d\nd e\ne f\nf g\nc x\nx y\ny z\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\nc\nx\nd\ny\ne\nz\nf\ng\n")
}

func TestTsortExtraOperand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "")
	writeFile(t, dir, "g", "")
	r := runBuiltinIn(t, dir, "tsort", "f", "g")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "extra operand")
}

func TestTsortHelpVersion(t *testing.T) {
	r := runBuiltin(t, "tsort", "--help")
	assertExitCode(t, r, 0)
	r = runBuiltin(t, "tsort", "--version")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "Misa")
}

func TestTsortFileNotFound(t *testing.T) {
	r := runBuiltin(t, "tsort", "nosuchfile.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
}
