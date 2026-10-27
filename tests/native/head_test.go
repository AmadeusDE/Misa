package native

import "testing"

func TestHead_Default10Lines(t *testing.T) {
	input := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"
	r := runStdin(t, "head", input)
	wantSuccess(t, r)
	wantOut(t, r, "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n")
}

func TestHead_FewLines(t *testing.T) {
	r := runStdin(t, "head", "a\nb\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\n")
}

func TestHead_N3(t *testing.T) {
	r := runStdin(t, "head", "a\nb\nc\nd\n", "-n", "3")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\nc\n")
}

func TestHead_N1(t *testing.T) {
	r := runStdin(t, "head", "first\nsecond\n", "-n", "1")
	wantSuccess(t, r)
	wantOut(t, r, "first\n")
}

func TestHead_Bytes(t *testing.T) {
	r := runStdin(t, "head", "abcdefghij", "-c", "5")
	wantSuccess(t, r)
	wantOut(t, r, "abcde")
}

func TestHead_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "line1\nline2\nline3\n")
	r := runIn(t, dir, "head", "-n", "2", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "line1\nline2\n")
}

func TestHead_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "alpha\n")
	mkFile(t, dir, "b.txt", "beta\n")
	r := runIn(t, dir, "head", "-n", "1", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "alpha")
	wantOutContains(t, r, "beta")
}

func TestHead_EmptyStdin(t *testing.T) {
	r := runStdin(t, "head", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestHead_InvalidByteCountPreservesLeadingZeros(t *testing.T) {
	r := runStdin(t, "head", "", "-c", "0fb")
	wantCode(t, r, 1)
	wantErr(t, r, "head: invalid number of bytes: '0fb'\n")
}

func TestHead_InvalidLineCountPreservesLeadingZeros(t *testing.T) {
	r := runStdin(t, "head", "", "-n", "00x")
	wantCode(t, r, 1)
	wantErr(t, r, "head: invalid number of lines: '00x'\n")
}

func TestHead_LowercaseSuffixRejected(t *testing.T) {
	r := runStdin(t, "head", "abcdefghij\n", "-c", "2g")
	wantCode(t, r, 1)
	wantErrContains(t, r, "invalid number of bytes: '2g'")
}

func TestHead_N0_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "head", "-n", "0", "missing1", "missing2")
	wantCode(t, r, 1)
	wantOut(t, r, "")
	wantErrContains(t, r, "head: cannot open 'missing1' for reading:")
	wantErrContains(t, r, "head: cannot open 'missing2' for reading:")
}

func TestHead_QuoteHeaders(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "\n", "newline_file\n")
	mkFile(t, dir, "normal", "normal_file\n")
	r := runIn(t, dir, "head", "-n", "1", "\n", "normal")
	wantSuccess(t, r)
	wantOut(t, r, "==> ''$'\\n' <==\nnewline_file\n\n==> normal <==\nnormal_file\n")
}
