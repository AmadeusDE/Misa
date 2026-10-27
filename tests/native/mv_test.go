package native

import (
	"os"
	"testing"
)

func TestMv_Basic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "data")
	r := runIn(t, dir, "mv", "src.txt", "dst.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/src.txt"); !os.IsNotExist(err) {
		t.Error("source should be gone")
	}
	got := readFile(t, dir+"/dst.txt")
	if got != "data" {
		t.Errorf("content = %q, want %q", got, "data")
	}
}

func TestMv_IntoDir(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "data")
	os.Mkdir(dir+"/dest", 0755)
	r := runIn(t, dir, "mv", "src.txt", "dest/")
	wantSuccess(t, r)
	got := readFile(t, dir+"/dest/src.txt")
	if got != "data" {
		t.Errorf("content = %q, want %q", got, "data")
	}
}

func TestMv_Missing(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mv", "nonexistent", "dst")
	wantFail(t, r)
}

func TestMv_Rename(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "old.txt", "content")
	r := runIn(t, dir, "mv", "old.txt", "new.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/new.txt")
	if got != "content" {
		t.Errorf("content = %q, want %q", got, "content")
	}
}

func TestMv_SameFile(t *testing.T) {
	dir := t.TempDir()
	f := mkFile(t, dir, "src.txt", "data")
	r := runIn(t, dir, "mv", f, f)
	wantCode(t, r, 1)
	wantErrContains(t, r, "same file")
}

func TestMv_Interactive(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "new_content")
	mkFile(t, dir, "dst.txt", "old_content")
	// Prompt answered 'n' -> do not overwrite; real GNU mv exits nonzero here.
	rNo := runStdinIn(t, dir, "mv", "n\n", "-i", "src.txt", "dst.txt")
	wantCode(t, rNo, 1)
	if got := readFile(t, dir+"/dst.txt"); got != "old_content" {
		t.Errorf("dst.txt content = %q, want %q", got, "old_content")
	}

	// Prompt answered 'y' -> overwrite
	rYes := runStdinIn(t, dir, "mv", "y\n", "-i", "src.txt", "dst.txt")
	wantSuccess(t, rYes)
	if got := readFile(t, dir+"/dst.txt"); got != "new_content" {
		t.Errorf("dst.txt content = %q, want %q", got, "new_content")
	}
}

func TestMv_SameFileDotDir(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "mv", ".", ".")
	wantCode(t, r, 1)
	wantErrContains(t, r, "same file")
}

func TestMv_ReplaceSymlinkWithDir(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a", "file_a")
	os.Mkdir(dir+"/b", 0755)
	os.Symlink("a", dir+"/symlink")

	r := runIn(t, dir, "mv", "-T", "b", "symlink")
	wantCode(t, r, 1)
	wantErrContains(t, r, "cannot overwrite non-directory")
}

func TestMv_Exchange(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "left", "L")
	mkFile(t, dir, "right", "R")

	r := runIn(t, dir, "mv", "--exchange", "left", "right")
	wantSuccess(t, r)
	if got := readFile(t, dir+"/left"); got != "R" {
		t.Errorf("left = %q, want %q", got, "R")
	}
	if got := readFile(t, dir+"/right"); got != "L" {
		t.Errorf("right = %q, want %q", got, "L")
	}
}

func TestMv_Backup(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src", "SRCDATA")
	mkFile(t, dir, "dst", "DSTDATA")

	r := runIn(t, dir, "mv", "--backup=simple", "src", "dst")
	wantSuccess(t, r)
	if got := readFile(t, dir+"/dst"); got != "SRCDATA" {
		t.Errorf("dst = %q, want %q", got, "SRCDATA")
	}
	if got := readFile(t, dir+"/dst~"); got != "DSTDATA" {
		t.Errorf("dst~ = %q, want %q", got, "DSTDATA")
	}
}

func TestMv_BackupMightDestroySource(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a", "dst")
	mkFile(t, dir, "a~", "src")

	r := runIn(t, dir, "mv", "--backup=existing", "a~", "a")
	wantCode(t, r, 1)
	wantErrContains(t, r, "might destroy source")
}

func TestMv_ReplaceSymlinkWithFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "real", "data")
	sym := dir + "/sym"
	os.Symlink("real", sym)

	r := runIn(t, dir, "mv", "real", "sym")
	wantSuccess(t, r)
	if got := readFile(t, dir+"/sym"); got != "data" {
		t.Errorf("sym content = %q, want %q", got, "data")
	}
}

func TestMv_OverwriteEmptyDir(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(dir+"/dir_a", 0755)
	os.Mkdir(dir+"/dir_b", 0755)

	r := runIn(t, dir, "mv", "-T", "dir_a", "dir_b")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/dir_a"); !os.IsNotExist(err) {
		t.Error("dir_a should be moved")
	}
	if _, err := os.Stat(dir + "/dir_b"); err != nil {
		t.Error("dir_b should exist")
	}
}
