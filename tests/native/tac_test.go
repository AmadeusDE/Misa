package native

import "testing"

func TestTac_Basic(t *testing.T) {
	r := runStdin(t, "tac", "a\nb\nc\n")
	wantSuccess(t, r)
	wantOut(t, r, "c\nb\na\n")
}

func TestTac_NoTrailingNewline(t *testing.T) {
	r := runStdin(t, "tac", "a\nb\nc")
	wantSuccess(t, r)
	wantOut(t, r, "cb\na\n")
}

func TestTac_SingleLine(t *testing.T) {
	r := runStdin(t, "tac", "only\n")
	wantSuccess(t, r)
	wantOut(t, r, "only\n")
}

func TestTac_Empty(t *testing.T) {
	r := runStdin(t, "tac", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestTac_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "first\nsecond\nthird\n")
	r := runIn(t, dir, "tac", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "third\nsecond\nfirst\n")
}

func TestTac_CustomSeparator(t *testing.T) {
	r := runStdin(t, "tac", "a:b:c:", "-s", ":")
	wantSuccess(t, r)
	wantOut(t, r, "c:b:a:")
}

func TestTac_Before(t *testing.T) {
	r := runStdin(t, "tac", "a\nb\nc\n", "-b")
	wantSuccess(t, r)
	wantOutContains(t, r, "c")
	wantOutContains(t, r, "b")
	wantOutContains(t, r, "a")
}

func TestTac_MissingFileErrMsg(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "one\n")
	mkFile(t, dir, "b.txt", "two\n")
	r := runIn(t, dir, "tac", "a.txt", "notfound", "b.txt")
	wantCode(t, r, 1)
	wantErrContains(t, r, "tac: notfound: No such file or directory")
	wantOutContains(t, r, "one\n")
	wantOutContains(t, r, "two\n")
}

func TestTac_NullSeparator(t *testing.T) {
	r := runStdin(t, "tac", "a\x00b\x00", "-s", "")
	wantSuccess(t, r)
	wantOut(t, r, "b\x00a\x00")
}

func TestTac_OverlappingSeparator(t *testing.T) {
	r := runStdin(t, "tac", "axxx", "-s", "xx")
	wantSuccess(t, r)
	wantOut(t, r, "axxx")

	r2 := runStdin(t, "tac", "axxxx", "-s", "xx")
	wantSuccess(t, r2)
	wantOut(t, r2, "xxaxx")
}

func TestTac_OverlappingSeparatorBefore(t *testing.T) {
	r := runStdin(t, "tac", "axxx", "-b", "-s", "xx")
	wantSuccess(t, r)
	wantOut(t, r, "xxax")
}

func TestTac_RegexSeparator(t *testing.T) {
	r := runStdin(t, "tac", "axyz", "-r", "-s", "[xyz]+")
	wantSuccess(t, r)
	wantOut(t, r, "zyax")
}

func TestTac_RegexOrOperator(t *testing.T) {
	r := runStdin(t, "tac", "abc", "-r", "-s", `[^x]\|x`)
	wantSuccess(t, r)
	wantOut(t, r, "cba")
}

func TestTac_RegexUnescapedMiddleAnchor(t *testing.T) {
	r := runStdin(t, "tac", "111^222", "-r", "-s", "1^2")
	wantSuccess(t, r)
	wantOut(t, r, "22111^2")
}
