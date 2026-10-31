package builtins

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures are bzip2 streams made with Python's bz2 module (libbz2), so the
// tests need no external bzip2 binary and no user configuration. Regenerate
// them with:
//
//	python3 -c 'import bz2,sys;sys.stdout.buffer.write(bz2.compress(b"hello\n"))' | base64
//	python3 -c 'import bz2,sys;sys.stdout.buffer.write(bz2.compress(b"one\n")+bz2.compress(b"two\n")+bz2.compress(b"three\n"))' | base64
//	python3 -c 'import bz2,sys;sys.stdout.buffer.write(bz2.compress(b"the quick brown fox jumps over the lazy dog\n"*50))' | base64
//	python3 -c 'import bz2,sys;sys.stdout.buffer.write(bz2.compress(b""))' | base64
const (
	// "hello\n"
	bz2HelloFixture = "QlpoOTFBWSZTWcHAgOIAAAFBAAAQAkSgADDNAMNGKZcXckU4UJDBwIDi"
	// "one\n", "two\n" and "three\n" as three concatenated streams.
	bz2MultiFixture = "QlpoOTFBWSZTWXgTwIIAAAHBAAAQAgGgACGaaDNNDLxdyRThQkHgTwIIQlpoOTFBWSZTWS63GtcAAAFBgAAQAACEgCAAIZpoM00HPF3JFOFCQLrca1xCWmg5MUFZJlNZAcczagAAAsGAABACQBQAIAAhg0GaCLAcXckU4UJABxzNqA=="
	// "the quick brown fox jumps over the lazy dog\n" repeated 50 times (2200
	// bytes), small enough to cut at every offset.
	bz2TextFixture = "QlpoOTFBWSZTWZ1lN3MAA5zRgAAQQAA////wMAC4CjQ0AAABRoaAAAAKVSammATIzRtSb0XNFhF2RYRckWiLJFoi/UXFF3RZIu5Fqi2IvKL2Raos0X8iyRbEWEXlFmi5ItEXtFmi6ovSLVFmi9IsIsIvxSPiL4i/xdyRThQkJ1lN3MA="
	// The empty string: a valid stream that decodes to nothing.
	bz2EmptyFixture = "QlpoORdyRThQkAAAAAA="

	bz2TextPayload = "the quick brown fox jumps over the lazy dog\n"
)

func bz2Fixture(t *testing.T, encoded string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("bad fixture %q: %v", encoded, err)
	}
	return data
}

// TestBunzip2Streams covers bytes in and bytes out, with the exit status and
// the diagnostic for the failures.
func TestBunzip2Streams(t *testing.T) {
	hello := bz2Fixture(t, bz2HelloFixture)
	text := bz2Fixture(t, bz2TextFixture)
	corruptCRC := append([]byte(nil), hello...)
	corruptCRC[len(corruptCRC)-1] ^= 0xff // inside the combined stream CRC

	tests := []struct {
		name   string
		stdin  []byte
		args   []string
		stdout string
		code   int
		stderr string
	}{
		{name: "stdin to stdout", stdin: hello, stdout: "hello\n"},
		{name: "explicit -", stdin: hello, args: []string{"-"}, stdout: "hello\n"},
		{name: "with -c", stdin: hello, args: []string{"-c"}, stdout: "hello\n"},
		{name: "concatenated streams", stdin: bz2Fixture(t, bz2MultiFixture), stdout: "one\ntwo\nthree\n"},
		{name: "empty payload", stdin: bz2Fixture(t, bz2EmptyFixture), stdout: ""},
		{name: "test ok", stdin: hello, args: []string{"-t"}, stdout: ""},
		{name: "test and -c discards output", stdin: hello, args: []string{"-t", "-c"}, stdout: ""},
		{name: "empty input", stdin: nil, code: 1, stderr: "not bzip2 data"},
		{name: "not bzip2 data", stdin: []byte("plain text\n"), code: 1, stderr: "not bzip2 data"},
		{name: "truncated header", stdin: hello[:2], code: 1, stderr: "unexpected end of file"},
		{name: "truncated body", stdin: text[:len(text)/2], code: 1, stderr: "unexpected end of file"},
		// The bytes decoded before the trailer failed still reach stdout, like
		// gunzip -c on a damaged file.
		{name: "corrupt crc", stdin: corruptCRC, stdout: "hello\n", code: 1, stderr: "invalid checksum (corrupt data)"},
		{name: "corrupt crc with -t", stdin: corruptCRC, args: []string{"-t"}, code: 1, stderr: "invalid checksum (corrupt data)"},
		{name: "trailing garbage", stdin: append(hello, "junk"...), stdout: "hello\n", code: 1, stderr: "trailing data after the last bzip2 stream"},
		{name: "unknown option", stdin: hello, args: []string{"-q"}, code: 1, stderr: "unknown option -q"},
		{name: "no -v", stdin: hello, args: []string{"-v"}, code: 1, stderr: "unknown option -v"},
		{name: "long option", stdin: hello, args: []string{"--bogus"}, code: 1, stderr: "unknown option --bogus"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "bunzip2", string(tc.stdin), tc.args...)
			assertExitCode(t, r, tc.code)
			assertStdout(t, r, tc.stdout)
			if tc.stderr != "" {
				assertStderrContains(t, r, tc.stderr)
			} else if r.stderr != "" {
				t.Errorf("unexpected stderr %q", r.stderr)
			}
		})
	}
}

// TestBzcat covers bzcat as bunzip2 -c: stdout, operands, stdin, no options.
func TestBzcat(t *testing.T) {
	dir := t.TempDir()
	hello := bz2Fixture(t, bz2HelloFixture)
	writeFile(t, dir, "a.bz2", string(hello))
	writeFile(t, dir, "b.bin", string(bz2Fixture(t, bz2MultiFixture)))

	r := runBuiltinIn(t, dir, "bzcat", "a.bz2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
	if _, err := os.Lstat(filepath.Join(dir, "a.bz2")); err != nil {
		t.Errorf("bzcat removed its input: %v", err)
	}
	mustNotExist(t, filepath.Join(dir, "a"))

	// No suffix is needed and several operands are concatenated.
	assertStdout(t, runBuiltinIn(t, dir, "bzcat", "b.bin", "a.bz2"), "one\ntwo\nthree\nhello\n")

	// No operands reads stdin, and "-" is stdin.
	assertStdout(t, runBuiltinStdin(t, "bzcat", string(hello)), "hello\n")
	assertStdout(t, runBuiltinStdin(t, "bzcat", string(hello), "-"), "hello\n")

	// bzcat has no options of its own.
	r = runBuiltinIn(t, dir, "bzcat", "-c", "a.bz2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown option -c")
	assertStdout(t, r, "")

	// A bad stream is an error, not silence.
	writeFile(t, dir, "bad.bz2", "not bzip2\n")
	r = runBuiltinIn(t, dir, "bzcat", "bad.bz2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "not bzip2 data")
}

func TestBunzip2FileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt.bz2", string(bz2Fixture(t, bz2HelloFixture)))

	assertExitCode(t, runBuiltinIn(t, dir, "bunzip2", "f.txt.bz2"), 0)
	mustNotExist(t, filepath.Join(dir, "f.txt.bz2"))
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "hello\n" {
		t.Errorf("content = %q", got)
	}
}

func TestBunzip2MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.bz2", string(bz2Fixture(t, bz2HelloFixture)))
	writeFile(t, dir, "b.bz2", string(bz2Fixture(t, bz2EmptyFixture)))

	assertExitCode(t, runBuiltinIn(t, dir, "bunzip2", "a.bz2", "b.bz2"), 0)
	if got := fileContent(t, filepath.Join(dir, "a")); got != "hello\n" {
		t.Errorf("a = %q", got)
	}
	if got := fileContent(t, filepath.Join(dir, "b")); got != "" {
		t.Errorf("b = %q", got)
	}
	mustNotExist(t, filepath.Join(dir, "a.bz2"))
	mustNotExist(t, filepath.Join(dir, "b.bz2"))

	// One failing operand does not stop the rest, and sets the exit status.
	writeFile(t, dir, "c.bz2", string(bz2Fixture(t, bz2MultiFixture)))
	r := runBuiltinIn(t, dir, "bunzip2", "nope.bz2", "c.bz2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "nope.bz2")
	if got := fileContent(t, filepath.Join(dir, "c")); got != "one\ntwo\nthree\n" {
		t.Errorf("c = %q", got)
	}
}

func TestBunzip2Keep(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "k.txt.bz2", string(bz2Fixture(t, bz2HelloFixture)))

	assertExitCode(t, runBuiltinIn(t, dir, "bunzip2", "-k", "k.txt.bz2"), 0)
	if got := fileContent(t, filepath.Join(dir, "k.txt")); got != "hello\n" {
		t.Errorf("output = %q", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "k.txt.bz2")); err != nil {
		t.Errorf("-k removed the input: %v", err)
	}
}

func TestBunzip2StdoutKeepsInput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt.bz2", string(bz2Fixture(t, bz2HelloFixture)))

	r := runBuiltinIn(t, dir, "bunzip2", "-c", "f.txt.bz2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "hello\n")
	mustNotExist(t, filepath.Join(dir, "f.txt"))
	if _, err := os.Lstat(filepath.Join(dir, "f.txt.bz2")); err != nil {
		t.Errorf("-c removed the input: %v", err)
	}

	// -c does not need a known suffix.
	writeFile(t, dir, "blob", string(bz2Fixture(t, bz2HelloFixture)))
	assertStdout(t, runBuiltinIn(t, dir, "bunzip2", "-c", "blob"), "hello\n")
}

func TestBunzip2RefusesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "precious\n")
	writeFile(t, dir, "f.txt.bz2", string(bz2Fixture(t, bz2HelloFixture)))

	r := runBuiltinIn(t, dir, "bunzip2", "f.txt.bz2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "already exists")
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "precious\n" {
		t.Errorf("existing output changed: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "f.txt.bz2")); err != nil {
		t.Errorf("input removed after refusal: %v", err)
	}

	r = runBuiltinIn(t, dir, "bunzip2", "-f", "f.txt.bz2")
	assertExitCode(t, r, 0)
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "hello\n" {
		t.Errorf("-f output = %q", got)
	}
	mustNotExist(t, filepath.Join(dir, "f.txt.bz2"))

	// A directory in the way is not replaced either.
	writeFile(t, dir, "d/x", "")
	writeFile(t, dir, "d.bz2", string(bz2Fixture(t, bz2HelloFixture)))
	assertExitCode(t, runBuiltinIn(t, dir, "bunzip2", "d.bz2"), 1)
}

func TestBunzip2Suffixes(t *testing.T) {
	dir := t.TempDir()
	hello := string(bz2Fixture(t, bz2HelloFixture))
	for _, name := range []string{"a.bz2", "b.tbz2", "c.tbz", "d.bz"} {
		writeFile(t, dir, name, hello)
		assertExitCode(t, runBuiltinIn(t, dir, "bunzip2", name), 0)
	}
	for _, name := range []string{"a", "b.tar", "c.tar", "d"} {
		if got := fileContent(t, filepath.Join(dir, name)); got != "hello\n" {
			t.Errorf("%s content = %q", name, got)
		}
	}
	// A name that is only a suffix keeps it, so it is not stripped.
	writeFile(t, dir, ".bz2", hello)
	r := runBuiltinIn(t, dir, "bunzip2", ".bz2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown suffix")
}

func TestBunzip2UnknownSuffix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "data.bin", string(bz2Fixture(t, bz2HelloFixture)))

	r := runBuiltinIn(t, dir, "bunzip2", "data.bin")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown suffix -- ignored (use -c to write to stdout)")
	assertStdout(t, r, "")
	if got := fileContent(t, filepath.Join(dir, "data.bin")); got == "hello\n" {
		t.Errorf("input decompressed without -f")
	}
}

// TestBunzip2ForceReplacesOriginal is toybox's -f "replace original": a name
// with no known suffix is decompressed onto itself.
func TestBunzip2ForceReplacesOriginal(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "data.bin", string(bz2Fixture(t, bz2HelloFixture)))
	if err := os.Chmod(p, 0755); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(1997, 8, 29, 1, 2, 3, 0, time.UTC)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	assertExitCode(t, runBuiltinIn(t, dir, "bunzip2", "-f", "data.bin"), 0)
	if got := fileContent(t, p); got != "hello\n" {
		t.Errorf("replaced content = %q", got)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("output missing after -f: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0755 {
		t.Errorf("mode = %o, want 755", perm)
	}
	if !info.ModTime().Equal(mtime) {
		t.Errorf("mtime = %s, want %s", info.ModTime(), mtime)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("-f left extra files: %v", entries)
	}

	// -f with a suffix still strips it and keeps the original gone.
	writeFile(t, dir, "s.bz2", string(bz2Fixture(t, bz2HelloFixture)))
	assertExitCode(t, runBuiltinIn(t, dir, "bunzip2", "-f", "s.bz2"), 0)
	if got := fileContent(t, filepath.Join(dir, "s")); got != "hello\n" {
		t.Errorf("-f with a suffix = %q", got)
	}
}

func TestBunzip2TestFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ok.bz2", string(bz2Fixture(t, bz2HelloFixture)))
	writeFile(t, dir, "bad.bz2", "not bzip2\n")

	r := runBuiltinIn(t, dir, "bunzip2", "-t", "ok.bz2")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "")
	mustNotExist(t, filepath.Join(dir, "ok"))
	if _, err := os.Lstat(filepath.Join(dir, "ok.bz2")); err != nil {
		t.Errorf("-t removed the input: %v", err)
	}

	r = runBuiltinIn(t, dir, "bunzip2", "-t", "bad.bz2")
	assertExitCode(t, r, 1)
	assertStdout(t, r, "")
	assertStderrContains(t, r, "not bzip2 data")
	mustNotExist(t, filepath.Join(dir, "bad"))
	if _, err := os.Lstat(filepath.Join(dir, "bad.bz2")); err != nil {
		t.Errorf("-t removed the input: %v", err)
	}
}

// TestBunzip2TruncatedAtEveryOffset cuts a stream at every length: each prefix
// must fail with exit status 1, no panic, no output file and no temp file.
func TestBunzip2TruncatedAtEveryOffset(t *testing.T) {
	full := bz2Fixture(t, bz2TextFixture)
	dir := t.TempDir()
	inPath := filepath.Join(dir, "cut.bz2")

	for cut := 0; cut < len(full); cut++ {
		data := string(full[:cut])
		if err := os.WriteFile(inPath, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}

		r := runBuiltinStdinIn(t, dir, "bunzip2", data, "cut.bz2")
		if r.code != 1 {
			t.Fatalf("cut at %d: exit %d, want 1 (stderr %q)", cut, r.code, r.stderr)
		}
		if r.stderr == "" {
			t.Fatalf("cut at %d: no diagnostic", cut)
		}
		if r.stdout != "" {
			t.Errorf("cut at %d: wrote %q before failing", cut, r.stdout)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "cut.bz2" {
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Fatalf("cut at %d: leftover files %v", cut, names)
		}
	}
}

// TestBunzip2DamagedData checks that corrupt data is an error, not silence, and
// that the input survives with no output next to it.
func TestBunzip2DamagedData(t *testing.T) {
	text := bz2Fixture(t, bz2TextFixture)
	corruptCRC := append([]byte(nil), text...)
	corruptCRC[len(corruptCRC)-1] ^= 0xff
	corruptBlock := append([]byte(nil), text...)
	corruptBlock[len(corruptBlock)/2] ^= 0x10

	tests := []struct {
		name  string
		data  []byte
		subst string
	}{
		{"empty", nil, "not bzip2 data"},
		{"not bzip2", []byte("the quick brown fox\n"), "not bzip2 data"},
		{"truncated", text[:len(text)-5], "unexpected end of file"},
		{"corrupt crc", corruptCRC, "invalid checksum"},
		{"corrupt block", corruptBlock, ""},
		{"trailing garbage", append(append([]byte(nil), text...), "\x00\x00"...), "trailing data"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "bad.bz2", string(tc.data))

			r := runBuiltinIn(t, dir, "bunzip2", "bad.bz2")
			assertExitCode(t, r, 1)
			if tc.subst != "" {
				assertStderrContains(t, r, tc.subst)
			} else if r.stderr == "" {
				t.Errorf("no diagnostic for corrupt data")
			}
			mustNotExist(t, filepath.Join(dir, "bad"))
			if got := fileContent(t, filepath.Join(dir, "bad.bz2")); got != string(tc.data) {
				t.Errorf("input changed")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				t.Errorf("leftover files after failure: %v", names)
			}
		})
	}
}

func TestBunzip2DirectoryAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "bunzip2", "sub")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Is a directory")

	r = runBuiltinIn(t, dir, "bunzip2", "nope.bz2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "nope.bz2")

	r = runBuiltinIn(t, dir, "bunzip2", "nope.bz2", "also-missing.bz2")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "also-missing.bz2")
}

func TestBunzip2Help(t *testing.T) {
	for _, name := range []string{"bunzip2", "bzcat"} {
		r := runBuiltin(t, name, "--help")
		assertExitCode(t, r, 0)
		if !strings.Contains(r.stdout, "Usage: "+name) {
			t.Errorf("%s --help = %q", name, r.stdout)
		}
		r = runBuiltin(t, name, "--version")
		assertExitCode(t, r, 0)
		assertStdoutContains(t, r, name)
	}
	// bzcat documents no options but -h/--help and --version.
	if strings.Contains(runBuiltin(t, "bzcat", "--help").stdout, "-c") {
		t.Errorf("bzcat --help lists -c")
	}
}

// TestBunzip2StreamReader exercises the decoder directly, including the empty
// input and a stream followed by another.
func TestBunzip2StreamReader(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
		err  bool
	}{
		{"single", bz2Fixture(t, bz2HelloFixture), "hello\n", false},
		{"concatenated", bz2Fixture(t, bz2MultiFixture), "one\ntwo\nthree\n", false},
		{"empty payload", bz2Fixture(t, bz2EmptyFixture), "", false},
		{"text", bz2Fixture(t, bz2TextFixture), strings.Repeat(bz2TextPayload, 50), false},
		{"empty input", nil, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := bunzip2Stream(bytes.NewReader(tc.data), &out)
			if tc.err && err == nil {
				t.Fatal("expected an error")
			}
			if !tc.err && err != nil {
				t.Fatalf("bunzip2Stream: %v", err)
			}
			if got := out.String(); got != tc.want {
				t.Errorf("output = %q, want %q", got, tc.want)
			}
		})
	}

	// A stream that decodes to nothing still counts as a stream.
	if err := bunzip2Stream(bytes.NewReader(bz2Fixture(t, bz2EmptyFixture)), io.Discard); err != nil {
		t.Errorf("empty payload: %v", err)
	}
}
