package native

import (
	"path/filepath"
	"strings"
	"testing"
)

// dd cases ported from toybox tests/dd.test (0BSD, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) and uutils coreutils
// tests/by-util/test_dd.rs (MIT, commit fee61ff5d4f87e0f912a52e639942b5f7ad58663).

func TestDD_StdinStdout(t *testing.T) {
	r := runStdin(t, "dd", "I WANT\n", "status=none")
	wantSuccess(t, r)
	wantOut(t, r, "I WANT\n")
	wantErr(t, r, "")
}

func TestDD_FileToFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input", "I WANT\n")
	r := runIn(t, dir, "dd", "if=input", "of=foo", "status=none")
	wantSuccess(t, r)
	wantOut(t, r, "")
	if got := readFile(t, filepath.Join(dir, "foo")); got != "I WANT\n" {
		t.Errorf("foo = %q, want %q", got, "I WANT\n")
	}
}

// toybox: "count=2" and the posix 2x3x4 multiplier.
func TestDD_Count(t *testing.T) {
	r := runStdin(t, "dd", "high\n", "count=2", "ibs=1", "status=none")
	wantSuccess(t, r)
	wantOut(t, r, "hi")

	r = runStdin(t, "dd", "abcdefghijklmnopqrstuvwxyz", "count=2x3x4", "ibs=1", "status=none")
	wantSuccess(t, r)
	wantOut(t, r, "abcdefghijklmnopqrstuvwx")
}

// toybox: "if=input status=noxfer ibs=1" reports one record per byte read.
func TestDD_NoxferRecords(t *testing.T) {
	r := runStdin(t, "dd", "I WANT\n", "status=noxfer", "ibs=1")
	wantSuccess(t, r)
	wantOut(t, r, "I WANT\n")
	wantErr(t, r, "7+0 records in\n0+1 records out\n")
}

// toybox: "status=none" silences the summary.
func TestDD_StatusNone(t *testing.T) {
	r := runStdin(t, "dd", "I WANT\n", "status=none", "ibs=1")
	wantSuccess(t, r)
	wantOut(t, r, "I WANT\n")
	wantErr(t, r, "")
}

// toybox: seek truncates the output at the new offset unless conv=notrunc.
func TestDD_SeekTruncate(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input", "1234567890abcdefghijklmnopqrstuvwxyz")
	r := runStdinIn(t, dir, "dd", "ABCDEFGHIJK", "of=input", "bs=3", "count=2", "seek=7",
		"oflag=seek_bytes", "status=none")
	wantSuccess(t, r)
	if got := readFile(t, filepath.Join(dir, "input")); got != "1234567ABCDEF" {
		t.Errorf("input = %q, want %q", got, "1234567ABCDEF")
	}
}

func TestDD_Notrunc(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "input", "1234567890abcdefghijklmnopqrstuvwxyz")
	r := runStdinIn(t, dir, "dd", "ABCDEFGHIJK", "of=input", "bs=3", "count=2", "seek=7",
		"conv=notrunc", "oflag=seek_bytes", "status=none")
	wantSuccess(t, r)
	if got, want := readFile(t, filepath.Join(dir, "input")), "1234567ABCDEFdefghijklmnopqrstuvwxyz"; got != want {
		t.Errorf("input = %q, want %q", got, want)
	}
}

func TestDD_BytesFlags(t *testing.T) {
	r := runStdin(t, "dd", "abcdef", "count=3B", "status=none")
	wantSuccess(t, r)
	wantOut(t, r, "abc")

	r = runStdin(t, "dd", "high", "count=2", "ibs=4096", "iflag=count_bytes", "status=none")
	wantSuccess(t, r)
	wantOut(t, r, "hi")
}

func TestDD_Unsupported(t *testing.T) {
	for _, arg := range []string{"conv=ucase", "conv=ascii", "iflag=direct", "status=progress"} {
		r := runStdin(t, "dd", "x", arg)
		wantFail(t, r)
		wantErrContains(t, r, "unsupported in this version")
	}
}

func TestDD_MissingInput(t *testing.T) {
	r := run(t, "dd", "if=nope", "status=none")
	wantFail(t, r)
	wantErrContains(t, r, "No such file or directory")
}

func TestDD_Help(t *testing.T) {
	r := run(t, "dd", "--help")
	wantSuccess(t, r)
	wantOutContains(t, r, "usage: dd")
	if !strings.Contains(r.stdout, "conv=") {
		t.Errorf("help does not mention conv=: %q", r.stdout)
	}
}
