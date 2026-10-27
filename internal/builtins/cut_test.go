package builtins

import "testing"

func TestCutFieldsDefault(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a\tb\tc\n", "-f", "1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\n")
}

func TestCutFieldsMultiple(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a\tb\tc\n", "-f", "1,3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\tc\n")
}

func TestCutCustomDelimiter(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a:b:c\n", "-d", ":", "-f", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\n")
}

func TestCutFieldRange(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a:b:c:d\n", "-d", ":", "-f", "2-3")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b:c\n")
}

func TestCutBytes(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "hello world\n", "-b", "1-5")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
}

func TestCutChars(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "abcdef\n", "-c", "2-4")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "bcd\n")
}

func TestCutComplement(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a:b:c\n", "-d", ":", "-f", "2", "--complement")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a:c\n")
}

func TestCutOnlyDelimited(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "no-tab\nhas\ttab\n", "-f", "1", "-s")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "has\n")
}

func TestCutNoDelimNoS(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "no-tab\n", "-f", "1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "no-tab\n")
}

func TestCutFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a:b:c\nx:y:z\n")
	r := runBuiltinIn(t, dir, "cut", "-d", ":", "-f", "1,3", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a:c\nx:z\n")
}

func TestCutMissingMode(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "data\n")
	assertExitCode(t, r, 1)
}

func TestCutFromEnd(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a:b:c:d\n", "-d", ":", "-f", "3-")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "c:d\n")
}

func TestCutFromStart(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a:b:c:d\n", "-d", ":", "-f", "-2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a:b\n")
}

func TestCutNewlineDelimiter(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "abc\ndef\nghi\n", "-d", "\n", "-f", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "def\n")
}

func TestCutEmptyDelimiterNul(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a\x00b\x00c\n", "--delimiter=", "-f", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\n")
}

func TestCutEmojiDelimiter(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "a🗿b🗿c\n", "-d🗿", "-f", "2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "b\n")
}

func TestCutNoArgsError(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "data\n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "you must specify a list of bytes, characters, or fields")
}

func TestCutDirectoryError(t *testing.T) {
	dir := t.TempDir()
	r := runBuiltinIn(t, dir, "cut", "-f1", ".")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Is a directory")
}

// TestCutRegexFieldsUnsorted covers -F (whitespace-delimited fields, like -f
// but regex-split) combined with -D (print fields in the order requested
// instead of ascending position order).
func TestCutRegexFieldsUnsorted(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "one two three four\n", "-DF", "3,1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "three one\n")
}

func TestCutRegexFieldsUnsortedSkipsMissing(t *testing.T) {
	r := runBuiltinStdin(t, "cut", "alpha beta\n", "-DF", "2,7,1")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "beta alpha\n")
}
