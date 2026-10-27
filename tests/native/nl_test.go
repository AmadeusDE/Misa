package native

import "testing"

func TestNl_Basic(t *testing.T) {
	r := runStdin(t, "nl", "a\nb\nc\n")
	wantSuccess(t, r)
	wantOutContains(t, r, "1")
	wantOutContains(t, r, "a")
}

func TestNl_AllLines(t *testing.T) {
	r := runStdin(t, "nl", "a\nb\n\nc\n", "-ba")
	wantSuccess(t, r)
	wantOutContains(t, r, "3")
}

func TestNl_NoBlank(t *testing.T) {
	r := runStdin(t, "nl", "a\n\nb\n", "-bt")
	wantSuccess(t, r)
	wantOutContains(t, r, "1")
	wantOutContains(t, r, "2")
}

func TestNl_Increment(t *testing.T) {
	r := runStdin(t, "nl", "a\nb\nc\n", "-i", "5")
	wantSuccess(t, r)
	wantOutContains(t, r, "1")
	wantOutContains(t, r, "6")
	wantOutContains(t, r, "11")
}

func TestNl_StartingLine(t *testing.T) {
	r := runStdin(t, "nl", "a\nb\n", "-v", "10")
	wantSuccess(t, r)
	wantOutContains(t, r, "10")
	wantOutContains(t, r, "11")
}

func TestNl_Width(t *testing.T) {
	r := runStdin(t, "nl", "a\n", "-w", "3")
	wantSuccess(t, r)
	wantOutContains(t, r, "  1")
}

func TestNl_Separator(t *testing.T) {
	r := runStdin(t, "nl", "a\n", "-s", ": ")
	wantSuccess(t, r)
	wantOutContains(t, r, ": a")
}

func TestNl_FormatLN(t *testing.T) {
	r := runStdin(t, "nl", "a\n", "-nln")
	wantSuccess(t, r)
	wantOutContains(t, r, "1     ")
}

func TestNl_FormatRZ(t *testing.T) {
	r := runStdin(t, "nl", "a\n", "-nrz")
	wantSuccess(t, r)
	wantOutContains(t, r, "000001")
}

func TestNl_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "line1\nline2\n")
	r := runIn(t, dir, "nl", "f.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "line1")
	wantOutContains(t, r, "line2")
}

func TestNl_SectionDelimiter(t *testing.T) {
	r := runStdin(t, "nl", "a\n\\:\\:\nb\n", "-d", `\:`)
	wantSuccess(t, r)
	wantOut(t, r, "     1\ta\n\n     1\tb\n")
}

func TestNl_CustomSectionDelimiter(t *testing.T) {
	r := runStdin(t, "nl", "a\nabcabc\nb\n", "-d", "abc")
	wantSuccess(t, r)
	wantOut(t, r, "     1\ta\n\n     1\tb\n")
}

func TestNl_InvalidWidth(t *testing.T) {
	r := runStdin(t, "nl", "a\n", "-w", "0")
	wantCode(t, r, 1)
	wantErrContains(t, r, "nl:")
}
