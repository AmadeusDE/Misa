package native

import (
	"os"
	"testing"
)

func TestRm_SingleFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "data")
	r := runIn(t, dir, "rm", "f.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/f.txt"); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

func TestRm_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a.txt", "")
	mkFile(t, dir, "b.txt", "")
	r := runIn(t, dir, "rm", "a.txt", "b.txt")
	wantSuccess(t, r)
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(dir + "/" + name); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", name)
		}
	}
}

func TestRm_Recursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/sub/deep", 0755)
	mkFile(t, dir, "sub/deep/f.txt", "data")
	r := runIn(t, dir, "rm", "-r", "sub")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/sub"); !os.IsNotExist(err) {
		t.Error("directory should be removed")
	}
}

func TestRm_Force(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "rm", "-f", "nonexistent")
	wantSuccess(t, r)
}

func TestRm_Missing(t *testing.T) {
	r := run(t, "rm", "nonexistent")
	wantFail(t, r)
}

func TestRm_DirectoryWithoutR(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(dir+"/sub", 0755)
	r := runIn(t, dir, "rm", "sub")
	wantFail(t, r)
}

func TestRm_VerboseFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "hello.txt", "data")
	r := runIn(t, dir, "rm", "-v", "hello.txt")
	wantSuccess(t, r)
	wantOut(t, r, "removed 'hello.txt'\n")
}

func TestRm_VerboseRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/d1", 0755)
	mkFile(t, dir, "d1/f1.txt", "")
	mkFile(t, dir, "d1/f2.txt", "")
	r := runIn(t, dir, "rm", "-rv", "d1")
	wantSuccess(t, r)
	wantOutContains(t, r, "removed 'd1/f1.txt'")
	wantOutContains(t, r, "removed 'd1/f2.txt'")
	wantOutContains(t, r, "removed directory 'd1'")
	if _, err := os.Stat(dir + "/d1"); !os.IsNotExist(err) {
		t.Error("directory should be removed")
	}
}

func TestRm_DirFlag(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(dir+"/empty_dir", 0755)
	r := runIn(t, dir, "rm", "-d", "empty_dir")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/empty_dir"); !os.IsNotExist(err) {
		t.Error("empty directory should be removed with -d")
	}
}

func TestRm_PreserveRootInvalid(t *testing.T) {
	r := run(t, "rm", "--preserve-root=invalid", "file")
	wantFail(t, r)
	wantErrContains(t, r, "invalid value 'invalid' for option '--preserve-root'")
}

func TestRm_Interactive(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "data")
	r := runStdinIn(t, dir, "rm", "y\n", "-i", "f.txt")
	wantSuccess(t, r)
	wantErrContains(t, r, "rm: remove regular file 'f.txt'?")
	if _, err := os.Stat(dir + "/f.txt"); !os.IsNotExist(err) {
		t.Error("file should be removed when user confirms with y")
	}
}

func TestRm_RecursiveLongFilepath(t *testing.T) {
	dir := t.TempDir()
	longDir := "nested_dir/"
	for i := 0; i < 30; i++ {
		longDir += "nested_dir/"
	}
	fileA := longDir + "test_file_a"
	mkFile(t, dir, fileA, "data")
	r := runIn(t, dir, "rm", "-r", "nested_dir")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/nested_dir"); !os.IsNotExist(err) {
		t.Error("deeply nested directory should be removed")
	}
}
