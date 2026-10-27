package native

import "testing"

func TestCmp_Identical(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello\n")
	mkFile(t, dir, "b.txt", "hello\n")
	r := runIn(t, dir, "cmp", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestCmp_Different(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello\n")
	mkFile(t, dir, "b.txt", "world\n")
	r := runIn(t, dir, "cmp", "a.txt", "b.txt")
	wantFail(t, r)
	wantOutContains(t, r, "differ")
}

func TestCmp_Silent(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello\n")
	mkFile(t, dir, "b.txt", "world\n")
	r := runIn(t, dir, "cmp", "-s", "a.txt", "b.txt")
	wantFail(t, r)
	wantOut(t, r, "")
}

func TestCmp_Verbose(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "ab\n")
	mkFile(t, dir, "b.txt", "ac\n")
	r := runIn(t, dir, "cmp", "-l", "a.txt", "b.txt")
	wantFail(t, r)
	wantOutContains(t, r, "2")
}

func TestCmp_Missing(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "x")
	r := runIn(t, dir, "cmp", "a.txt", "nonexistent")
	wantCode(t, r, 2)
}

func TestCmp_EOFPosition(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "ab\nc\nx")
	mkFile(t, dir, "b.txt", "ab\nc\n")
	r := runIn(t, dir, "cmp", "a.txt", "b.txt")
	wantFail(t, r)
	wantErrContains(t, r, "EOF on b.txt after byte 5, line 2")
}

func TestCmp_EOFPositionVerbose(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "ab\nc\nx")
	mkFile(t, dir, "b.txt", "ab\nc\n")
	r := runIn(t, dir, "cmp", "-l", "a.txt", "b.txt")
	wantFail(t, r)
	wantErrContains(t, r, "EOF on b.txt after byte 5")
}

func TestCmp_OneArgStdin(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "hello\n")
	r := runStdinIn(t, dir, "cmp", "hello\n", "a.txt")
	wantSuccess(t, r)
}

func TestCmp_OneArgStdinDiff(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "ab\nc\n")
	r := runStdinIn(t, dir, "cmp", "ab\nx\n", "a.txt")
	wantFail(t, r)
	wantOutContains(t, r, "differ: byte 4, line 2")
}

func TestCmp_NFlag(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "abcdef123")
	r := runStdinIn(t, dir, "cmp", "vwxyzdef987", "-n", "3", "a.txt", "-", "3", "5")
	wantSuccess(t, r)
}

func TestCmp_EOFToStderr(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "abc")
	mkFile(t, dir, "b.txt", "ab")
	r := runIn(t, dir, "cmp", "a.txt", "b.txt")
	wantFail(t, r)
	wantOut(t, r, "")
	wantErrContains(t, r, "EOF on b.txt")
}
