package builtins

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// failingWriter fails every write, simulating a full device like /dev/full.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("no space left on device")
}

func TestSeqBasic(t *testing.T) {
	r := runBuiltin(t, "seq", "5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n2\n3\n4\n5\n")
}

func TestSeqFirstLast(t *testing.T) {
	r := runBuiltin(t, "seq", "3", "6")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3\n4\n5\n6\n")
}

func TestSeqIncrement(t *testing.T) {
	r := runBuiltin(t, "seq", "1", "2", "9")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n3\n5\n7\n9\n")
}

func TestSeqReverse(t *testing.T) {
	r := runBuiltin(t, "seq", "--", "5", "-1", "1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "5\n4\n3\n2\n1\n")
}

func TestSeqSeparator(t *testing.T) {
	r := runBuiltin(t, "seq", "-s", ", ", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1, 2, 3\n")
}

func TestSeqEqualWidth(t *testing.T) {
	r := runBuiltin(t, "seq", "-w", "8", "10")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "08\n09\n10\n")
}

func TestSeqFloat(t *testing.T) {
	r := runBuiltin(t, "seq", "0.5", "0.5", "2.0")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "0.5\n1.0\n1.5\n2.0\n")
}

func TestSeqSingle(t *testing.T) {
	r := runBuiltin(t, "seq", "1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n")
}

func TestSeqNoOutput(t *testing.T) {
	r := runBuiltin(t, "seq", "5", "1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestSeqZeroIncrement(t *testing.T) {
	r := runBuiltin(t, "seq", "1", "0", "5")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Zero increment")
}

func TestSeqNaNArgumentRejected(t *testing.T) {
	r := runBuiltin(t, "seq", "nan")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not-a-number")
}

func TestSeqInvalidNumber(t *testing.T) {
	r := runBuiltin(t, "seq", "abc")
	assertExitCode(t, r, 1)
}

func TestSeqMissingOperand(t *testing.T) {
	r := runBuiltin(t, "seq")
	assertExitCode(t, r, 1)
}

func TestSeqPadOneArg(t *testing.T) {
	r := runBuiltin(t, "seq", "-w", "003")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "001\n002\n003\n")
}

func TestSeqPadTwoArgs(t *testing.T) {
	r := runBuiltin(t, "seq", "-w", "005", "7")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "005\n006\n007\n")
}

func TestSeqPadCountDown(t *testing.T) {
	r := runBuiltin(t, "seq", "-w", "8", "-3", "04")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "08\n05\n")
}

func TestSeqTerminator(t *testing.T) {
	r := runBuiltin(t, "seq", "-t", "", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n2\n3")
}

func TestSeqTerminatorCustom(t *testing.T) {
	r := runBuiltin(t, "seq", "-t", ";\n", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n2\n3;\n")
}

func TestSeqHexInput(t *testing.T) {
	r := runBuiltin(t, "seq", "0xa")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n")
}

func TestSeqHexRange(t *testing.T) {
	r := runBuiltin(t, "seq", "0x8", "0xa")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "8\n9\n10\n")
}

// TestSeqPrecisionIgnoresLastOperand verifies that decimal precision comes
// only from FIRST and INCREMENT, never from LAST, matching GNU seq.c's
// "prec = MAX(first.precision, step.precision)".
func TestSeqPrecisionIgnoresLastOperand(t *testing.T) {
	r := runBuiltin(t, "seq", "3", ".30", "4.000")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "3.00\n3.30\n3.60\n3.90\n")
}

// TestSeqEqualWidthUsesFirstAndLastLiteralWidth verifies that -w pads to a
// width derived from the first/last operands' own literal width (adjusted for
// the shared precision), not simply the longest operand string.
func TestSeqEqualWidthUsesFirstAndLastLiteralWidth(t *testing.T) {
	r := runBuiltin(t, "seq", "-w", "03", ".3", "0004")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "0003.0\n0003.3\n0003.6\n0003.9\n")
}

// TestSeqWriteErrorStopsImmediately verifies seq detects an output write
// error (e.g. writing to a full device) and exits 1 right away, instead of
// silently ignoring it and looping forever (or until timeout) on an
// effectively unbounded sequence.
func TestSeqWriteErrorStopsImmediately(t *testing.T) {
	cmd, ok := Lookup("seq")
	if !ok {
		t.Fatal("seq not registered")
	}
	var stderr bytes.Buffer
	ctx := &Context{
		Args:   []string{"1", "inf"},
		Width:  80,
		Height: 24,
		Stdout: failingWriter{},
		Stderr: &stderr,
		Stdin:  strings.NewReader(""),
	}
	code := cmd.Run(ctx)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (stderr: %s)", code, stderr.String())
	}
}
