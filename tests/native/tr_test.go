package native

import "testing"

func TestTr_SimpleReplace(t *testing.T) {
	r := runStdin(t, "tr", "hello", "aeiou", "AEIOU")
	wantSuccess(t, r)
	wantOut(t, r, "hEllO")
}

func TestTr_Delete(t *testing.T) {
	r := runStdin(t, "tr", "hello world", "-d", "lo")
	wantSuccess(t, r)
	wantOut(t, r, "he wrd")
}

func TestTr_Squeeze(t *testing.T) {
	r := runStdin(t, "tr", "aaabbbccc", "-s", "abc")
	wantSuccess(t, r)
	wantOut(t, r, "abc")
}

func TestTr_DeleteSqueeze(t *testing.T) {
	r := runStdin(t, "tr", "aaabbbccc", "-ds", "a", "b")
	wantSuccess(t, r)
	wantOut(t, r, "bccc")
}

func TestTr_InvalidClass(t *testing.T) {
	r := runStdin(t, "tr", "hello", "[:lowre:]", "x")
	wantFail(t, r)
	wantErrContains(t, r, "invalid character class")
}

func TestTr_InvalidRepeatCount(t *testing.T) {
	r := runStdin(t, "tr", "hello", "a", "[b*c]")
	wantFail(t, r)
	wantErrContains(t, r, "invalid repeat count")
}

func TestTr_RangeReplace(t *testing.T) {
	r := runStdin(t, "tr", "abc", "a-c", "A-C")
	wantSuccess(t, r)
	wantOut(t, r, "ABC")
}

func TestTr_Upper(t *testing.T) {
	r := runStdin(t, "tr", "hello", "[:lower:]", "[:upper:]")
	wantSuccess(t, r)
	wantOut(t, r, "HELLO")
}

func TestTr_Lower(t *testing.T) {
	r := runStdin(t, "tr", "HELLO", "[:upper:]", "[:lower:]")
	wantSuccess(t, r)
	wantOut(t, r, "hello")
}

func TestTr_Complement(t *testing.T) {
	r := runStdin(t, "tr", "hello 123 world", "-cd", "[:alpha:]")
	wantSuccess(t, r)
	wantOut(t, r, "helloworld")
}

func TestTr_SqueezeSpaces(t *testing.T) {
	r := runStdin(t, "tr", "hello   world", "-s", " ")
	wantSuccess(t, r)
	wantOut(t, r, "hello world")
}

func TestTr_DeleteNewlines(t *testing.T) {
	r := runStdin(t, "tr", "a\nb\nc\n", "-d", "\n")
	wantSuccess(t, r)
	wantOut(t, r, "abc")
}

func TestTr_DigitToLetter(t *testing.T) {
	r := runStdin(t, "tr", "123", "0-9", "a-j")
	wantSuccess(t, r)
	wantOut(t, r, "bcd")
}

func TestTr_TruncateOneArgIsIdentity(t *testing.T) {
	r := runStdin(t, "tr", "1234", "-t", "1234")
	wantSuccess(t, r)
	wantOut(t, r, "1234")
}

func TestTr_ComplementUpperFlag(t *testing.T) {
	r := runStdin(t, "tr", "hello 123 world", "-Cd", "[:alpha:]")
	wantSuccess(t, r)
	wantOut(t, r, "helloworld")
}

func TestTr_EscapedRange(t *testing.T) {
	r := runStdin(t, "tr", "A0B", "-c", "\\0-@", "*")
	wantSuccess(t, r)
	wantOut(t, r, "*0*")
}
