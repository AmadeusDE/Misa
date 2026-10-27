package native

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTouch_Create(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "new.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/new.txt"); err != nil {
		t.Error("file should be created")
	}
}

func TestTouch_Existing(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "data")
	r := runIn(t, dir, "touch", "f.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/f.txt")
	if got != "data" {
		t.Errorf("touch should not change contents, got %q", got)
	}
}

func TestTouch_NoCreate(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-c", "nonexistent")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/nonexistent"); !os.IsNotExist(err) {
		t.Error("file should not be created with -c")
	}
}

func TestTouch_Multiple(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "a.txt", "b.txt")
	wantSuccess(t, r)
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(dir + "/" + name); err != nil {
			t.Errorf("%s should exist", name)
		}
	}
}

func TestTouch_NoCreateExisting(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "walrus", "hello")
	r := runIn(t, dir, "touch", "-c", "walrus")
	wantSuccess(t, r)
	if _, err := os.Stat(filepath.Join(dir, "walrus")); err != nil {
		t.Error("existing file should still exist after touch -c")
	}
}

func TestTouch_TimestampT_CCYY(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-t", "201201231234", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2012, 1, 23, 12, 34, 0, 0, time.Local)
	if !info.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestTouch_TimestampT_Seconds(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-t", "201201231234.56", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2012, 1, 23, 12, 34, 56, 0, time.Local)
	if !info.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestTouch_TimestampT_MMDDhhmm(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-t", "01231234", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	want := time.Date(now.Year(), 1, 23, 12, 34, 0, 0, time.Local)
	if !info.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestTouch_DateD(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-d", "2009-02-13T23:31:30Z", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2009, 2, 13, 23, 31, 30, 0, time.UTC)
	if !info.ModTime().Equal(want) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), want)
	}
}

func TestTouch_ReferenceR(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "ref.txt", "hello")
	refTime := time.Date(2020, 6, 15, 10, 30, 0, 0, time.UTC)
	os.Chtimes(filepath.Join(dir, "ref.txt"), refTime, refTime)

	r := runIn(t, dir, "touch", "-r", "ref.txt", "new.txt")
	wantSuccess(t, r)
	info, err := os.Stat(filepath.Join(dir, "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(refTime) {
		t.Errorf("mtime = %v, want %v", info.ModTime(), refTime)
	}
}

func TestTouch_DummyF(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-f", "f.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(filepath.Join(dir, "f.txt")); err != nil {
		t.Error("f.txt should exist with -f option")
	}
}

func TestTouch_TrailingSlashNoCreate(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-c", "no-file/")
	wantSuccess(t, r)
}

func TestTouch_ErrorFormatCannotTouch(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "/no-such-dir/file")
	wantCode(t, r, 1)
	wantErrContains(t, r, "touch: cannot touch '/no-such-dir/file': No such file or directory")
}

func TestTouch_EmptyFileUpdatesTimes(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "empty.txt", "")
	oldTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(filepath.Join(dir, "empty.txt"), oldTime, oldTime)

	r := runIn(t, dir, "touch", "empty.txt")
	wantSuccess(t, r)

	info, err := os.Stat(filepath.Join(dir, "empty.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Equal(oldTime) {
		t.Error("expected modtime to be updated on empty file")
	}
}

func TestTouch_60SecondsLeapSecond(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "touch", "-t", "197001010000.60", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Sub(time.Unix(0, 0)) < 59*time.Second {
		t.Errorf("expected time near 60s epoch, got %v", info.ModTime())
	}
}

func TestTouch_ObsolescentPosixFormat(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POSIXLY_CORRECT", "1")
	r := runIn(t, dir, "touch", "01010000", "target.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(filepath.Join(dir, "target.txt")); err != nil {
		t.Error("target.txt should be created")
	}
	if _, err := os.Stat(filepath.Join(dir, "01010000")); !os.IsNotExist(err) {
		t.Error("01010000 should be parsed as date timestamp, not created as file")
	}
}

func TestTouch_ReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ro.txt")
	mkFile(t, dir, "ro.txt", "content")
	os.Chmod(path, 0444)

	r := runIn(t, dir, "touch", "ro.txt")
	wantSuccess(t, r)
}

func TestTouch_RelativeDateWithReference(t *testing.T) {
	dir := t.TempDir()
	refTime := time.Date(2004, 1, 16, 12, 0, 0, 0, time.UTC)
	refPath := filepath.Join(dir, "ref.txt")
	mkFile(t, dir, "ref.txt", "data")
	os.Chtimes(refPath, refTime, refTime)

	targetPath := filepath.Join(dir, "target.txt")
	mkFile(t, dir, "target.txt", "data")

	r := runIn(t, dir, "touch", "--ref", "ref.txt", "--date=-5 days", "target.txt")
	wantSuccess(t, r)

	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	wantTime := time.Date(2004, 1, 11, 12, 0, 0, 0, time.UTC)
	if info.ModTime().UTC().Format("2006-01-02") != wantTime.Format("2006-01-02") {
		t.Errorf("got date %v, want %v", info.ModTime().UTC(), wantTime)
	}
}
