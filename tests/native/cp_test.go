package native

import (
	"os"
	"testing"
)

func TestCp_Basic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "hello")
	r := runIn(t, dir, "cp", "src.txt", "dst.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/dst.txt")
	if got != "hello" {
		t.Errorf("copied content = %q, want %q", got, "hello")
	}
}

func TestCp_PreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "data")
	r := runIn(t, dir, "cp", "src.txt", "dst.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/src.txt"); err != nil {
		t.Error("source should still exist")
	}
}

func TestCp_Recursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/src/sub", 0755)
	mkFile(t, dir, "src/sub/f.txt", "inner")
	r := runIn(t, dir, "cp", "-r", "src", "dst")
	wantSuccess(t, r)
	got := readFile(t, dir+"/dst/sub/f.txt")
	if got != "inner" {
		t.Errorf("copied inner = %q, want %q", got, "inner")
	}
}

func TestCp_MissingSrc(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "cp", "missing", "dst")
	wantFail(t, r)
}

func TestCp_IntoDir(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "data")
	os.Mkdir(dir+"/destdir", 0755)
	r := runIn(t, dir, "cp", "src.txt", "destdir/")
	wantSuccess(t, r)
	got := readFile(t, dir+"/destdir/src.txt")
	if got != "data" {
		t.Errorf("copied content = %q, want %q", got, "data")
	}
}

func TestCp_SameFile(t *testing.T) {
	dir := t.TempDir()
	f := mkFile(t, dir, "src.txt", "data")
	r := runIn(t, dir, "cp", f, f)
	wantCode(t, r, 1)
	wantErrContains(t, r, "same file")
}

func TestCp_SymlinkOverwrite(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target1.txt", "t1")
	mkFile(t, dir, "target2.txt", "t2")
	os.Symlink(dir+"/target1.txt", dir+"/link.txt")

	os.Symlink(dir+"/target2.txt", dir+"/src_link.txt")
	r := runIn(t, dir, "cp", "-d", "src_link.txt", "link.txt")
	wantSuccess(t, r)

	target, err := os.Readlink(dir + "/link.txt")
	if err != nil {
		t.Fatal(err)
	}
	if target != dir+"/target2.txt" {
		t.Errorf("link target = %q, want %q", target, dir+"/target2.txt")
	}
}

func TestCp_Interactive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/a", 0755)
	os.MkdirAll(dir+"/b", 0755)
	mkFile(t, dir, "a/one", "potato\n")
	mkFile(t, dir, "a/two", "potato\n")
	mkFile(t, dir, "b/one", "")
	mkFile(t, dir, "b/two", "")

	r := runStdinIn(t, dir, "cp", "n\ny\n", "-ri", "a/.", "b/.")
	t.Logf("r code=%d stdout=%q stderr=%q", r.code, r.stdout, r.stderr)
	wantSuccess(t, r)

	gotOne := readFile(t, dir+"/b/one")
	if gotOne != "" {
		t.Errorf("b/one = %q, want empty (skipped)", gotOne)
	}

	gotTwo := readFile(t, dir+"/b/two")
	if gotTwo != "potato\n" {
		t.Errorf("b/two = %q, want %q", gotTwo, "potato\n")
	}
}

func TestCp_PreservePrefixAll(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a", "data")
	r := runIn(t, dir, "cp", "--preserve=al", "a", "b")
	wantSuccess(t, r)
	got := readFile(t, dir+"/b")
	if got != "data" {
		t.Errorf("b = %q, want %q", got, "data")
	}
}

func TestCp_ParentsRecursiveEndingInParentDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/src/sub", 0755)
	os.MkdirAll(dir+"/d", 0755)
	r := runIn(t, dir, "cp", "--parents", "-r", "src/sub/..", "d")
	wantFail(t, r)
	wantErrContains(t, r, "File exists")
}

func TestCp_TargetDirectoryIsFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target.txt", "t")
	mkFile(t, dir, "src.txt", "s")
	r := runIn(t, dir, "cp", "-t", "target.txt", "src.txt")
	wantFail(t, r)
	wantErrContains(t, r, "is not a directory")
}

func TestCp_LinkSameFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file", "data")
	r := runIn(t, dir, "cp", "--link", "file", "file")
	wantSuccess(t, r)
}

// Tests for --attributes-only error message fixes.

func TestCp_AttributesOnlyCreatesFile(t *testing.T) {
	// --attributes-only on a missing dest should create an empty file.
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "content")
	r := runIn(t, dir, "cp", "--attributes-only", "src.txt", "dst.txt")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/dst.txt"); err != nil {
		t.Errorf("dest file should exist: %v", err)
	}
}

func TestCp_AttributesOnlyBadDestErrorMessage(t *testing.T) {
	// When dest path is uncreateable (e.g. under /dev/null), the error must
	// include 'cannot create regular file' with the destination path.
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "")
	r := runIn(t, dir, "cp", "--attributes-only", "src.txt", "/dev/null/n.txt")
	wantFail(t, r)
	wantErrContains(t, r, "cannot create regular file '/dev/null/n.txt'")
	wantErrContains(t, r, "Not a directory")
}

func TestCp_CannotCreateRegularFile(t *testing.T) {
	// Regular copy to an invalid dest also produces the full error message.
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "data")
	r := runIn(t, dir, "cp", "src.txt", "/dev/null/n.txt")
	wantFail(t, r)
	wantErrContains(t, r, "cannot create regular file '/dev/null/n.txt'")
}

func TestCp_TargetDirectoryNotExist(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "src.txt", "data")
	r := runIn(t, dir, "cp", "-t", "nonexistent_dir", "src.txt")
	wantFail(t, r)
	wantErrContains(t, r, "target directory 'nonexistent_dir'")
}

func TestCp_ParentsTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/src/sub", 0755)
	os.MkdirAll(dir+"/dst", 0755)
	r := runIn(t, dir, "cp", "-R", "--parents", "src/", "dst")
	wantSuccess(t, r)
	if _, err := os.Stat(dir + "/dst/src"); err != nil {
		t.Errorf("dst/src should exist: %v", err)
	}
}

func TestCp_CombinedPreserveRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/src", 0755)
	mkFileMode(t, dir, "src/f", "x", 0640)
	wantSuccess(t, runIn(t, dir, "cp", "-pr", "src", "dst"))
	info, err := os.Stat(dir + "/dst/f")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("mode = %o, want 640", info.Mode().Perm())
	}
}
