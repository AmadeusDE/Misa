package native

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChmod_OctalMode(t *testing.T) {
	dir := t.TempDir()
	f := mkFile(t, dir, "file.txt", "hello")
	r := runIn(t, dir, "chmod", "0755", "file.txt")
	wantSuccess(t, r)

	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("got perm %04o, want 0755", info.Mode().Perm())
	}
}

func TestChmod_SymbolicAdd(t *testing.T) {
	dir := t.TempDir()
	f := mkFileMode(t, dir, "script.sh", "echo hi", 0644)
	r := runIn(t, dir, "chmod", "+x", "script.sh")
	wantSuccess(t, r)

	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("expected executable perm, got %04o", info.Mode().Perm())
	}
}

func TestChmod_ChangesFlag(t *testing.T) {
	dir := t.TempDir()
	mkFileMode(t, dir, "one", "test", 0644)
	r := runIn(t, dir, "chmod", "-c", "u+x", "one")
	wantSuccess(t, r)
	wantOutContains(t, r, "changed")

	// Second run with -c should produce no output as mode didn't change
	r2 := runIn(t, dir, "chmod", "-c", "u+x", "one")
	wantSuccess(t, r2)
	wantOut(t, r2, "")
}

func TestChmod_Recursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	os.MkdirAll(sub, 0755)
	f1 := mkFileMode(t, dir, "one", "a", 0644)
	f2 := mkFileMode(t, dir, "subdir/two", "b", 0644)

	r := runIn(t, dir, "chmod", "-R", "0700", ".")
	wantSuccess(t, r)

	i1, _ := os.Stat(f1)
	i2, _ := os.Stat(f2)
	if i1.Mode().Perm() != 0700 || i2.Mode().Perm() != 0700 {
		t.Errorf("recursive failed: f1=%04o, f2=%04o", i1.Mode().Perm(), i2.Mode().Perm())
	}
}

func TestChmod_NegativeSymbolicMode(t *testing.T) {
	dir := t.TempDir()
	f := mkFileMode(t, dir, "f.txt", "data", 0755)
	r := runIn(t, dir, "chmod", "-x", "f.txt")
	wantSuccess(t, r)
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 != 0 {
		t.Errorf("expected no exec bit, got %04o", info.Mode().Perm())
	}
}

func TestChmod_NonExistingFile(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "chmod", "-R", "-r,a+w", "does-not-exist")
	wantCode(t, r, 1)
	wantErrContains(t, r, "cannot access 'does-not-exist': No such file or directory")
}

func TestChmod_QuietFlag(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "chmod", "-R", "--quiet", "-r,a+w", "does-not-exist")
	wantCode(t, r, 1)
	wantErr(t, r, "")
}

func TestChmod_SilentFlag(t *testing.T) {
	dir := t.TempDir()
	r := runIn(t, dir, "chmod", "-R", "-f", "-r,a+w", "does-not-exist")
	wantCode(t, r, 1)
	wantErr(t, r, "")
}

func TestChmod_InvalidOperator(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "probe", "test")
	r := runIn(t, dir, "chmod", "g+rw?x", "probe")
	wantCode(t, r, 1)
	wantErrContains(t, r, "invalid mode")
}

// Test fix for equals.sh: chmod g=u (copy group from user)
func TestChmod_ReferenceClass_CopyUser(t *testing.T) {
	dir := t.TempDir()
	f := mkFileMode(t, dir, "file", "test", 0754) // rwxr-xr--
	r := runIn(t, dir, "chmod", "g=u", "file")
	wantSuccess(t, r)

	info, _ := os.Stat(f)
	// User is 7 (rwx), so group should also be 7 (rwx) -> 774
	if perm := info.Mode().Perm(); perm != 0774 {
		t.Errorf("chmod g=u: got %04o, want 0774", perm)
	}
}

// Test fix for equals.sh: chmod a=,u=rwx,g=u (multiple clauses with reference)
func TestChmod_ReferenceClass_MultipleClauses(t *testing.T) {
	dir := t.TempDir()
	f := mkFileMode(t, dir, "file", "test", 0777)
	r := runIn(t, dir, "chmod", "a=,u=rwx,g=u", "file")
	wantSuccess(t, r)

	info, _ := os.Stat(f)
	// After a=, clear all. Then u=rwx sets user to 7. Then g=u copies user to group -> 770
	if perm := info.Mode().Perm(); perm != 0770 {
		t.Errorf("chmod a=,u=rwx,g=u: got %04o, want 0770", perm)
	}
}

// Test fix for equal-x.sh: chmod =x with umask
func TestChmod_EqualsX_WithUmask(t *testing.T) {
	dir := t.TempDir()
	// Create file and use umask in subprocess
	f := mkFileMode(t, dir, "file", "test", 0777)

	// Use umask 005 (masks write and execute for others)
	// chmod =x should result in execute only for user and group
	r := runInWithUmask(t, dir, 0005, "chmod", "=x", "file")
	wantSuccess(t, r)

	info, _ := os.Stat(f)
	// Result should be 110 (---x--x---) when umask is 005
	if perm := info.Mode().Perm(); perm != 0110 {
		t.Errorf("chmod =x with umask 005: got %04o, want 0110", perm)
	}
}

// Test fix for thru-dangling.sh: proper error message for dangling symlinks
func TestChmod_DanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	symlinkPath := filepath.Join(dir, "dangle")
	os.Symlink("non-existent", symlinkPath)

	r := runIn(t, dir, "chmod", "644", "dangle")
	wantCode(t, r, 1)
	wantErrContains(t, r, "cannot operate on dangling symlink 'dangle'")
}

// Test fix for ignore-symlink.sh: -R ignores symlinks silently
func TestChmod_Recursive_IgnoresSymlinks(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "dir")
	os.MkdirAll(subdir, 0755)
	f := mkFileMode(t, subdir, "file", "test", 0644)
	linkPath := filepath.Join(subdir, "link")
	os.Symlink("file", linkPath)

	r := runIn(t, dir, "chmod", "u+w", "-R", "dir")
	wantSuccess(t, r)
	wantErr(t, r, "")

	// File should be changed, but symlink should be left alone
	info, _ := os.Stat(f)
	if info.Mode().Perm() != 0644 {
		t.Errorf("expected file unchanged, got %04o", info.Mode().Perm())
	}
}
