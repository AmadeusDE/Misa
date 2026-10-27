package native

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/builtins"
)

func TestWc_EmptyStdin(t *testing.T) {
	r := runStdin(t, "wc", "")
	wantSuccess(t, r)
	wantOut(t, r, "      0       0       0\n")
}

func TestWc_Stdin(t *testing.T) {
	r := runStdin(t, "wc", "a b\nc")
	wantSuccess(t, r)
	wantOut(t, r, "      1       3       5\n")
}

func TestWc_StdinCharsOnly(t *testing.T) {
	r := runStdin(t, "wc", "a b\nc", "-c")
	wantSuccess(t, r)
	wantOut(t, r, "5\n")
}

func TestWc_StdinLinesChars(t *testing.T) {
	r := runStdin(t, "wc", "a b\nc", "-cl")
	wantSuccess(t, r)
	wantOut(t, r, "      1       5\n")
}

func TestWc_MaxLineLength(t *testing.T) {
	r := runStdin(t, "wc", "a\tb\n", "-L")
	wantSuccess(t, r)
	wantOut(t, r, "9\n")
}

func TestWc_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello world\nbye\n")
	r := runIn(t, dir, "wc", "f.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "f.txt")
}

func TestWc_FileBytesOnly(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "some words\t. \n\nsome\nlines\n")
	r := runIn(t, dir, "wc", "-c", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "26 f.txt\n")
}

func TestWc_FileLinesOnly(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "some words\t. \n\nsome\nlines\n")
	r := runIn(t, dir, "wc", "-l", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "4 f.txt\n")
}

func TestWc_FileWordsOnly(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "some words\t. \n\nsome\nlines\n")
	r := runIn(t, dir, "wc", "-w", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "5 f.txt\n")
}

func TestWc_MaxLineLen(t *testing.T) {
	r := runStdin(t, "wc", "a\n", "-L")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestWc_MaxLineLenNoNewline(t *testing.T) {
	r := runStdin(t, "wc", "a", "-L")
	wantSuccess(t, r)
	wantOut(t, r, "1\n")
}

func TestWc_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "one\n")
	mkFile(t, dir, "b.txt", "two\n")
	r := runIn(t, dir, "wc", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "total")
}

func TestWc_MultipleFilesColumnWidth(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input", "a\nb")
	mkFile(t, dir, "file1", "some words\t. \n\nsome\nlines\n")
	r := runStdinIn(t, dir, "wc", "a b", "input", "-", "file1")
	wantSuccess(t, r)
	wantOut(t, r, "      1       2       3 input\n      0       2       3 -\n      4       5      26 file1\n      5       9      32 total\n")
}

func TestWc_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "empty.txt", "")
	r := runIn(t, dir, "wc", "empty.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "0")
}

func TestWc_CharCountUTF8(t *testing.T) {
	r := runStdin(t, "wc", "hello, 世界!\n", "-m")
	wantSuccess(t, r)
	wantOut(t, r, "11\n")
}

func TestWc_CharCountASCII(t *testing.T) {
	r := runStdin(t, "wc", "hello\n", "-m")
	wantSuccess(t, r)
	wantOut(t, r, "6\n")
}

func TestWc_CharCountMultibyteFile(t *testing.T) {
	dir := t.TempDir()
	content := " "
	for i := 0; i < 16; i++ {
		content += "üüüüüüüüüüüüüüüü"
	}
	mkFile(t, dir, "utf.txt", content)
	r := runIn(t, dir, "wc", "-m", "utf.txt")
	wantSuccess(t, r)
	wantOut(t, r, "257 utf.txt\n")
}

func TestWc_Files0From(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello\n")
	mkFile(t, dir, "list.txt", "a.txt\x00")
	r := runIn(t, dir, "wc", "--files0-from=list.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "a.txt")
}

func TestWc_MaxLineLenControlChars(t *testing.T) {
	r1 := runStdin(t, "wc", "\t\b\b\b\n", "-L")
	wantSuccess(t, r1)
	wantOut(t, r1, "8\n")

	r2 := runStdin(t, "wc", "abc\bd\n", "-L")
	wantSuccess(t, r2)
	wantOut(t, r2, "4\n")

	r3 := runStdin(t, "wc", "\x01\n", "-L")
	wantSuccess(t, r3)
	wantOut(t, r3, "0\n")
}

func TestWc_PosixlyCorrect(t *testing.T) {
	cmd, ok := builtins.Lookup("wc")
	if !ok {
		t.Fatal("wc not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := &builtins.Context{
		Args:   []string{"-w"},
		Env:    []string{"POSIXLY_CORRECT=1"},
		Stdout: &stdout,
		Stderr: &stderr,
		Stdin:  strings.NewReader("word\u00A0word"),
	}
	code := cmd.Run(ctx)
	if code != 0 {
		t.Fatalf("exit code %d, want 0", code)
	}
	if stdout.String() != "1\n" {
		t.Errorf("got %q, want %q", stdout.String(), "1\n")
	}
}

func TestWc_FilenameQuoting(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a\nb", "test\n")
	r := runIn(t, dir, "wc", "a\nb")
	wantSuccess(t, r)
	wantOutContains(t, r, "'a'$'\\n''b'")
}

func TestWc_ReadDirectoryError(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "wc", ".")
	wantFail(t, r)
	wantErrContains(t, r, "Is a directory")
}
