package native

import (
	"os"
	"testing"
)

func TestStat_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello")
	r := runIn(t, dir, "stat", "f.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "f.txt")
	wantOutContains(t, r, "Size:")
}

func TestStat_FormatSize(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "12345")
	r := runIn(t, dir, "stat", "-c", "%s", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "5\n")
}

func TestStat_FormatEscapes(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "12345")
	r := runIn(t, dir, "stat", "-c", "%s\\t%n\\n", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "5\tf.txt\n\n")
}

func TestStat_FormatName(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%n", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "f.txt\n")
}

func TestStat_FormatType(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%F", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "regular empty file\n")
}

func TestStat_FormatPerms(t *testing.T) {
	dir := t.TempDir()
	p := mkFile(t, dir, "f.txt", "x")
	os.Chmod(p, 0644)
	r := runIn(t, dir, "stat", "-c", "%a", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "644\n")
}

func TestStat_Directory(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "stat", ".")
	wantSuccess(t, r)
	wantOutContains(t, r, "directory")
}

func TestStat_Symlink(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target.txt", "content")
	symlink(t, dir+"/target.txt", dir+"/link.txt")
	r := runIn(t, dir, "stat", "-c", "%F", "link.txt")
	wantSuccess(t, r)
	wantOutContains(t, r, "symbolic link")
}

func TestStat_FormatAccessTimeEpoch(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "x")
	r := runIn(t, dir, "stat", "-c", "%X", "f.txt")
	wantSuccess(t, r)
	if len(r.stdout) == 0 || r.stdout == "\n" {
		t.Errorf("expected epoch timestamp, got %q", r.stdout)
	}
}

func TestStat_FormatModTimeEpoch(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "x")
	r := runIn(t, dir, "stat", "-c", "%Y", "f.txt")
	wantSuccess(t, r)
	if len(r.stdout) == 0 || r.stdout == "\n" {
		t.Errorf("expected epoch timestamp, got %q", r.stdout)
	}
}

func TestStat_FormatAccessTimeHuman(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "x")
	r := runIn(t, dir, "stat", "-c", "%x", "f.txt")
	wantSuccess(t, r)
	if !containsAll(r.stdout, "-", ":", ".") {
		t.Errorf("expected human-readable timestamp, got %q", r.stdout)
	}
}

func TestStat_FormatModTimeHuman(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "x")
	r := runIn(t, dir, "stat", "-c", "%y", "f.txt")
	wantSuccess(t, r)
	if !containsAll(r.stdout, "-", ":", ".") {
		t.Errorf("expected human-readable timestamp, got %q", r.stdout)
	}
}

func TestStat_FormatEscape(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%%", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "%\n")
}

func TestStat_FormatUnknown(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%q", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "?\n")
}

func TestStat_Missing(t *testing.T) {
	r := run(t, "stat", "nonexistent")
	wantFail(t, r)
}

func TestStat_MultipleFormats(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello")
	r := runIn(t, dir, "stat", "-c", "%n %s", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "f.txt 5\n")
}

func TestStat_Printf(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello")
	r := runIn(t, dir, "stat", "--printf", "-%n", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "-f.txt")
}

func TestStat_FileSystem(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "stat", "-f", ".")
	wantSuccess(t, r)
	wantOutContains(t, r, "Block size:")
}

// Tests for format-directive fixes.

func TestStat_FormatDeviceDecimal(t *testing.T) {
	// %d: device number in decimal — must succeed and produce a non-empty number.
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%d", "f.txt")
	wantSuccess(t, r)
	if r.stdout == "\n" || r.stdout == "" {
		t.Errorf("expected non-empty device number, got %q", r.stdout)
	}
}

func TestStat_FormatDeviceHex(t *testing.T) {
	// %D: device number in hex — must succeed and produce a non-empty value.
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%D", "f.txt")
	wantSuccess(t, r)
	if r.stdout == "\n" || r.stdout == "" {
		t.Errorf("expected non-empty hex device number, got %q", r.stdout)
	}
}

func TestStat_FormatIOBlockSize(t *testing.T) {
	// %o: preferred I/O block size.
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%o", "f.txt")
	wantSuccess(t, r)
	if r.stdout == "\n" || r.stdout == "" {
		t.Errorf("expected non-empty block size, got %q", r.stdout)
	}
}

func TestStat_FormatInvalidDirectiveIncompletePrecision(t *testing.T) {
	// '%d%.3' — %d is valid but %.3 at end of format string is an invalid directive.
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%d%.3", "f.txt")
	wantFail(t, r)
	wantErr(t, r, "stat: '%.3': invalid directive\n")
}

func TestStat_FormatUnknownBareLetterOutputsQuestionMark(t *testing.T) {
	// A bare unknown letter like %q must still output '?' (backward compat).
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "")
	r := runIn(t, dir, "stat", "-c", "%q", "f.txt")
	wantSuccess(t, r)
	wantOut(t, r, "?\n")
}
