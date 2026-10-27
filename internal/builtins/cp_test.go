package builtins

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCpSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "content")
	r := runBuiltinIn(t, dir, "cp", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "dst.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content" {
		t.Errorf("dst content = %q, want %q", data, "content")
	}
}

func TestCpNoClobber(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "new")
	writeFile(t, dir, "dst.txt", "old")
	r := runBuiltinIn(t, dir, "cp", "-n", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
	data, _ := os.ReadFile(filepath.Join(dir, "dst.txt"))
	if string(data) != "old" {
		t.Errorf("dst was overwritten despite -n")
	}
}

func TestCpRecursive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	writeFile(t, dir, "subdir/file.txt", "hello")
	r := runBuiltinIn(t, dir, "cp", "-r", "subdir", "copy")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "copy", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("copied content = %q", data)
	}
}

func TestCpDirWithoutRecursive(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	r := runBuiltinIn(t, dir, "cp", "subdir", "copy")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "omitting directory")
}

func TestCpMissingOperand(t *testing.T) {
	r := runBuiltin(t, "cp", "onlyone")
	assertExitCode(t, r, 1)
}

func TestCpVerbose(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "data")
	r := runBuiltinIn(t, dir, "cp", "-v", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "->")
}

func TestCpMultipleToDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "b.txt", "b")
	os.Mkdir(filepath.Join(dir, "dest"), 0755)
	r := runBuiltinIn(t, dir, "cp", "a.txt", "b.txt", "dest")
	assertExitCode(t, r, 0)
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dir, "dest", name)); err != nil {
			t.Errorf("missing %s in dest", name)
		}
	}
}

func TestCpRecursiveUpperR(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	writeFile(t, dir, "subdir/file.txt", "hello")
	r := runBuiltinIn(t, dir, "cp", "-R", "subdir", "copy")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "copy", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("copied content = %q", data)
	}
}

// TestCpRecursivePreservesSymlinks verifies that plain "cp -R" (no -H/-L/-P)
// copies symlinks found anywhere in the tree as symlinks, matching the POSIX
// default of never dereferencing during a recursive copy.
func TestCpRecursivePreservesSymlinks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src/file", "data")
	if err := os.Symlink("file", filepath.Join(dir, "src", "file_symlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", filepath.Join(dir, "dir_symlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "dst"), 0755); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "cp", "-R", "src", "dir_symlink", "dst")
	assertExitCode(t, r, 0)

	if fi, err := os.Lstat(filepath.Join(dir, "dst", "src", "file_symlink")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected dst/src/file_symlink to remain a symlink, err=%v mode=%v", err, fi)
	}
	if fi, err := os.Lstat(filepath.Join(dir, "dst", "dir_symlink")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected top-level dir_symlink to remain a symlink under plain -R, err=%v mode=%v", err, fi)
	}
}

// TestCpRecursiveHDereferencesTopLevelOnly verifies that "cp -RH" dereferences
// symlinks passed directly as command-line arguments, but preserves symlinks
// found while recursing into a directory.
func TestCpRecursiveHDereferencesTopLevelOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src/file", "data")
	if err := os.Symlink("file", filepath.Join(dir, "src", "file_symlink")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", filepath.Join(dir, "dir_symlink")); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "cp", "-RH", "dir_symlink", "dst")
	assertExitCode(t, r, 0)

	if fi, err := os.Lstat(filepath.Join(dir, "dst")); err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Errorf("expected top-level symlink arg to be dereferenced into a real directory, err=%v mode=%v", err, fi)
	}
	if fi, err := os.Lstat(filepath.Join(dir, "dst", "file_symlink")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected nested file_symlink to remain a symlink under -H, err=%v mode=%v", err, fi)
	}
}

func TestCpPreserve(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "data")
	os.Chmod(filepath.Join(dir, "src.txt"), 0644)
	r := runBuiltinIn(t, dir, "cp", "-p", "src.txt", "dst.txt")
	assertExitCode(t, r, 0)
	srcInfo, _ := os.Stat(filepath.Join(dir, "src.txt"))
	dstInfo, _ := os.Stat(filepath.Join(dir, "dst.txt"))
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Errorf("permissions differ: src=%v dst=%v", srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
	}
}

// TestCpBackupSimple exercises --backup=simple: the pre-existing
// destination must be renamed to dst+suffix, not lost.
func TestCpBackupSimple(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "F", "test\n")
	r := runBuiltinIn(t, dir, "cp", "--force", "--backup=simple", "--suffix=.b", "F", "F")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "F.b"))
	if err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	if string(data) != "test\n" {
		t.Errorf("backup content = %q", data)
	}
	data2, err := os.ReadFile(filepath.Join(dir, "F"))
	if err != nil || string(data2) != "test\n" {
		t.Errorf("F content after self-copy = %q, err=%v", data2, err)
	}
}

// TestCpBackupNumbered exercises --backup=numbered, which must produce
// dst.~1~ rather than colliding with a plain suffix.
func TestCpBackupNumbered(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src", "new")
	writeFile(t, dir, "dst", "old")
	r := runBuiltinIn(t, dir, "cp", "--backup=numbered", "src", "dst")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "dst.~1~"))
	if err != nil || string(data) != "old" {
		t.Errorf("numbered backup missing/wrong: %v %q", err, data)
	}
}

// TestCpIntoSelfDirectory covers the historically infinite-recursing case
// of copying a directory into itself.
func TestCpIntoSelfDirectory(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "d"), 0755)
	r := runBuiltinIn(t, dir, "cp", "-R", "d", "d")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "cannot copy a directory")
	assertStderrContains(t, r, "into itself")
}

// TestCpRemoveDestination covers --remove-destination replacing an
// existing directory outright, including on a re-run.
func TestCpRemoveDestination(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "d"), 0755)
	os.Mkdir(filepath.Join(dir, "e"), 0755)
	r := runBuiltinIn(t, dir, "cp", "-R", "--remove-destination", "d", "e")
	assertExitCode(t, r, 0)
	r = runBuiltinIn(t, dir, "cp", "-R", "--remove-destination", "d", "e")
	assertExitCode(t, r, 0)
}

// TestCpNoPreserveModeUsesDefaultNotSourceBits ensures --no-preserve=mode
// falls back to the umask default rather than "source mode minus umask".
func TestCpNoPreserveModeUsesDefaultNotSourceBits(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "x")
	os.Chmod(filepath.Join(dir, "a"), 0731)
	r := runBuiltinIn(t, dir, "cp", "-a", "--no-preserve=mode", "a", "b")
	assertExitCode(t, r, 0)
	info, err := os.Stat(filepath.Join(dir, "b"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() == 0731 {
		t.Errorf("--no-preserve=mode kept the source's exact mode bits (0731); should have used the default create mode")
	}
}

// TestCpSameFileThroughDestSymlink covers writing to a destination that is
// itself a symlink resolving to the source: this must be refused rather
// than silently truncating the source through the symlink.
func TestCpSameFileThroughDestSymlink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a/foo", "bar\n")
	os.Mkdir(filepath.Join(dir, "b"), 0755)
	if err := os.Symlink("../a/foo", filepath.Join(dir, "b", "foo")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "cp", "-d", "a/foo", "b")
	assertExitCode(t, r, 1)
	data, err := os.ReadFile(filepath.Join(dir, "a", "foo"))
	if err != nil || string(data) != "bar\n" {
		t.Errorf("source was modified: %q err=%v", data, err)
	}
}

// TestCpThruDanglingSymlinkRefused covers cp's default refusal to write
// through a dangling symlink destination.
func TestCpThruDanglingSymlinkRefused(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "hi\n")
	if err := os.Symlink("no-such", filepath.Join(dir, "dangle")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "cp", "f", "dangle")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not writing through dangling symlink")
	if _, err := os.Stat(filepath.Join(dir, "no-such")); err == nil {
		t.Error("dangling symlink target should not have been created")
	}
}

// TestCpJustCreatedSymlinkProtection covers the symlink-attack protection:
// once one operand's copy creates a symlink at the destination, a later
// operand in the same invocation must not silently write through it.
func TestCpJustCreatedSymlinkProtection(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "a"), 0755)
	os.Mkdir(filepath.Join(dir, "b"), 0755)
	os.Mkdir(filepath.Join(dir, "c"), 0755)
	if err := os.Symlink("../t", filepath.Join(dir, "a", "1")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "b/1", "payload\n")
	r := runBuiltinIn(t, dir, "cp", "-dR", "a/1", "b/1", "c")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "will not copy")
	assertStderrContains(t, r, "just-created symlink")
	if _, err := os.Stat(filepath.Join(dir, "t")); err == nil {
		t.Error("cp must not have created the dangling symlink's target")
	}
}

// TestCpUpdateOlderSkipsWhenNotNewer and TestCpUpdateNoneAlwaysSkips cover
// --update's "older" (default) and "none" modes.
func TestCpUpdateOlderSkipsWhenNotNewer(t *testing.T) {
	dir := t.TempDir()
	old := writeFile(t, dir, "old", "old\n")
	newf := writeFile(t, dir, "new", "new\n")
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	os.Chtimes(old, past, past)
	os.Chtimes(newf, future, future)
	r := runBuiltinIn(t, dir, "cp", "--update", "old", "new")
	assertExitCode(t, r, 0)
	data, _ := os.ReadFile(newf)
	if string(data) != "new\n" {
		t.Errorf("--update copied an older source over a newer dest: %q", data)
	}
}

func TestCpUpdateNoneAlwaysSkips(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src", "new")
	writeFile(t, dir, "dst", "old")
	r := runBuiltinIn(t, dir, "cp", "--update=none", "src", "dst")
	assertExitCode(t, r, 0)
	data, _ := os.ReadFile(filepath.Join(dir, "dst"))
	if string(data) != "old" {
		t.Errorf("--update=none should have left the existing destination alone, got %q", data)
	}
}

// TestCpHardlinkOntoExistingHardlinkIsNoop covers "cp -l" onto a
// destination that's already hard-linked to the source: real GNU treats
// this as a no-op success rather than a same-file error.
func TestCpHardlinkOntoExistingHardlinkIsNoop(t *testing.T) {
	skipWithoutHardlinks(t)
	dir := t.TempDir()
	foo := writeFile(t, dir, "foo", "x")
	hardlink := filepath.Join(dir, "hardlink")
	if err := os.Link(foo, hardlink); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "cp", "-l", "foo", "hardlink")
	assertExitCode(t, r, 0)
}

// TestCpPreservesHardlinksAcrossDereferencedSymlinkArgs covers
// --preserve=links -H: two operands that resolve to the same file after
// dereferencing a symlink argument must end up hard-linked at dst too.
func TestCpPreservesHardlinksAcrossDereferencedSymlinkArgs(t *testing.T) {
	skipWithoutHardlinks(t)
	dir := t.TempDir()
	writeFile(t, dir, "a", "x")
	if err := os.Symlink("a", filepath.Join(dir, "b")); err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(dir, "c"), 0755)
	r := runBuiltinIn(t, dir, "cp", "--preserve=links", "-R", "-H", "a", "b", "c")
	assertExitCode(t, r, 0)
	aInfo, err := os.Stat(filepath.Join(dir, "c", "a"))
	if err != nil {
		t.Fatal(err)
	}
	bInfo, err := os.Stat(filepath.Join(dir, "c", "b"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(aInfo, bInfo) {
		t.Error("c/a and c/b should be hard-linked, preserving the source relationship")
	}
}
