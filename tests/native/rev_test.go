package native

import "testing"

func TestRev_Basic(t *testing.T) {
	r := runStdin(t, "rev", "hello\n")
	wantSuccess(t, r)
	wantOut(t, r, "olleh\n")
}

func TestRev_MultipleLines(t *testing.T) {
	r := runStdin(t, "rev", "abc\ndef\n")
	wantSuccess(t, r)
	wantOut(t, r, "cba\nfed\n")
}

func TestRev_EmptyLine(t *testing.T) {
	r := runStdin(t, "rev", "\n")
	wantSuccess(t, r)
	wantOut(t, r, "\n")
}

func TestRev_NoNewline(t *testing.T) {
	r := runStdin(t, "rev", "abc")
	wantSuccess(t, r)
	wantOut(t, r, "cba")
}

func TestRev_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello\nworld\n")
	r := runIn(t, dir, "rev", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "olleh\ndlrow\n")
}

func TestRev_Empty(t *testing.T) {
	r := runStdin(t, "rev", "")
	wantSuccess(t, r)
}

func TestRev_Spaces(t *testing.T) {
	r := runStdin(t, "rev", "a b c\n")
	wantSuccess(t, r)
	wantOut(t, r, "c b a\n")
}

func TestRev_DoubleDash(t *testing.T) {
	r := runStdin(t, "rev", "hello\n", "--")
	wantSuccess(t, r)
	wantOut(t, r, "olleh\n")
}

func TestRev_MissingFileContinues(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file1", "one\n")
	mkFile(t, dir, "file2", "two\n")
	r := runIn(t, dir, "rev", "file1", "notfound", "file2")
	wantFail(t, r)
	wantOut(t, r, "eno\nowt\n")
	wantErrContains(t, r, "notfound: No such file or directory")
}
