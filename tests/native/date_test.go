package native

import (
	"strings"
	"testing"
	"time"
)

func TestDate_Default(t *testing.T) {
	r := run(t, "date")
	wantSuccess(t, r)
	if len(r.stdout) < 10 {
		t.Errorf("expected date output, got %q", r.stdout)
	}
}

func TestDate_FormatYear(t *testing.T) {
	r := run(t, "date", "+%Y")
	wantSuccess(t, r)
	year := time.Now().Format("2006")
	wantOut(t, r, year+"\n")
}

func TestDate_FormatFull(t *testing.T) {
	r := run(t, "date", "-u", "+%Y-%m-%d")
	wantSuccess(t, r)
	today := time.Now().UTC().Format("2006-01-02")
	wantOut(t, r, today+"\n")
}

func TestDate_Epoch(t *testing.T) {
	r := run(t, "date", "+%s")
	wantSuccess(t, r)
	if len(strings.TrimSpace(r.stdout)) < 10 {
		t.Errorf("expected epoch timestamp, got %q", r.stdout)
	}
}

func TestDate_UTC(t *testing.T) {
	r := run(t, "date", "-u", "+%Z")
	wantSuccess(t, r)
	wantOut(t, r, "UTC\n")
}

func TestDate_DateString(t *testing.T) {
	r := run(t, "date", "-d", "2024-01-15", "+%Y-%m-%d")
	wantSuccess(t, r)
	wantOut(t, r, "2024-01-15\n")
}

func TestDate_EpochInput(t *testing.T) {
	r := run(t, "date", "-d", "@0", "-u", "+%Y")
	wantSuccess(t, r)
	wantOut(t, r, "1970\n")
}

func TestDate_ReferenceFile(t *testing.T) {
	dir := t.TempDir()
	f := mkFile(t, dir, "ref.txt", "content")
	r := runIn(t, dir, "date", "-r", f, "-u", "+%Y-%m-%d")
	wantSuccess(t, r)
	today := time.Now().UTC().Format("2006-01-02")
	wantOut(t, r, today+"\n")
}

func TestDate_SetString(t *testing.T) {
	r := run(t, "date", "-s", "2024-05-20", "+%Y-%m-%d")
	wantSuccess(t, r)
	wantOut(t, r, "2024-05-20\n")
}

func TestDate_InvalidDate(t *testing.T) {
	r := run(t, "date", "-d", "invalid-date-string")
	wantFail(t, r)
}

func TestDate_PosixTZ(t *testing.T) {
	t.Setenv("TZ", "GMT0BST,M3.5.0/1,M10.5.0/2")
	r1 := run(t, "date", "-d", "2021-03-28 00:59:59 +0000")
	wantSuccess(t, r1)
	wantOut(t, r1, "Sun Mar 28 00:59:59 GMT 2021\n")

	r2 := run(t, "date", "-d", "2021-03-28 01:00:01 +0000")
	wantSuccess(t, r2)
	wantOut(t, r2, "Sun Mar 28 02:00:01 BST 2021\n")
}

func TestDate_EpochLocal(t *testing.T) {
	t.Setenv("TZ", "EET-2EEST,M3.5.0/3,M10.5.0/4")
	r := run(t, "date", "-d", "@1288486799")
	wantSuccess(t, r)
	wantOut(t, r, "Sun Oct 31 03:59:59 EEST 2010\n")
}

func TestDate_FileInput(t *testing.T) {
	dir := t.TempDir()
	f := mkFile(t, dir, "dates.txt", "2024-01-01\n2024-06-15\n")
	r := runIn(t, dir, "date", "-f", f, "+%Y-%m-%d")
	wantSuccess(t, r)
	wantOut(t, r, "2024-01-01\n2024-06-15\n")
}

func TestDate_ISO8601(t *testing.T) {
	r := run(t, "date", "-d", "2024-01-15T12:30:00Z", "-u", "-Iseconds")
	wantSuccess(t, r)
	wantOut(t, r, "2024-01-15T12:30:00+00:00\n")
}

func TestDate_RFCEmail(t *testing.T) {
	r := run(t, "date", "-d", "2024-01-15T12:30:00Z", "-u", "-R")
	wantSuccess(t, r)
	wantOut(t, r, "Mon, 15 Jan 2024 12:30:00 +0000\n")
}

func TestDate_MilitaryTZ(t *testing.T) {
	t.Setenv("TZ", "UTC+9")
	r := run(t, "date", "-d", "m9")
	wantSuccess(t, r)
	if !strings.Contains(r.stdout, "12:00:00") {
		t.Errorf("expected 12:00:00, got %q", r.stdout)
	}
}

func TestDate_StrftimeFlags(t *testing.T) {
	r := run(t, "date", "-d", "2024-01-15 14:05:00", "-u", "+%#P %-d %_m %03j %::z")
	wantSuccess(t, r)
	wantOut(t, r, "pm 15  1 015 +00:00:00\n")
}

func TestDate_NegativeFractionalEpoch(t *testing.T) {
	r := run(t, "date", "-d", "@-0.5", "-u", "+%S.%N")
	wantSuccess(t, r)
	wantOut(t, r, "59.500000000\n")
}
