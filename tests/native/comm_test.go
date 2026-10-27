package native

import "testing"

func TestComm_Basic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "a\nb\nc\n")
	mkFile(t, dir, "b.txt", "b\nc\nd\n")
	r := runIn(t, dir, "comm", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "a\n\t\tb\n\t\tc\n\td\n")
}

func TestComm_Suppress1(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "a\nb\nc\n")
	mkFile(t, dir, "b.txt", "b\nc\nd\n")
	r := runIn(t, dir, "comm", "-1", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "\tb\n\tc\nd\n")
}

func TestComm_Suppress12(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "a\nb\nc\n")
	mkFile(t, dir, "b.txt", "b\nc\nd\n")
	r := runIn(t, dir, "comm", "-12", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "b\nc\n")
}

func TestComm_Suppress3(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "a\nb\nc\n")
	mkFile(t, dir, "b.txt", "b\nc\nd\n")
	r := runIn(t, dir, "comm", "-3", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "a\n\td\n")
}

func TestComm_Identical(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "x\ny\nz\n")
	mkFile(t, dir, "b.txt", "x\ny\nz\n")
	r := runIn(t, dir, "comm", "a.txt", "b.txt")
	wantSuccess(t, r)
	wantOut(t, r, "\t\tx\n\t\ty\n\t\tz\n")
}
