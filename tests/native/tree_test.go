package native

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTree_Basic(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	mkFile(t, dir, "sub/file.txt", "hello")

	r := runIn(t, dir, "tree")
	wantSuccess(t, r)
	wantOutContains(t, r, "sub")
	wantOutContains(t, r, "file.txt")
	wantOutContains(t, r, "1 directory, 1 file")
}

func TestTree_DirsOnly(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	mkFile(t, dir, "sub/file.txt", "hello")

	r := runIn(t, dir, "tree", "-d")
	wantSuccess(t, r)
	wantOutContains(t, r, "sub")
	wantOutContains(t, r, "1 directory")
}

func TestTree_Ascii(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	mkFile(t, dir, "sub/file.txt", "hello")

	r := runIn(t, dir, "tree", "-A")
	wantSuccess(t, r)
	wantOutContains(t, r, "`-- file.txt")
}

func TestTree_NoReport(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	mkFile(t, dir, "sub/file.txt", "hello")

	r := runIn(t, dir, "tree", "--noreport")
	wantSuccess(t, r)
	wantOutContains(t, r, "sub")
	if len(r.stdout) > 0 && (r.stdout[len(r.stdout)-1] == '\n') {
		if r.stdout == "1 directory, 1 file\n" {
			t.Error("should not contain report footer")
		}
	}
}
