package native

import "testing"

func TestCut_ByteRange(t *testing.T) {
	r := runStdin(t, "cut", "abcdef\n", "-b", "1-3")
	wantSuccess(t, r)
	wantOut(t, r, "abc\n")
}

func TestCut_ByteList(t *testing.T) {
	r := runStdin(t, "cut", "abcdef\n", "-b", "1,3,5")
	wantSuccess(t, r)
	wantOut(t, r, "ace\n")
}

func TestCut_ByteFromEnd(t *testing.T) {
	r := runStdin(t, "cut", "abcdef\n", "-b", "3-")
	wantSuccess(t, r)
	wantOut(t, r, "cdef\n")
}

func TestCut_ByteFromStart(t *testing.T) {
	r := runStdin(t, "cut", "abcdef\n", "-b", "-3")
	wantSuccess(t, r)
	wantOut(t, r, "abc\n")
}

func TestCut_CharRange(t *testing.T) {
	r := runStdin(t, "cut", "abcdef\n", "-c", "2-4")
	wantSuccess(t, r)
	wantOut(t, r, "bcd\n")
}

func TestCut_CharMultibyte(t *testing.T) {
	r := runStdin(t, "cut", "ábćd\n", "-c", "1-2")
	wantSuccess(t, r)
	wantOut(t, r, "áb\n")
}

func TestCut_FieldTab(t *testing.T) {
	r := runStdin(t, "cut", "a\tb\tc\n", "-f", "2")
	wantSuccess(t, r)
	wantOut(t, r, "b\n")
}

func TestCut_FieldCustomDelim(t *testing.T) {
	r := runStdin(t, "cut", "a:b:c\n", "-d:", "-f", "2")
	wantSuccess(t, r)
	wantOut(t, r, "b\n")
}

func TestCut_FieldRange(t *testing.T) {
	r := runStdin(t, "cut", "a:b:c:d\n", "-d:", "-f", "2-3")
	wantSuccess(t, r)
	wantOut(t, r, "b:c\n")
}

func TestCut_FieldOnlyDelimited(t *testing.T) {
	r := runStdin(t, "cut", "no delim\nhas\ttab\n", "-f", "1", "-s")
	wantSuccess(t, r)
	wantOut(t, r, "has\n")
}

func TestCut_FieldComplement(t *testing.T) {
	r := runStdin(t, "cut", "a\tb\tc\n", "-f", "2", "--complement")
	wantSuccess(t, r)
	wantOut(t, r, "a\tc\n")
}

func TestCut_MultipleLines(t *testing.T) {
	r := runStdin(t, "cut", "aaa:bbb\nccc:ddd\n", "-d:", "-f", "1")
	wantSuccess(t, r)
	wantOut(t, r, "aaa\nccc\n")
}

func TestCut_OutputDelim(t *testing.T) {
	r := runStdin(t, "cut", "a:b:c\n", "-d:", "-f", "1,3", "--output-delimiter= ")
	wantSuccess(t, r)
	wantOut(t, r, "a c\n")
}

func TestCut_NoDelimInLine(t *testing.T) {
	r := runStdin(t, "cut", "nope\n", "-d:", "-f", "1")
	wantSuccess(t, r)
	wantOut(t, r, "nope\n")
}

func TestCut_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "a:b:c\nd:e:f\n")
	r := runIn(t, dir, "cut", "-d:", "-f", "2", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "b\ne\n")
}

func TestCut_EmptyField(t *testing.T) {
	r := runStdin(t, "cut", ":::\n", "-d:", "-f", "2")
	wantSuccess(t, r)
	wantOut(t, r, "\n")
}

func TestCut_WhitespaceDelimited(t *testing.T) {
	r := runStdin(t, "cut", "   alpha beta\n", "--whitespace-delimited", "-f1,2")
	wantSuccess(t, r)
	wantOut(t, r, "\talpha\n")

	r2 := runStdin(t, "cut", "  hello world  \n", "--whitespace-delimited=trimmed", "-f1,2")
	wantSuccess(t, r2)
	wantOut(t, r2, "hello\tworld\n")
}

func TestCut_NewlineDelimiter(t *testing.T) {
	r := runStdin(t, "cut", "line1\nline2\nline3\n", "-d", "\n", "-f", "2", "--complement")
	wantSuccess(t, r)
	wantOut(t, r, "line1\nline3\n")
}

func TestCut_ExclusiveOptions(t *testing.T) {
	r := run(t, "cut", "-w", "-d,", "-f3")
	wantCode(t, r, 1)
	wantErrContains(t, r, "mutually exclusive")
}

func TestCut_NoSplitMultibyte(t *testing.T) {
	r := runStdin(t, "cut", "tøys\n", "-b", "3-5", "-n")
	wantSuccess(t, r)
	wantOut(t, r, "øys\n")
}
