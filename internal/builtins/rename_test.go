package builtins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renameExists reports whether the directory entry exists, without following a
// final symlink.
func renameExists(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Lstat(filepath.Join(dir, name))
	return err == nil
}

// renameLinkTarget is the target text a symlink holds.
func renameLinkTarget(t *testing.T, dir, name string) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("readlink %s: %v", name, err)
	}
	return target
}

// renameSymlink creates a relative symlink named link with the given target.
func renameSymlink(t *testing.T, dir, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, link)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
		t.Fatal(err)
	}
}

func renameMkdir(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestRenamePlain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")

	r := runBuiltinIn(t, dir, "rename", "s", "z", "s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	if !renameExists(t, dir, "z1") {
		t.Error("z1 was not created")
	}
	if renameExists(t, dir, "s1") {
		t.Error("s1 still exists")
	}
}

// A no-match is not a failure and not a success: it is invisible to the tally.
func TestRenameNoMatchExitsFour(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "n1", "")

	r := runBuiltinIn(t, dir, "rename", "q", "z", "n1")
	assertExitCode(t, r, 4)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	if !renameExists(t, dir, "n1") {
		t.Error("n1 was renamed")
	}
}

// The existence check runs before the substitution result matters.
func TestRenameMissingOperandFailsWithoutMatch(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "rename", "q", "z", "s9")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "rename: s9: not accessible: ")
	assertStderrContains(t, r, "No such file or directory")
}

// The truth table: two counters rather than flags, so a success and a no-match
// still report success and a failure and a no-match still report failure.
func TestRenameTallySelectsOneStatusPerMix(t *testing.T) {
	cases := []struct {
		operands []string
		code     int
	}{
		{[]string{"s1"}, 0},
		{[]string{"s9"}, 1},
		{[]string{"n1"}, 4},
		{[]string{"s1", "s2"}, 0},
		{[]string{"s1", "s9"}, 2},
		{[]string{"s1", "n1"}, 0},
		{[]string{"s8", "s9"}, 1},
		{[]string{"s9", "n1"}, 1},
		{[]string{"n1", "n2"}, 4},
		{[]string{"s1", "s9", "n1"}, 2},
		{[]string{"s9", "n1", "n2"}, 1},
		{[]string{"s9", "s1"}, 2},
		{[]string{"n1", "s1"}, 0},
		{[]string{"n1", "s9"}, 1},
	}
	for _, c := range cases {
		dir := t.TempDir()
		for _, name := range []string{"s1", "s2", "n1", "n2"} {
			writeFile(t, dir, name, "")
		}
		r := runBuiltinIn(t, dir, "rename", append([]string{"s", "z"}, c.operands...)...)
		if r.code != c.code {
			t.Errorf("operands %v: exit %d, want %d (stderr %q)", c.operands, r.code, c.code, r.stderr)
		}
	}
}

// The short circuit is for the whole run, before any operand is touched, so a
// missing operand is not even noticed.
func TestRenameIdenticalArgumentsTouchNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")

	r := runBuiltinIn(t, dir, "rename", "s", "s", "s1", "s9")
	assertExitCode(t, r, 4)
	assertStdout(t, r, "")
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	if !renameExists(t, dir, "s1") {
		t.Error("s1 was renamed")
	}
}

func TestRenameVerboseReport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "s", "z", "s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`s1' -> `z1'\n")
}

// The report delimits both names with a backtick and a quote and escapes
// nothing, so a name holding either one goes out as it is.
func TestRenameReportDoesNotEscapeDelimiters(t *testing.T) {
	cases := []struct {
		name, needle, replacement, expected string
	}{
		{"q'te", "q", "Q", "`Q'te'"},
		{"sp ace", "sp", "SP", "`SP ace'"},
		{"t`ck", "t", "T", "`T`ck'"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, c.name, "")
		r := runBuiltinIn(t, dir, "rename", "-v", c.needle, c.replacement, c.name)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "`"+c.name+"' -> "+c.expected+"\n")
	}
}

// Which occurrence each mode takes, and how each scan behaves where the matches
// overlap: -l scans backward rather than taking the last match a forward scan
// would find. The replacement is never rescanned.
func TestRenameModesChooseTheOccurrence(t *testing.T) {
	cases := []struct {
		flags, needle, replacement, name, want string
	}{
		{"-v", "x", "Z", "axbxcx", "aZbxcx"},
		{"-va", "x", "Z", "axbxcx", "aZbZcZ"},
		{"-vl", "x", "Z", "axbxcx", "axbxcZ"},
		{"-v", "aaa", "Z", "aaaa", "Za"},
		{"-va", "aaa", "Z", "aaaa", "Za"},
		{"-vl", "aaa", "Z", "aaaa", "aZ"},
		{"-va", "a", "aa", "aaaa", "aaaaaaaa"},
		{"-v", "x", "", "axbxcx", "abxcx"},
		{"-va", "x", "", "axbxcx", "abc"},
		{"-vl", "x", "", "axbxcx", "axbxc"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, c.name, "")
		r := runBuiltinIn(t, dir, "rename", c.flags, c.needle, c.replacement, c.name)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "`"+c.name+"' -> `"+c.want+"'\n")
		if !renameExists(t, dir, c.want) {
			t.Errorf("%s %q %q %q: %q not created", c.flags, c.needle, c.replacement, c.name, c.want)
		}
	}
}

// The count is one more than the number of bytes in the name.
func TestRenameEmptyNeedleInsertsAtEveryBoundary(t *testing.T) {
	cases := []struct {
		flags, replacement, name, want string
	}{
		{"-v", "Z", "abc", "Zabc"},
		{"-vl", "Z", "abc", "abcZ"},
		{"-va", "Z", "abc", "ZaZbZcZ"},
		{"-va", "ZY", "abc", "ZYaZYbZYcZY"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, c.name, "")
		r := runBuiltinIn(t, dir, "rename", c.flags, "", c.replacement, c.name)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "`"+c.name+"' -> `"+c.want+"'\n")
		if !renameExists(t, dir, c.want) {
			t.Errorf("%s %q %q: %q not created", c.flags, c.replacement, c.name, c.want)
		}
	}
}

func TestRenameNoActChangesNothingButReports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")

	r := runBuiltinIn(t, dir, "rename", "-nv", "s", "z", "s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`s1' -> `z1'\n")
	if !renameExists(t, dir, "s1") {
		t.Error("s1 was renamed")
	}
	if renameExists(t, dir, "z1") {
		t.Error("z1 was created under -n")
	}
}

// Under -n both safeguards still run and still report, and the verbose line
// prints on top of the skip - two lines for an operand that counts as neither.
// -i degrades to exactly what -o does and never reads stdin.
func TestRenameNoActDegradesBothSafeguardsToASkip(t *testing.T) {
	for _, c := range []struct{ flags, answer string }{
		{"-nvo", ""},
		{"-nvi", "y\n"},
		{"-nvi", "n\n"},
	} {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")
		writeFile(t, dir, "n1", "")

		r := runBuiltinStdinIn(t, dir, "rename", c.answer, c.flags, "s", "n", "s1")
		assertExitCode(t, r, 4)
		assertStdout(t, r, "Skipping existing file: `n1'\n`s1' -> `n1'\n")
		if !renameExists(t, dir, "s1") {
			t.Errorf("%s answered %q: s1 was renamed", c.flags, c.answer)
		}
	}
}

func TestRenameOnlyTheFinalComponentIsRewritten(t *testing.T) {
	dir := t.TempDir()
	renameMkdir(t, dir, "d1")
	writeFile(t, dir, "d1/s1", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "d1", "d9", "d1/s1")
	assertExitCode(t, r, 4)
	assertStdout(t, r, "")
	if !renameExists(t, dir, "d1/s1") {
		t.Error("d1/s1 moved")
	}
}

// A separator in either argument widens the scope to the whole path, which is
// how a rename moves a file between directories.
func TestRenameSeparatorInAnArgumentMovesTheFile(t *testing.T) {
	dir := t.TempDir()
	renameMkdir(t, dir, "d1")
	renameMkdir(t, dir, "d9")
	writeFile(t, dir, "d1/s1", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "d1/", "d9/", "d1/s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`d1/s1' -> `d9/s1'\n")
	if !renameExists(t, dir, "d9/s1") {
		t.Error("d9/s1 was not created")
	}
}

// The scope is decided on the two argument strings, before any matching, so a
// separator widens it even when the needle cannot match the component.
func TestRenameSeparatorInEitherArgumentWidensTheScope(t *testing.T) {
	dir := t.TempDir()
	renameMkdir(t, dir, "d1")
	writeFile(t, dir, "d1/s1", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "d1/s", "X", "d1/s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`d1/s1' -> `X1'\n")
	if !renameExists(t, dir, "X1") {
		t.Error("X1 was not created")
	}

	dir = t.TempDir()
	renameMkdir(t, dir, "d1")
	renameMkdir(t, dir, "d3")
	writeFile(t, dir, "d1/s1", "")
	r = runBuiltinIn(t, dir, "rename", "d1", "d3/", "d1/s1")
	assertExitCode(t, r, 0)
	if !renameExists(t, dir, "d3/s1") {
		t.Error("d3/s1 was not created")
	}
}

// The report shows the stripped form on both sides, however many separators
// there were.
func TestRenameTrailingSeparatorsAreStripped(t *testing.T) {
	cases := []struct {
		needle, replacement, operand, want string
	}{
		{"1", "9", "d1/", "`d1' -> `d9'\n"},
		{"1", "9", "d1///", "`d1' -> `d9'\n"},
		{"", "Y", "d1/", "`d1' -> `Yd1'\n"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		renameMkdir(t, dir, "d1")
		r := runBuiltinIn(t, dir, "rename", "-v", c.needle, c.replacement, c.operand)
		assertExitCode(t, r, 0)
		assertStdout(t, r, c.want)
	}
}

// Whole-path mode never splits a component out, so it has nothing to strip and
// the trailing separator survives into the new name.
func TestRenameWholePathModeKeepsTrailingSeparator(t *testing.T) {
	dir := t.TempDir()
	renameMkdir(t, dir, "d1")

	r := runBuiltinIn(t, dir, "rename", "-v", "d1/", "d9/", "d1/")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`d1/' -> `d9/'\n")
	if !renameExists(t, dir, "d9") {
		t.Error("d9 was not created")
	}
}

// The comparison that decides whether anything changed is between the stripped
// name and the new one; comparing against the raw operand would find a
// difference here and report a rename of d1 onto itself.
func TestRenameChangeComparisonHappensAfterStripping(t *testing.T) {
	dir := t.TempDir()
	renameMkdir(t, dir, "d1")

	r := runBuiltinIn(t, dir, "rename", "-v", "QQ", "ZZ", "d1/")
	assertExitCode(t, r, 4)
	assertStdout(t, r, "")
	if !renameExists(t, dir, "d1") {
		t.Error("d1 disappeared")
	}
}

// A substitution that only rewrites a separator names the very same file, and
// that still counts as a rename.
func TestRenameThatOnlyChangesTheStringStillCounts(t *testing.T) {
	dir := t.TempDir()
	renameMkdir(t, dir, "d1")
	writeFile(t, dir, "d1/s1", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "/", "//", "d1/s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`d1/s1' -> `d1//s1'\n")
	if !renameExists(t, dir, "d1/s1") {
		t.Error("d1/s1 disappeared")
	}
}

// A leading ./ is prefix, not component, so it is neither matched against nor
// lost.
func TestRenameDotRelativeOperandKeepsItsPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "top1", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "top", "TOP", "./top1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`./top1' -> `./TOP1'\n")
	if !renameExists(t, dir, "TOP1") {
		t.Error("TOP1 was not created")
	}

	dir = t.TempDir()
	writeFile(t, dir, "top1", "")
	r = runBuiltinIn(t, dir, "rename", ".", "X", "./top1")
	assertExitCode(t, r, 4)
	if !renameExists(t, dir, "top1") {
		t.Error("top1 was renamed")
	}
}

// The stripping happens after the existence check, which still sees the operand
// exactly as it was typed - so a trailing separator on a regular file is an
// error rather than a rename.
func TestRenameExistenceCheckSeesTheUntouchedOperand(t *testing.T) {
	dir := t.TempDir()
	renameMkdir(t, dir, "d1")
	writeFile(t, dir, "d1/s1", "")

	r := runBuiltinIn(t, dir, "rename", "s", "z", "d1/s1/")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "rename: d1/s1/: not accessible: ")
	if !renameExists(t, dir, "d1/s1") {
		t.Error("d1/s1 was renamed")
	}
}

// A quiet -o run says nothing at all where a quiet -i run still asks. With -v
// the prompt and whatever follows share a line, because the prompt carries no
// newline of its own.
func TestRenameSkipReportIsVerboseGatedAndPromptIsNot(t *testing.T) {
	cases := []struct {
		flags, answer, want string
		code                int
	}{
		{"-o", "", "", 4},
		{"-vo", "", "Skipping existing file: `n1'\n", 4},
		{"-i", "n\n", "rename: overwrite `n1'? ", 4},
		{"-vi", "n\n", "rename: overwrite `n1'? Skipping existing file: `n1'\n", 4},
		{"-i", "y\n", "rename: overwrite `n1'? ", 0},
		{"-vi", "y\n", "rename: overwrite `n1'? `s1' -> `n1'\n", 0},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")
		writeFile(t, dir, "n1", "")

		r := runBuiltinStdinIn(t, dir, "rename", c.answer, c.flags, "s", "n", "s1")
		assertExitCode(t, r, c.code)
		assertStdout(t, r, c.want)
		if r.stderr != "" {
			t.Errorf("%s answered %q: stderr = %q, want empty", c.flags, c.answer, r.stderr)
		}
		if c.code == 0 && renameExists(t, dir, "s1") {
			t.Errorf("%s answered %q: s1 was not renamed", c.flags, c.answer)
		}
		if c.code != 0 && !renameExists(t, dir, "s1") {
			t.Errorf("%s answered %q: s1 was renamed", c.flags, c.answer)
		}
	}
}

// The answer is matched on its first byte: "yes" is accepted, " y" is not.
func TestRenameAnswerIsMatchedOnItsFirstByte(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", "Y extra\n"} {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")
		writeFile(t, dir, "z1", "")

		r := runBuiltinStdinIn(t, dir, "rename", answer, "-i", "s", "z", "s1")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "rename: overwrite `z1'? ")
		if renameExists(t, dir, "s1") {
			t.Errorf("%q should have been accepted", answer)
		}
	}
	for _, answer := range []string{"n\n", "maybe\n", " y\n", "\n", "1\n"} {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")
		writeFile(t, dir, "z1", "")

		r := runBuiltinStdinIn(t, dir, "rename", answer, "-i", "s", "z", "s1")
		assertExitCode(t, r, 4)
		assertStdout(t, r, "rename: overwrite `z1'? ")
		if !renameExists(t, dir, "s1") {
			t.Errorf("%q should have been declined", answer)
		}
	}
}

// Neither guard fires unless something is actually in the way, and -i does not
// read stdin when it has nothing to ask about.
func TestRenameAbsentDestinationIsNeverGuarded(t *testing.T) {
	for _, flags := range []string{"-vo", "-vi"} {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")

		r := runBuiltinStdinIn(t, dir, "rename", "n\n", flags, "s", "z", "s1")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "`s1' -> `z1'\n")
		if !renameExists(t, dir, "z1") {
			t.Errorf("%s: z1 was not created", flags)
		}
	}
}

func TestRenameNoOverwriteRefusesADestinationOfAnyKind(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")
	renameMkdir(t, dir, "z1")

	r := runBuiltinIn(t, dir, "rename", "-vo", "s", "z", "s1")
	assertExitCode(t, r, 4)
	assertStdout(t, r, "Skipping existing file: `z1'\n")
	if !renameExists(t, dir, "s1") {
		t.Error("s1 was renamed")
	}
}

// An accepted prompt is a decision to try, not a promise that it works.
func TestRenameAcceptedPromptThatCannotBeCarriedOutFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")
	renameMkdir(t, dir, "z1")
	writeFile(t, dir, "z1/keep", "")

	r := runBuiltinStdinIn(t, dir, "rename", "y\n", "-i", "s", "z", "s1")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "rename: overwrite `z1'? ")
	assertStderrContains(t, r, "rename: s1: rename to z1 failed: ")
	if !renameExists(t, dir, "s1") {
		t.Error("s1 was renamed")
	}
}

// A prompt that reaches end of input answers itself with a literal n and
// terminates the line the prompt left open. The echo is not verbose-gated.
func TestRenameInteractiveAtEndOfInputDeclines(t *testing.T) {
	cases := []struct {
		flags, want string
	}{
		{"-vi", "rename: overwrite `z1'? n\nSkipping existing file: `z1'\n"},
		{"-i", "rename: overwrite `z1'? n\n"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")
		writeFile(t, dir, "z1", "")

		r := runBuiltinIn(t, dir, "rename", c.flags, "s", "z", "s1")
		assertExitCode(t, r, 4)
		assertStdout(t, r, c.want)
		if !renameExists(t, dir, "s1") {
			t.Errorf("%s: s1 was renamed", c.flags)
		}
	}
}

// A run that outlives its input reaches end of input on the second prompt
// rather than re-reading the first answer, and a multi-character yes does not
// spill into the answer the next prompt reads.
func TestRenameEachPromptConsumesAWholeLine(t *testing.T) {
	cases := []struct {
		answers, want string
		second        bool
	}{
		{"y\n", "rename: overwrite `n1'? rename: overwrite `n2'? n\n", false},
		{"yn\n", "rename: overwrite `n1'? rename: overwrite `n2'? n\n", false},
		{"yes\ny\n", "rename: overwrite `n1'? rename: overwrite `n2'? ", true},
	}
	for _, c := range cases {
		dir := t.TempDir()
		for _, name := range []string{"s1", "s2", "n1", "n2"} {
			writeFile(t, dir, name, "")
		}
		r := runBuiltinStdinIn(t, dir, "rename", c.answers, "-i", "s", "n", "s1", "s2")
		assertExitCode(t, r, 0)
		assertStdout(t, r, c.want)
		if renameExists(t, dir, "s1") {
			t.Errorf("%q: s1 was not renamed", c.answers)
		}
		if got := !renameExists(t, dir, "s2"); got != c.second {
			t.Errorf("%q: second renamed = %v, want %v", c.answers, got, c.second)
		}
	}
}

// Three operands are required and a usage error is exit 1, which is the same
// status as "everything failed".
func TestRenameTooFewOperands(t *testing.T) {
	for _, args := range [][]string{nil, {"s", "z"}, {"-v", "s", "z"}} {
		r := runBuiltinIn(t, t.TempDir(), "rename", args...)
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "rename: missing operand")
		assertStderrContains(t, r, "Try 'rename --help' for more information.")
	}
}

// The only two mutually exclusive pairs.
func TestRenameExclusivePairsAreRefused(t *testing.T) {
	for _, args := range [][]string{
		{"-a", "-l", "x", "Z", "axbxcx"},
		{"-o", "-i", "x", "Z", "axbxcx"},
		{"-l", "-a", "-l", "x", "Z", "axbxcx"},
	} {
		dir := t.TempDir()
		writeFile(t, dir, "axbxcx", "")
		r := runBuiltinIn(t, dir, "rename", args...)
		assertExitCode(t, r, 1)
		assertStdout(t, r, "")
		assertStderrContains(t, r, "Try 'rename --help' for more information.")
		if !renameExists(t, dir, "axbxcx") {
			t.Errorf("%v: axbxcx was renamed", args)
		}
	}
}

func TestRenameUnknownOption(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "rename", "--definitely-invalid")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "rename: unknown option --definitely-invalid")
	assertStderrContains(t, r, "Try 'rename --help' for more information.")
}

// Options are permuted: a flag is recognized anywhere, including between the
// two arguments and between two file operands.
func TestRenameFlagIsRecognizedAnywhere(t *testing.T) {
	for _, args := range [][]string{
		{"-v", "s", "z", "s1"},
		{"s", "-v", "z", "s1"},
		{"s", "z", "-v", "s1"},
		{"s", "z", "s1", "-v"},
	} {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")
		r := runBuiltinIn(t, dir, "rename", args...)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "`s1' -> `z1'\n")
	}

	dir := t.TempDir()
	writeFile(t, dir, "s1", "")
	writeFile(t, dir, "s2", "")
	r := runBuiltinIn(t, dir, "rename", "s", "z", "s1", "-v", "s2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`s1' -> `z1'\n`s2' -> `z2'\n")
}

func TestRenameTerminatorTurnsEverythingIntoAnOperand(t *testing.T) {
	for _, args := range [][]string{{"--", "s", "z", "s1"}, {"s", "z", "--", "s1"}} {
		dir := t.TempDir()
		writeFile(t, dir, "s1", "")
		r := runBuiltinIn(t, dir, "rename", args...)
		assertExitCode(t, r, 0)
		if !renameExists(t, dir, "z1") {
			t.Errorf("%v: z1 was not created", args)
		}
	}

	// The flag is an operand now, and a missing one at that.
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")
	r := runBuiltinIn(t, dir, "rename", "--", "s", "z", "s1", "-v")
	assertExitCode(t, r, 2)
	assertStdout(t, r, "")
	if !renameExists(t, dir, "z1") {
		t.Error("z1 was not created")
	}
}

func TestRenameRepeatedOptionIsAccepted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")
	r := runBuiltinIn(t, dir, "rename", "-v", "-v", "s", "z", "s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`s1' -> `z1'\n")

	dir = t.TempDir()
	writeFile(t, dir, "axbxcx", "")
	r = runBuiltinIn(t, dir, "rename", "-a", "-a", "x", "Z", "axbxcx")
	assertExitCode(t, r, 0)
	if !renameExists(t, dir, "aZbZcZ") {
		t.Error("aZbZcZ was not created")
	}
}

// A long option may be abbreviated to an unambiguous prefix; a prefix shared by
// two options is an error rather than a choice.
func TestRenameLongOptionPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")
	r := runBuiltinIn(t, dir, "rename", "--verb", "s", "z", "s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`s1' -> `z1'\n")

	dir = t.TempDir()
	writeFile(t, dir, "s1", "")
	r = runBuiltinIn(t, dir, "rename", "--n", "s", "z", "s1")
	assertExitCode(t, r, 1)
	if !renameExists(t, dir, "s1") {
		t.Error("s1 was renamed")
	}
}

func TestRenameHelpAndVersion(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "rename", "--help")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "EXPRESSION")
	assertStdoutContains(t, r, "--no-overwrite")
	assertStdoutContains(t, r, "REPLACEMENT")
	assertStdoutContains(t, r, "FILE...")
	assertStdoutContains(t, r, "Exit status:")

	for _, flag := range []string{"-V", "--version"} {
		r = runBuiltinIn(t, t.TempDir(), "rename", flag)
		assertExitCode(t, r, 0)
		assertStdout(t, r, "rename (Misa) 0.1.0\n")
	}
}

// A replacement that deletes the whole name is not caught before the syscall:
// the empty string is handed to rename(2) and the kernel refuses it.
func TestRenameEmptyReplacementFailsInTheKernel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "abc", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "abc", "", "abc")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "rename: abc: rename to  failed: ")
	if !renameExists(t, dir, "abc") {
		t.Error("abc disappeared")
	}
}

// The tally counts failures, not kinds of failure.
func TestRenameRenameFailureCountsOnce(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "top1", "")
	renameMkdir(t, dir, "d3")
	writeFile(t, dir, "d3/keep", "")

	r := runBuiltinIn(t, dir, "rename", "top1", "d3", "top1")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "rename: top1: rename to d3 failed: ")
	if !renameExists(t, dir, "top1") {
		t.Error("top1 disappeared")
	}
}

// Every failing operand is reported once and in order, and the report of the
// operand that worked precedes them.
func TestRenameReportsEveryFailureInOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s2", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "s", "z", "s8", "s2", "s9")
	assertExitCode(t, r, 2)
	assertStdout(t, r, "`s2' -> `z2'\n")
	want := "rename: s8: not accessible: No such file or directory\n" +
		"rename: s9: not accessible: No such file or directory\n"
	if r.stderr != want {
		t.Errorf("stderr = %q, want %q", r.stderr, want)
	}
}

// A name holding a newline really does split the line it is printed on, on
// either stream.
func TestRenameNameWithNewlineSplitsTheLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nl\nx", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "nl", "NL", "nl\nx")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`nl\nx' -> `NL\nx'\n")
	if !renameExists(t, dir, "NL\nx") {
		t.Error("NL\\nx was not created")
	}

	r = runBuiltinIn(t, dir, "rename", "-v", "no", "yes", "no\nsuch")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "rename: no\nsuch: not accessible: No such file or directory\n")
}

// -- symlink mode ---------------------------------------------------------

func TestRenameSymlinkModeRewritesTheTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "t1", "")
	renameSymlink(t, dir, "t1", "l_ok")

	r := runBuiltinIn(t, dir, "rename", "-vs", "t", "z", "l_ok")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "l_ok: `t1' -> `z1'\n")
	if got := renameLinkTarget(t, dir, "l_ok"); got != "z1" {
		t.Errorf("link target = %q, want z1", got)
	}
}

// The symlink type check precedes the match test, so a non-symlink fails even
// when the expression appears nowhere.
func TestRenameSymlinkModeOnRegularFileFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "t1", "")

	r := runBuiltinIn(t, dir, "rename", "-s", "q", "Q", "t1")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	if r.stderr != "rename: t1: not a symbolic link\n" {
		t.Errorf("stderr = %q, want %q", r.stderr, "rename: t1: not a symbolic link\n")
	}
}

// -s never resolves a chain: the target text of l_c1 is l_c2, not t1.
func TestRenameSymlinkModeDoesNotFollowAChain(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "t1", "")
	renameSymlink(t, dir, "t1", "l_c2")
	renameSymlink(t, dir, "l_c2", "l_c1")

	r := runBuiltinIn(t, dir, "rename", "-s", "t", "z", "l_c1")
	assertExitCode(t, r, 4)
	if got := renameLinkTarget(t, dir, "l_c1"); got != "l_c2" {
		t.Errorf("link target = %q, want l_c2", got)
	}
}

// -s rewrites the target text and nothing else, whatever that text is.
func TestRenameSymlinkModeRewritesATargetOfAnyShape(t *testing.T) {
	cases := []struct {
		args  []string
		link  string
		want  string
		extra []string
	}{
		{[]string{"-s", "t", "z", "l_dangle"}, "l_dangle", "z_missing", nil},
		{[]string{"-s", "t", "z", "l_abs"}, "l_abs", "/nonexistent/z1", nil},
		{[]string{"-s", "", "P", "l_deep"}, "l_deep", "sub/Pt2", []string{"sub/t2"}},
		{[]string{"-s", "-a", "x", "X", "l_rep"}, "l_rep", "aXbXcX", nil},
	}
	for _, c := range cases {
		dir := t.TempDir()
		for _, name := range c.extra {
			writeFile(t, dir, name, "")
		}
		targets := map[string]string{
			"l_dangle": "t_missing",
			"l_abs":    "/nonexistent/t1",
			"l_deep":   "sub/t2",
			"l_rep":    "axbxcx",
		}
		renameSymlink(t, dir, targets[c.link], c.link)

		r := runBuiltinIn(t, dir, "rename", c.args...)
		assertExitCode(t, r, 0)
		if got := renameLinkTarget(t, dir, c.link); got != c.want {
			t.Errorf("%v: link target = %q, want %q", c.args, got, c.want)
		}
	}
}

// The target is scoped exactly the way a filename is: the final component only,
// unless an argument holds a separator.
func TestRenameSymlinkModeScopesTheTargetLikeAFilename(t *testing.T) {
	cases := []struct{ args []string }{
		{[]string{"-s", "sub", "SUB", "l_deep"}},
		{[]string{"-s", "ok", "OK", "l_ok"}},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, "t1", "")
		writeFile(t, dir, "sub/t2", "")
		renameSymlink(t, dir, "t1", "l_ok")
		renameSymlink(t, dir, "sub/t2", "l_deep")

		r := runBuiltinIn(t, dir, "rename", c.args...)
		assertExitCode(t, r, 4)
		assertStdout(t, r, "")
	}
}

// The type error is an ordinary per-operand failure, so it mixes with a success
// the way any other failure does.
func TestRenameSymlinkModeMixesATypeErrorIntoTally(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "t1", "")
	renameSymlink(t, dir, "t1", "l_ok")

	r := runBuiltinIn(t, dir, "rename", "-s", "-v", "t", "z", "t1", "l_ok")
	assertExitCode(t, r, 2)
	assertStdout(t, r, "l_ok: `t1' -> `z1'\n")
	if r.stderr != "rename: t1: not a symbolic link\n" {
		t.Errorf("stderr = %q", r.stderr)
	}
	if got := renameLinkTarget(t, dir, "l_ok"); got != "z1" {
		t.Errorf("link target = %q, want z1", got)
	}
}

// Under -s the guard asks whether the new target NAME is a directory entry, not
// whether the link would resolve.
func TestRenameSymlinkModeNoOverwriteProbesTheNewTargetName(t *testing.T) {
	cases := []struct {
		args []string
		want string
		code int
	}{
		{[]string{"-s", "-o", "t1", "sub", "l_ok"}, "t1", 4},
		{[]string{"-s", "-o", "t1", "zzz", "l_ok"}, "zzz", 0},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, "t1", "")
		renameMkdir(t, dir, "sub")
		renameSymlink(t, dir, "t1", "l_ok")

		r := runBuiltinIn(t, dir, "rename", c.args...)
		assertExitCode(t, r, c.code)
		if got := renameLinkTarget(t, dir, "l_ok"); got != c.want {
			t.Errorf("%v: link target = %q, want %q", c.args, got, c.want)
		}
	}

	// The skip line names the link and the target it still has, not the one it
	// was refused.
	dir := t.TempDir()
	writeFile(t, dir, "t1", "")
	renameMkdir(t, dir, "sub")
	renameSymlink(t, dir, "t1", "l_ok")
	r := runBuiltinIn(t, dir, "rename", "-s", "-o", "-v", "t1", "sub", "l_ok")
	assertExitCode(t, r, 4)
	assertStdout(t, r, "Skipping existing link: `l_ok' -> `t1'\n")
}

// Under -s the prompt names the new target rather than the link.
func TestRenameSymlinkModePromptsAboutTheNewTarget(t *testing.T) {
	cases := []struct {
		args, answer, want string
		code               int
		target             string
	}{
		{"-s -i", "y\n", "rename: overwrite `sub'? ", 0, "sub"},
		{"-s -v -i", "n\n", "rename: overwrite `sub'? Skipping existing link: `l_ok' -> `t1'\n", 4, "t1"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, "t1", "")
		renameMkdir(t, dir, "sub")
		renameSymlink(t, dir, "t1", "l_ok")

		args := append(strings.Fields(c.args), "t1", "sub", "l_ok")
		r := runBuiltinStdinIn(t, dir, "rename", c.answer, args...)
		assertExitCode(t, r, c.code)
		assertStdout(t, r, c.want)
		if got := renameLinkTarget(t, dir, "l_ok"); got != c.target {
			t.Errorf("%s: link target = %q, want %q", c.args, got, c.target)
		}
	}
}

// Under -s the -o guard is an lstat on the new target, so a dangling entry
// there blocks the rewrite, where the default path stats instead, finds
// nothing, and lets the rename clobber it.
func TestRenameSymlinkModeNoOverwriteBlockedByDanglingTarget(t *testing.T) {
	dir := t.TempDir()
	renameSymlink(t, dir, "gone", "old_t")
	renameSymlink(t, dir, "also_gone", "new_t")
	renameSymlink(t, dir, "old_t", "lnk")

	r := runBuiltinIn(t, dir, "rename", "-vos", "old_t", "new_t", "lnk")
	assertExitCode(t, r, 4)
	assertStdout(t, r, "Skipping existing link: `lnk' -> `old_t'\n")
	if got := renameLinkTarget(t, dir, "lnk"); got != "old_t" {
		t.Errorf("link target = %q, want old_t", got)
	}
}

// The default guard follows links, so a dangling destination does not count as
// existing and -o lets the rename clobber it.
func TestRenameNoOverwriteDoesNotProtectADanglingDestination(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "s1", "")
	renameSymlink(t, dir, "t_missing", "n1")

	r := runBuiltinIn(t, dir, "rename", "-vo", "s", "n", "s1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`s1' -> `n1'\n")
	if renameExists(t, dir, "n1") {
		if target, err := os.Readlink(filepath.Join(dir, "n1")); err == nil {
			t.Errorf("n1 is still a symlink to %q", target)
		}
	} else {
		t.Error("n1 was not created")
	}
}

// The -s guard probes a relative new target against the working directory
// rather than the link's own directory: sub/b1 exists, but the probe is for b1.
func TestRenameSymlinkModeProbesRelativeToWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub/a1", "")
	writeFile(t, dir, "sub/b1", "")
	renameSymlink(t, dir, "a1", "sub/lnk")

	r := runBuiltinIn(t, dir, "rename", "-vos", "a", "b", "sub/lnk")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "sub/lnk: `a1' -> `b1'\n")
	if got := renameLinkTarget(t, dir, "sub/lnk"); got != "b1" {
		t.Errorf("link target = %q, want b1", got)
	}
}

// The rewrite is not atomic and deliberately so: the unlink has already
// happened when the symlink fails, and the original link is gone. The message
// names the target it tried to create, so an empty target prints two spaces.
func TestRenameSymlinkThatCannotBeCreatedLeavesNoLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "t1", "")
	renameSymlink(t, dir, "t1", "l_ok")

	r := runBuiltinIn(t, dir, "rename", "-s", "t1", "", "l_ok")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "rename: l_ok: symlinking to  failed: ")
	if renameExists(t, dir, "l_ok") {
		t.Error("l_ok survived a failed symlink")
	}
}

// Without -s a symlink is an ordinary directory entry: its name is rewritten
// and its target is left alone.
func TestRenameWithoutSymlinkModeRenamesTheLinkByName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "t1", "")
	renameSymlink(t, dir, "t1", "l_ok")

	r := runBuiltinIn(t, dir, "rename", "_ok", "_OK", "l_ok")
	assertExitCode(t, r, 0)
	if !renameExists(t, dir, "l_OK") {
		t.Fatal("l_OK was not created")
	}
	if renameExists(t, dir, "l_ok") {
		t.Error("l_ok still exists")
	}
	if got := renameLinkTarget(t, dir, "l_OK"); got != "t1" {
		t.Errorf("link target = %q, want t1", got)
	}
}

// -- bytes, not runes -----------------------------------------------------

// A filename is a byte string, and one that is not valid UTF-8 goes through
// unchanged.
func TestRenameNameThatIsNotValidUTF8Survives(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lat\xe9n", "")

	r := runBuiltinIn(t, dir, "rename", "-v", "lat", "LAT", "lat\xe9n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "`lat\xe9n' -> `LAT\xe9n'\n")
	if !renameExists(t, dir, "LAT\xe9n") {
		t.Error("LAT\\xe9n was not created")
	}
}

// A needle may split a multi-byte character, and an empty needle counts bytes
// rather than characters.
func TestRenameNeedleMaySplitAMultibyteCharacter(t *testing.T) {
	cases := []struct{ needle, replacement, want string }{
		{"\xc3", "X", "cafX\xa9"},
		{"f\xc3", "F", "caF\xa9"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		writeFile(t, dir, "caf\xc3\xa9", "")

		r := runBuiltinIn(t, dir, "rename", c.needle, c.replacement, "caf\xc3\xa9")
		assertExitCode(t, r, 0)
		if !renameExists(t, dir, c.want) {
			t.Errorf("needle %q: %q was not created", c.needle, c.want)
		}
	}

	dir := t.TempDir()
	writeFile(t, dir, "caf\xc3\xa9", "")
	r := runBuiltinIn(t, dir, "rename", "-a", "", "_", "caf\xc3\xa9")
	assertExitCode(t, r, 0)
	if !renameExists(t, dir, "_c_a_f_\xc3_\xa9_") {
		t.Error("byte-wise insertion of the empty needle was not created")
	}
}

// -- the substitution engine ---------------------------------------------

// The engine's own table, ported from subst.rs: a name that is nothing but
// separators keeps them all, and an empty needle still inserts once into an
// empty name.
func TestRenameSubstituteEngine(t *testing.T) {
	cases := []struct {
		name, needle, replacement string
		mode                      renameMode
		want                      string
	}{
		{"", "", "Z", renameAll, "Z"},
		{"", "", "Z", renameFirst, "Z"},
		{"", "", "Z", renameLast, "Z"},
		{"/", "", "Y", renameFirst, "Y/"},
		{"/", "", "Y", renameAll, "Y/Y"},
		{"//", "", "Y", renameFirst, "/Y/"},
		{"//", "", "Y", renameAll, "/Y/Y"},
		{"///", "", "Y", renameFirst, "//Y/"},
		{"/", "x", "y", renameFirst, "/"},
		{"", "x", "y", renameFirst, ""},
		{"d1/", "1", "9", renameFirst, "d9"},
		{"d1///", "1", "9", renameFirst, "d9"},
		{"./top1", "top", "TOP", renameFirst, "./TOP1"},
		{"d1/s1", "d1/", "d9/", renameFirst, "d9/s1"},
	}
	for _, c := range cases {
		_, got := renameRewrite(c.name, c.needle, c.replacement, c.mode)
		if got != c.want {
			t.Errorf("rewrite(%q, %q, %q, %v) = %q, want %q", c.name, c.needle, c.replacement, c.mode, got, c.want)
		}
	}

	// The old side is the stripped form outside whole-path mode, and the raw
	// name inside it.
	if old, _ := renameRewrite("d1///", "1", "9", renameFirst); old != "d1" {
		t.Errorf("old = %q, want d1", old)
	}
	if old, _ := renameRewrite("d1/", "d1/", "d9/", renameFirst); old != "d1/" {
		t.Errorf("old = %q, want d1/", old)
	}
	if old, _ := renameRewrite("/", "", "Y", renameFirst); old != "/" {
		t.Errorf("old = %q, want /", old)
	}
}
