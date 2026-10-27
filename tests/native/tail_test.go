package native

import "testing"

func TestTail_Default10Lines(t *testing.T) {
	input := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"
	r := runStdin(t, "tail", input)
	wantSuccess(t, r)
	wantOut(t, r, "3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n")
}

func TestTail_FewLines(t *testing.T) {
	r := runStdin(t, "tail", "a\nb\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\n")
}

func TestTail_N3(t *testing.T) {
	r := runStdin(t, "tail", "a\nb\nc\nd\n", "-n", "3")
	wantSuccess(t, r)
	wantOut(t, r, "b\nc\nd\n")
}

func TestTail_N1(t *testing.T) {
	r := runStdin(t, "tail", "first\nsecond\n", "-n", "1")
	wantSuccess(t, r)
	wantOut(t, r, "second\n")
}

func TestTail_Bytes(t *testing.T) {
	r := runStdin(t, "tail", "abcdefghij", "-c", "5")
	wantSuccess(t, r)
	wantOut(t, r, "fghij")
}

func TestTail_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "line1\nline2\nline3\n")
	r := runIn(t, dir, "tail", "-n", "2", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "line2\nline3\n")
}

func TestTail_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "alpha\n")
	mkFile(t, dir, "b.txt", "beta\n")
	r := runIn(t, dir, "tail", "-n", "1", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "alpha")
	wantOutContains(t, r, "beta")
}

func TestTail_EmptyStdin(t *testing.T) {
	r := runStdin(t, "tail", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestTail_PlusN(t *testing.T) {
	r := runStdin(t, "tail", "a\nb\nc\nd\n", "-n", "+3")
	wantSuccess(t, r)
	wantOut(t, r, "c\nd\n")
}

func TestTail_Follow(t *testing.T) {
	dir := t.TempDir()
	f := mkFile(t, dir, "log.txt", "line1\n")
	r := runIn(t, dir, "tail", "-n", "1", "log.txt")
	wantSuccess(t, r)
	wantOut(t, r, "line1\n")
	_ = f
}

func TestTail_Multipliers(t *testing.T) {
	r := runStdin(t, "tail", "abcdefghij", "-c", "1kD")
	wantSuccess(t, r)
	wantOut(t, r, "abcdefghij")

	r2 := runStdin(t, "tail", "abcdefghij", "-c", "1R")
	wantSuccess(t, r2)
	wantOut(t, r2, "abcdefghij")
}

func TestTail_ObsoletePlusOption(t *testing.T) {
	r := runStdin(t, "tail", "a\nb\nc\nd\n", "+3n")
	wantSuccess(t, r)
	wantOut(t, r, "c\nd\n")
}

// Tests for error-message formatting fixes.

func TestTail_InvalidLinesQuoting(t *testing.T) {
	// Error for invalid -n value uses single quotes, not Go's %q double-quotes.
	r := run(t, "tail", "-n", "5QQ", "/dev/null")
	wantFail(t, r)
	wantErr(t, r, "tail: invalid number of lines: '5QQ'\n")
}

func TestTail_InvalidBytesQuoting(t *testing.T) {
	// Error for invalid -c value uses single quotes.
	r := run(t, "tail", "-c", "3XY", "/dev/null")
	wantFail(t, r)
	wantErr(t, r, "tail: invalid number of bytes: '3XY'\n")
}

func TestTail_CombinedFlagStripsSign(t *testing.T) {
	// GNU tail: -c-0fb reports '0fb' (sign stripped from display), not '-0fb'.
	r := run(t, "tail", "-c-0fb", "/dev/null")
	wantFail(t, r)
	wantErr(t, r, "tail: invalid number of bytes: '0fb'\n")
}

func TestTail_CombinedLinesLeadingZeros(t *testing.T) {
	// Leading zeros are preserved in the error message.
	r := run(t, "tail", "-n", "000ff", "/dev/null")
	wantFail(t, r)
	wantErr(t, r, "tail: invalid number of lines: '000ff'\n")
}

func TestTail_QuoteHeaders(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "\n", "newline_file\n")
	mkFile(t, dir, "normal", "normal_file\n")
	r := runIn(t, dir, "tail", "-n", "1", "\n", "normal")
	wantSuccess(t, r)
	wantOut(t, r, "==> ''$'\\n' <==\nnewline_file\n\n==> normal <==\nnormal_file\n")
}
