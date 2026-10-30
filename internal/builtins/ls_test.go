package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLsDefaultListing(t *testing.T) {
	// With no format flag and a non-terminal stdout (as in these tests),
	// GNU ls defaults to one entry per line rather than a grid.
	dir := t.TempDir()
	writeFile(t, dir, "a", "")
	writeFile(t, dir, "b", "")
	r := runBuiltinIn(t, dir, "ls")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

func TestLsOnePerLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "")
	writeFile(t, dir, "b", "")
	r := runBuiltinIn(t, dir, "ls", "-1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
}

// TestLsHidesDotfilesByDefaultWhenNotATerminal guards against a bug where
// Misa's "always show hidden files" interactive convenience leaked into
// non-interactive (piped/redirected) runs, breaking GNU ls's documented
// default of hiding dotfiles unless -a/-A is given (coreutils
// ls/non-utf8-hidden.sh). runBuiltinIn's stdout is a bytes.Buffer, so it
// is never a terminal, matching how the GNU test harness invokes ls.
func TestLsHidesDotfilesByDefaultWhenNotATerminal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "visible", "")
	writeFile(t, dir, ".hidden", "")

	r := runBuiltinIn(t, dir, "ls", "-1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "visible\n")

	rAll := runBuiltinIn(t, dir, "ls", "-1", "-a")
	assertExitCode(t, rAll, 0)
	assertStdoutContains(t, rAll, ".hidden\n")
	assertStdoutContains(t, rAll, "visible\n")
}

func TestLsSymlinkArrowOnlyInLongFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f", "")
	if err := os.Symlink("f", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "ls", "-1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "f\nlink\n")

	rl := runBuiltinIn(t, dir, "ls", "-1", "-l")
	assertExitCode(t, rl, 0)
	assertStdoutContains(t, rl, "link -> f")
}

func TestLsSymlinkToDirListsContentsByDefault(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "sub/inside", "")
	if err := os.Symlink(sub, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "ls", "link")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "inside\n")
}

func TestLsDanglingSymlinkPlainSucceeds(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("no-such-file", filepath.Join(dir, "dangle")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "ls", "dangle")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "dangle\n")
}

func TestLsDanglingSymlinkWithDereferenceFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("no-such-file", filepath.Join(dir, "dangle")); err != nil {
		t.Fatal(err)
	}
	rL := runBuiltinIn(t, dir, "ls", "-L", "dangle")
	assertExitCode(t, rL, 2)

	rH := runBuiltinIn(t, dir, "ls", "-H", "dangle")
	assertExitCode(t, rH, 2)
}

func TestLsRecursiveHeaders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a/1/I", "")
	writeFile(t, dir, "a/2/.keep", "")
	os.Remove(filepath.Join(dir, "a", "2", ".keep"))
	writeFile(t, dir, "f", "")

	r := runBuiltinIn(t, dir, "ls", "-R1", "a", "f")
	assertExitCode(t, r, 0)
	// GNU ls lists non-directory arguments as one block before any
	// directory argument's section, regardless of argument order.
	want := "f\n\na:\n1\n2\n\na/1:\nI\n\na/2:\n"
	assertStdout(t, r, want)
}

func TestLsRecursiveSymlinkLoopDetected(t *testing.T) {
	dir := t.TempDir()
	loop := filepath.Join(dir, "loop")
	if err := os.Mkdir(loop, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "loop"), filepath.Join(loop, "sub")); err != nil {
		t.Fatal(err)
	}

	r := runBuiltinIn(t, dir, "ls", "-RL", "loop")
	assertExitCode(t, r, 2)
	assertStdout(t, r, "loop:\nsub\n")
	if r.stderr != "ls: loop/sub: not listing already-listed directory\n" {
		t.Errorf("stderr = %q, want cycle-detection message", r.stderr)
	}
}

func TestLsUnsortedDisablesSort(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "z", "")
	writeFile(t, dir, "a", "")

	sorted := runBuiltinIn(t, dir, "ls", "-1")
	assertExitCode(t, sorted, 0)
	assertStdout(t, sorted, "a\nz\n")

	unsorted := runBuiltinIn(t, dir, "ls", "-1U")
	assertExitCode(t, unsorted, 0)
	if unsorted.stdout == sorted.stdout {
		t.Skip("filesystem happened to return entries in sorted order already")
	}
}

func TestLsSortWidth(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"aaaaa", "zz", "cccc", "d", "bbb"} {
		writeFile(t, dir, n, "")
	}
	r := runBuiltinIn(t, dir, "ls", "-1", "--sort=width")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "d\nzz\nbbb\ncccc\naaaaa\n")
}

func TestLsGroupDirectoriesFirst(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file_c", "")
	writeFile(t, dir, "file_a", "")
	writeFile(t, dir, "file_b", "")
	for _, n := range []string{"dir_b", "dir_a", "dir_c"} {
		if err := os.Mkdir(filepath.Join(dir, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := runBuiltinIn(t, dir, "ls", "-1", "--group-directories-first")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "dir_a\ndir_b\ndir_c\nfile_a\nfile_b\nfile_c\n")
}

func TestLsClassify(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "regular", "")
	if err := os.Mkdir(filepath.Join(dir, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	always := runBuiltinIn(t, dir, "ls", "--classify=always")
	assertExitCode(t, always, 0)
	assertStdoutContains(t, always, "dir/")

	never := runBuiltinIn(t, dir, "ls", "--classify=never")
	assertExitCode(t, never, 0)
	if got := never.stdout; got != "dir\nregular\n" {
		t.Errorf("--classify=never stdout = %q", got)
	}

	invalid := runBuiltinIn(t, dir, "ls", "--classify=bogus")
	assertExitCode(t, invalid, 1)
}

func TestLsQuotingStyle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "com,ma", "")
	r := runBuiltinIn(t, dir, "ls", "-1", "--quoting-style=shell")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "'com,ma'\n")
}

func TestLsCommaWidthWrap(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "")
	writeFile(t, dir, "bb", "")
	writeFile(t, dir, "c", "")
	r := runBuiltinIn(t, dir, "ls", "-w5", "-m")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a,\nbb, c\n")
}

func TestLsZeroSeparatesWithNUL(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "")
	writeFile(t, dir, "b", "")
	r := runBuiltinIn(t, dir, "ls", "--zero")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\x00b\x00")
}

func TestLsZeroAndDiredIncompatible(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "")
	r := runBuiltinIn(t, dir, "ls", "-l", "--dired", "--zero")
	assertExitCode(t, r, 2)
}

func TestLsTimeStyleDiagInvalid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "")
	r := runBuiltinIn(t, dir, "ls", "-l", "--time-style=XX")
	assertExitCode(t, r, 2)
	assertStdout(t, r, "")
	if r.stderr == "" {
		t.Errorf("expected an error message on stderr")
	}
}

func TestLsSymlinkSlashDereferences(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(dir, "symlink")); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "ls", "-l", "symlink/")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "total 0\n")
}

// TestLsEscapeQuoting covers -b/--escape, the quoting style the dir and
// vdir aliases default to (uutils coreutils quoting::ESCAPE).
func TestLsEscapeQuoting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "plain", "")
	writeFile(t, dir, "a b", "")
	writeFile(t, dir, "tab\there", "")
	writeFile(t, dir, "back\\slash", "")

	for _, flag := range []string{"-b", "--escape", "--quoting-style=escape"} {
		r := runBuiltinIn(t, dir, "ls", "-1", flag)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "a\\ b\nback\\\\slash\nplain\ntab\\there\n")
	}
}

func TestLsEscapeOnlyEscapesNongraphicNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "plain", "")

	r := runBuiltinIn(t, dir, "ls", "-1", "-b")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "plain\n")

	// An explicit --quoting-style still wins over -b.
	explicit := runBuiltinIn(t, dir, "ls", "-1", "-b", "--quoting-style=literal")
	assertExitCode(t, explicit, 0)
	assertStdout(t, explicit, "plain\n")

	// Piped output stays one entry per line without -C.
	writeFile(t, dir, "second", "")
	grid := runBuiltinIn(t, dir, "ls", "-b")
	assertExitCode(t, grid, 0)
	assertStdout(t, grid, "plain\nsecond\n")
}
