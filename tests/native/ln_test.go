package native

import (
	"os"
	"testing"
)

func TestLn_Symbolic(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target.txt", "data")
	r := runIn(t, dir, "ln", "-s", "target.txt", "link.txt")
	wantSuccess(t, r)
	info, err := os.Lstat(dir + "/link.txt")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink")
	}
}

func TestLn_Hard(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "target.txt", "data")
	r := runIn(t, dir, "ln", "target.txt", "link.txt")
	wantSuccess(t, r)
	got := readFile(t, dir+"/link.txt")
	if got != "data" {
		t.Errorf("hard link content = %q, want %q", got, "data")
	}
}

func TestLn_MissingTarget(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "ln", "nonexistent", "link")
	wantFail(t, r)
}

func TestLn_SymbolicMissingTarget(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "ln", "-s", "nonexistent", "link")
	wantSuccess(t, r)
	_, err := os.Lstat(dir + "/link")
	if err != nil {
		t.Error("symlink should exist even with missing target")
	}
}

func TestLn_RelativeSymlink(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/one/two", 0755)
	os.MkdirAll(dir+"/three", 0755)
	mkFile(t, dir, "three/four", "hello\n")
	r := runIn(t, dir, "ln", "-sr", "three/four", "one/two/five")
	wantSuccess(t, r)
	got := readFile(t, dir+"/one/two/five")
	if got != "hello\n" {
		t.Errorf("relative symlink content = %q, want %q", got, "hello\n")
	}
}

func TestLn_SameFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file.txt", "data")
	// Without -f, symlink to same name should fail
	r := runIn(t, dir, "ln", "-s", "file.txt", "file.txt")
	wantFail(t, r)
	wantErrContains(t, r, "are the same file")
	// Force must not replace an existing source with a self-referential link.
	r = runIn(t, dir, "ln", "-sf", "file.txt", "file.txt")
	wantFail(t, r)
	wantErrContains(t, r, "are the same file")
	if got := readFile(t, dir+"/file.txt"); got != "data" {
		t.Errorf("source content = %q, want data", got)
	}
}

func TestLn_HardSameFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file.txt", "data")
	r := runIn(t, dir, "ln", "file.txt", "file.txt")
	wantFail(t, r)
	wantErrContains(t, r, "are the same file")
}

func TestLn_Backup(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "a", "alpha")
	mkFile(t, dir, "b", "beta")
	r := runIn(t, dir, "ln", "--backup", "a", "b")
	wantSuccess(t, r)
	if readFile(t, dir+"/b~") != "beta" {
		t.Errorf("expected backup file b~ containing 'beta'")
	}
}

func TestLn_NoTargetMissingOperand(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "data")
	r := runIn(t, dir, "ln", "-s", "-T", "f.txt")
	wantFail(t, r)
	wantErrContains(t, r, "missing destination file operand after 'f.txt'")
}

func TestLn_BackupSameFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "file1", "data")
	r := runIn(t, dir, "ln", "--backup", "file1", "./file1")
	wantFail(t, r)
	wantErrContains(t, r, "are the same file")
}
