package builtins

import "testing"

func TestUniqBasic(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "aaa\naaa\nbbb\nccc\nccc\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "aaa\nbbb\nccc\n")
}

func TestUniqCount(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\na\nb\nc\nc\n", "-c")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "3 a")
	assertStdoutContains(t, r, "1 b")
	assertStdoutContains(t, r, "2 c")
}

func TestUniqDuplicatesOnly(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\nc\nc\n", "-d")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nc\n")
}

func TestUniqUniqueOnly(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\nc\nc\n", "-u")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\n")
}

func TestUniqIgnoreCase(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "Hello\nhello\nHELLO\nworld\n", "-i")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "Hello\nworld\n")
}

func TestUniqFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "x\nx\ny\ny\ny\nz\n")
	r := runBuiltinIn(t, dir, "uniq", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "x\ny\nz\n")
}

func TestUniqEmpty(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
}

func TestUniqNoConsecutive(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\nb\na\nb\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\na\nb\n")
}

func TestUniqMissingFile(t *testing.T) {
	r := runBuiltinIn(t, t.TempDir(), "uniq", "nope.txt")
	assertExitCode(t, r, 1)
}

func TestUniqInputDash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "one\ntwo\ntwo\nthree\nthree\nthree\n")
	r := runBuiltinIn(t, dir, "uniq", "f.txt", "-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "one\ntwo\nthree\n")
}

func TestUniqSkipFieldsAndChars(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "cc\tdd\tee8\nbb\tcc\tdd8\naa\tbb\tcc9\n", "-f2", "-s", "3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "cc\tdd\tee8\naa\tbb\tcc9\n")
}

func TestUniqCheckChars(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "cc1\ncc2\ncc3\n", "-w", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "cc1\n")
}

func TestUniqSkipCharsAndCheckChars(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "aaccaa\naaccbb\nbbccaa\n", "-s", "2", "-w", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "aaccaa\n")
}

func TestUniqObsoleteNumeric(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "x a\ny a\nz b\n", "-1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "x a\nz b\n")
}

func TestUniqObsoleteNumericMixed(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\n", "-5q")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown option")
}

func TestUniqZeroTerminated(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\x00a\x00b\x00", "-z")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\x00b\x00")
}

func TestUniqAllRepeated(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\nc\nc\n", "-D")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\na\nc\nc\n")
}

func TestUniqAllRepeatedSeparate(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\nc\nc\n", "--all-repeated=separate")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\na\n\nc\nc\n")
}

func TestUniqAllRepeatedPrepend(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\nc\nc\n", "--all-repeated=prepend")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\na\na\n\nc\nc\n")
}

func TestUniqGroupSeparate(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\n", "--group")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\na\n\nb\n")
}

func TestUniqGroupPrepend(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\n", "--group=prepend")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\na\na\n\nb\n")
}

func TestUniqGroupAppend(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\n", "--group=append")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\na\n\nb\n\n")
}

func TestUniqGroupBoth(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\n", "--group=both")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "\na\na\n\nb\n\n")
}

func TestUniqErrorMessage(t *testing.T) {
	r := runBuiltin(t, "uniq", "nonexistent_file_xyz")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "No such file or directory")
}

func TestUniqAllRepeatedPrefixMatch(t *testing.T) {
	r := runBuiltinStdin(t, "uniq", "a\na\nb\nc\nc\n", "--all-repeated=sep")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\na\n\nc\nc\n")
}
