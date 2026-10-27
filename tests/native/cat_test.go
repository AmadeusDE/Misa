package native

import (
	"strings"
	"testing"
)

func TestCat_SimpleFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello\n")
	r := runIn(t, dir, "cat", "a.txt")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestCat_TwoFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "one\n")
	mkFile(t, dir, "b.txt", "two\n")
	r := runIn(t, dir, "cat", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "one\ntwo\n")
}

func TestCat_StdinDash(t *testing.T) {
	r := runStdin(t, "cat", "hello\n", "-")
	wantSuccess(t, r)
	wantOut(t, r, "hello\n")
}

func TestCat_StdinThenFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "file\n")
	r := runStdinIn(t, dir, "cat", "stdin\n", "-", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "stdin\nfile\n")
}

func TestCat_FileThenStdin(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "file\n")
	r := runStdinIn(t, dir, "cat", "stdin\n", "f.txt", "-")
	wantSuccess(t, r)
	wantOut(t, r, "file\nstdin\n")
}

func TestCat_NumberLines(t *testing.T) {
	r := runStdin(t, "cat", "a\nb\nc\n", "-n")
	wantSuccess(t, r)
	wantOut(t, r, "     1\ta\n     2\tb\n     3\tc\n")
}

func TestCat_NumberNonblank(t *testing.T) {
	r := runStdin(t, "cat", "a\n\nb\n", "-b")
	wantSuccess(t, r)
	wantOut(t, r, "     1\ta\n\n     2\tb\n")
}

func TestCat_SqueezeBlank(t *testing.T) {
	r := runStdin(t, "cat", "a\n\n\n\nb\n", "-s")
	wantSuccess(t, r)
	wantOut(t, r, "a\n\nb\n")
}

func TestCat_ShowEnds(t *testing.T) {
	r := runStdin(t, "cat", "line\n", "-E")
	wantSuccess(t, r)
	wantOut(t, r, "line$\n")
}

func TestCat_ShowTabs(t *testing.T) {
	r := runStdin(t, "cat", "a\tb\n", "-T")
	wantSuccess(t, r)
	wantOut(t, r, "a^Ib\n")
}

func TestCat_MissingFile(t *testing.T) {
	r := run(t, "cat", "nonexistent")
	wantFail(t, r)
	wantErrContains(t, r, "nonexistent")
}

func TestCat_MissingFileContinues(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "one\n")
	mkFile(t, dir, "b.txt", "two\n")
	r := runIn(t, dir, "cat", "a.txt", "nonexistent", "b.txt")
	wantCode(t, r, 1)
	if !strings.Contains(r.stdout, "one\n") || !strings.Contains(r.stdout, "two\n") {
		t.Errorf("expected output from valid files even with missing file in between, got %q", r.stdout)
	}
}

func TestCat_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "empty.txt", "")
	r := runIn(t, dir, "cat", "empty.txt")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestCat_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "no newline")
	r := runIn(t, dir, "cat", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "no newline")
}

func TestCat_ShowEndsNoTrailingNewline(t *testing.T) {
	r := runStdin(t, "cat", "line", "-E")
	wantSuccess(t, r)
	wantOut(t, r, "line")
}

func TestCat_ErrorMessageFormat(t *testing.T) {
	r := run(t, "cat", "nosuchfile")
	wantCode(t, r, 1)
	wantErrContains(t, r, "cat: nosuchfile: No such file or directory")
}

func TestCat_ShowEndsCRLF(t *testing.T) {
	r := runStdin(t, "cat", "b\r\n", "-E")
	wantSuccess(t, r)
	wantOut(t, r, "b^M$\n")
}

func TestCat_ShowEndsSpanningFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "in2", "1\r")
	mkFile(t, dir, "in2b", "\n2\r\n")
	r := runIn(t, dir, "cat", "-E", "in2", "in2b")
	wantSuccess(t, r)
	wantOut(t, r, "1^M$\n2^M$\n")
}

func TestCat_ShowEndsCRLF_MultipleLines(t *testing.T) {
	r := runStdin(t, "cat", "1\r\n2\r\n", "-E")
	wantSuccess(t, r)
	wantOut(t, r, "1^M$\n2^M$\n")
}
