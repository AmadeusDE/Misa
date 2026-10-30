package native

import "testing"

func TestOd_Default(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.bin", "0123456789")
	r := runIn(t, dir, "od", "a.bin")
	wantSuccess(t, r)
	wantOut(t, r, "0000000 030460 031462 032464 033466 034470\n0000012\n")
}

func TestOd_Char(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.bin", "abc\n")
	r := runIn(t, dir, "od", "-c", "a.bin")
	wantSuccess(t, r)
	wantOut(t, r, "0000000   a   b   c  \\n\n0000004\n")
}

func TestOd_NoAddress(t *testing.T) {
	r := runStdin(t, "od", "0123", "-An", "-t", "x1")
	wantSuccess(t, r)
	wantOut(t, r, " 30 31 32 33\n")
}

func TestOd_SkipLimit(t *testing.T) {
	r := runStdin(t, "od", "0123456789", "-An", "-c", "-j", "2", "-N", "3")
	wantSuccess(t, r)
	wantOut(t, r, "   2   3   4\n")
}

func TestOd_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.bin", "abc")
	mkFile(t, dir, "b.bin", "defgh")
	r := runIn(t, dir, "od", "-An", "-c", "a.bin", "b.bin")
	wantSuccess(t, r)
	wantOut(t, r, "   a   b   c   d   e   f   g   h\n")
}

func TestOd_SuppressedDuplicates(t *testing.T) {
	r := runStdin(t, "od", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "-An", "-c")
	wantSuccess(t, r)
	wantOut(t, r, "   A   A   A   A   A   A   A   A   A   A   A   A   A   A   A   A\n*\n")
}

func TestOd_VerboseDuplicates(t *testing.T) {
	r := runStdin(t, "od", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "-An", "-c", "-v")
	wantSuccess(t, r)
	wantOutContains(t, r, "   A   A   A   A   A   A   A   A   A   A   A   A   A   A   A   A\n   A   A")
}

func TestOd_MissingFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.bin", "abc")
	r := runIn(t, dir, "od", "-An", "-c", "nope", "a.bin")
	wantFail(t, r)
	wantErrContains(t, r, "nope")
	wantOut(t, r, "   a   b   c\n")
}

func TestOd_UnknownOption(t *testing.T) {
	r := run(t, "od", "-S3")
	wantFail(t, r)
	wantErrContains(t, r, "unknown option")
}

func TestOd_InvalidType(t *testing.T) {
	r := run(t, "od", "-t", "q")
	wantFail(t, r)
	wantErrContains(t, r, "invalid character")
}

func TestOd_HelpAndVersion(t *testing.T) {
	r := run(t, "od", "--help")
	wantSuccess(t, r)
	wantOutContains(t, r, "Usage: od")
	wantOutContains(t, r, "--address-radix")

	r = run(t, "od", "--version")
	wantSuccess(t, r)
	wantOutContains(t, r, "od (Misa)")

	r = run(t, "od", "-h")
	wantSuccess(t, r)
	wantOutContains(t, r, "Usage: od")
}
