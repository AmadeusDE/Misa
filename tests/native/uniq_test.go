package native

import (
	"testing"
)

func TestUniq_CLocaleWidthUsesBytes(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	r := runStdin(t, "uniq", "가나다라마\n가나다바사\n", "-w", "4")
	wantSuccess(t, r)
	wantOut(t, r, "가나다라마\n")
}

func TestUniq_Basic(t *testing.T) {
	r := runStdin(t, "uniq", "a\na\nb\nb\nc\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\nc\n")
}

func TestUniq_Count(t *testing.T) {
	r := runStdin(t, "uniq", "a\na\nb\nc\nc\nc\n", "-c")
	wantSuccess(t, r)
	wantOutContains(t, r, "2 a")
	wantOutContains(t, r, "1 b")
	wantOutContains(t, r, "3 c")
}

func TestUniq_Repeated(t *testing.T) {
	r := runStdin(t, "uniq", "a\na\nb\nc\nc\n", "-d")
	wantSuccess(t, r)
	wantOut(t, r, "a\nc\n")
}

func TestUniq_Unique(t *testing.T) {
	r := runStdin(t, "uniq", "a\na\nb\nc\nc\n", "-u")
	wantSuccess(t, r)
	wantOut(t, r, "b\n")
}

func TestUniq_CaseInsensitive(t *testing.T) {
	r := runStdin(t, "uniq", "Hello\nhello\nWorld\n", "-i")
	wantSuccess(t, r)
	wantOut(t, r, "Hello\nWorld\n")
}

func TestUniq_NoAdjacentDupes(t *testing.T) {
	r := runStdin(t, "uniq", "a\nb\na\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\na\n")
}

func TestUniq_Empty(t *testing.T) {
	r := runStdin(t, "uniq", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestUniq_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "x\nx\ny\n")
	r := runIn(t, dir, "uniq", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "x\ny\n")
}

func TestUniq_InvalidOption(t *testing.T) {
	r := runStdin(t, "uniq", "a\na\n", "-5q")
	wantFail(t, r)
	wantErrContains(t, r, "unknown option")
}

func TestUniq_AllRepeatedOverride(t *testing.T) {
	r := runStdin(t, "uniq", "copper\ncopper\ngold\ngold\n", "--all-repeated=prepend", "-D")
	wantSuccess(t, r)
	wantOut(t, r, "copper\ncopper\ngold\ngold\n")
}
