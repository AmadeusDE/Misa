package native

import "testing"

func TestPaste_TwoFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "1\n2\n3\n")
	mkFile(t, dir, "b.txt", "a\nb\nc\n")
	r := runIn(t, dir, "paste", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "1\ta\n2\tb\n3\tc\n")
}

func TestPaste_Delim(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "1\n2\n")
	mkFile(t, dir, "b.txt", "a\nb\n")
	r := runIn(t, dir, "paste", "-d:", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "1:a\n2:b\n")
}

func TestPaste_Serial(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "1\n2\n3\n")
	r := runIn(t, dir, "paste", "-s", "a.txt")
	wantSuccess(t, r)
	wantOut(t, r, "1\t2\t3\n")
}

func TestPaste_StdinDash(t *testing.T) {
	r := runStdin(t, "paste", "a\nb\nc\n", "-s", "-")
	wantSuccess(t, r)
	wantOut(t, r, "a\tb\tc\n")
}

func TestPaste_UnequalLength(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "1\n2\n3\n")
	mkFile(t, dir, "b.txt", "a\n")
	r := runIn(t, dir, "paste", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "1\ta\n2\t\n3\t\n")
}

func TestPaste_MultibyteDelimiter(t *testing.T) {
	r := runStdin(t, "paste", "one\ntwo\nthree\nfour\n", "-d", "私", "-", "-")
	wantSuccess(t, r)
	wantOut(t, r, "one私two\nthree私four\n")
}

func TestPaste_MixedEmptyDelimiter(t *testing.T) {
	r := runStdin(t, "paste", "a\nb\nc\nd\n", "-d", `x\0y`, "-", "-", "-", "-")
	wantSuccess(t, r)
	wantOut(t, r, "axbcyd\n")
}

func TestPaste_CombiningCharacterDelimiter(t *testing.T) {
	r := runStdin(t, "paste", "1\n2\n3\n4\n", "-d", "私\\0\xcc\xb4\xcc\x97\xcc\x9e\xcc\xa0eq", "-", "-", "-", "-")
	wantSuccess(t, r)
	wantOut(t, r, "1私23\xcc\xb4\xcc\x97\xcc\x9e\xcc\xa0e4\n")
}
