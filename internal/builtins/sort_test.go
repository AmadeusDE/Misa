package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSortBasic(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "banana\napple\ncherry\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "apple\nbanana\ncherry\n")
}

func TestSortReverse(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "a\nb\nc\n", "-r")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "c\nb\na\n")
}

func TestSortNumeric(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "10\n2\n1\n20\n", "-n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n2\n10\n20\n")
}

func TestSortUnique(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "b\na\nb\na\nc\n", "-u")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\nc\n")
}

func TestSortIgnoreCase(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "Banana\napple\nCherry\n", "-f")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "apple\nBanana\nCherry\n")
}

func TestSortKeyField(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "b 2\na 3\nc 1\n", "-k", "2", "-n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "c 1\nb 2\na 3\n")
}

func TestSortFieldSeparator(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "b:2\na:3\nc:1\n", "-t", ":", "-k", "2", "-n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "c:1\nb:2\na:3\n")
}

func TestSortOutputFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "in.txt", "c\na\nb\n")
	r := runBuiltinIn(t, dir, "sort", "-o", "out.txt", "in.txt")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\nb\nc\n" {
		t.Errorf("output = %q", data)
	}
}

func TestSortFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "in.txt", "z\ny\nx\n")
	r := runBuiltinIn(t, dir, "sort", "in.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "x\ny\nz\n")
}

func TestSortEmpty(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

// A missing input file is a file-access error: real GNU sort exits 2
// (SORT_FAILURE) for these, not 1.
func TestSortMissingFile(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "sort", "nope.txt")
	assertExitCode(t, r, 2)
}

func TestSortStable(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "b 1\na 1\nc 1\n", "-s", "-k", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b 1\na 1\nc 1\n")
}

// TestSortNumericKeyTieBreaksOnWholeLine verifies that when a numeric key
// comparison ties, sort falls back to comparing the whole line (ascending),
// not the extracted key substring itself.
func TestSortNumericKeyTieBreaksOnWholeLine(t *testing.T) {
	input := "42\t1\t3\twoot\n42\t1\t010\tzoology\negg\t1\t2\tpapyrus\n7\t3\t42\tsoup\n999\t3\t0\talgebra\n"
	r := runBuiltinStdin(t, "sort", input, "-k2,3n")
	assertExitCode(t, r, 0)
	want := "42\t1\t010\tzoology\n42\t1\t3\twoot\negg\t1\t2\tpapyrus\n7\t3\t42\tsoup\n999\t3\t0\talgebra\n"
	assertStdout(t, r, want)
}

// TestSortKeyOwnOrderOptsIgnoresGlobalReverse verifies that a -k key spec
// carrying its own ordering suboption (here "n") ignores a separate global
// -r for the primary key comparison, per GNU sort semantics; only the
// whole-line tie-break fallback honors the global reverse.
func TestSortKeyOwnOrderOptsIgnoresGlobalReverse(t *testing.T) {
	input := "42\t1\t3\twoot\n42\t1\t010\tzoology\negg\t1\t2\tpapyrus\n7\t3\t42\tsoup\n999\t3\t0\talgebra\n"
	r := runBuiltinStdin(t, "sort", input, "-k2,3n", "-r")
	assertExitCode(t, r, 0)
	want := "egg\t1\t2\tpapyrus\n42\t1\t3\twoot\n42\t1\t010\tzoology\n999\t3\t0\talgebra\n7\t3\t42\tsoup\n"
	assertStdout(t, r, want)
}

// TestSortKeyWithoutOwnOrderOptsInheritsGlobalReverse verifies that a bare
// -k spec with no suboptions of its own (unlike the case above) does inherit
// the global -r for its own comparison.
func TestSortKeyWithoutOwnOrderOptsInheritsGlobalReverse(t *testing.T) {
	r := runBuiltinStdin(t, "sort", "a 1\nb 2\nc 1\nd 2\n", "-k2", "-r", "-s")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b 2\nd 2\na 1\nc 1\n")
}

// TestSortMergeAccepted verifies -m (merge already-sorted inputs) is accepted
// rather than rejected as an unknown option; re-sorting pre-sorted inputs
// yields the same correct result as a true merge would.
func TestSortMergeAccepted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "1\n3\n5\n")
	writeFile(t, dir, "b", "2\n4\n6\n")
	r := runBuiltinIn(t, dir, "sort", "-n", "-m", "a", "b")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n2\n3\n4\n5\n6\n")
}

// TestSortDashAmongFilesReadsStdin verifies "-" is treated as stdin even when
// mixed with real file operands, not looked up as a literal file named "-".
func TestSortDashAmongFilesReadsStdin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "3\n")
	r := runBuiltinStdinIn(t, dir, "sort", "1\n2\n", "-n", "a", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "1\n2\n3\n")
}

// TestSortOutputNotWritableFailsFast verifies sort checks the -o target is
// writable before consuming any input, exiting 2 (GNU sort's SORT_FAILURE)
// rather than 1.
func TestSortOutputNotWritableFailsFast(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks don't apply as root")
	}
	dir := t.TempDir()
	out := writeFile(t, dir, "out", "")
	if err := os.Chmod(out, 0400); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinStdinIn(t, dir, "sort", "", "-o", "out")
	assertExitCode(t, r, 2)
}

// TestSortUnreadableInputFailsWithoutCreatingOutput verifies sort checks all
// inputs are readable before creating the -o target at all.
func TestSortUnreadableInputFailsWithoutCreatingOutput(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission checks don't apply as root")
	}
	dir := t.TempDir()
	unreadable := writeFile(t, dir, "unreadable", "x\n")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadable, 0644)

	r := runBuiltinStdinIn(t, dir, "sort", "", "-o", "newfile", "-", "unreadable")
	assertExitCode(t, r, 2)
	if _, err := os.Stat(filepath.Join(dir, "newfile")); err == nil {
		t.Errorf("expected sort not to create the output file when an input is unreadable")
	}
}
