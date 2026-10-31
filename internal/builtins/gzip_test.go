package builtins

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gunzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	r := runBuiltinStdin(t, "gunzip", string(data))
	assertExitCode(t, r, 0)
	return []byte(r.stdout)
}

func headerOf(t *testing.T, path string) gzip.Header {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("not a gzip file (%s): %v", path, err)
	}
	defer zr.Close()
	return zr.Header
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("%s should not exist", path)
	}
}

func fileContent(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestGzipStdinRoundTrip covers stdin->stdout compression at every supported
// level and decompression with gunzip and zcat.
func TestGzipStdinRoundTrip(t *testing.T) {
	payload := strings.Repeat("the quick brown fox jumps over the lazy dog\n", 500)
	for _, level := range []string{"", "-1", "-2", "-3", "-4", "-5", "-6", "-7", "-8", "-9"} {
		args := []string{}
		if level != "" {
			args = append(args, level)
		}
		comp := runBuiltinStdin(t, "gzip", payload, args...)
		assertExitCode(t, comp, 0)
		if !strings.HasPrefix(comp.stdout, "\x1f\x8b\x08") {
			t.Fatalf("%s: output is not a gzip stream: %q", level, comp.stdout[:8])
		}
		if len(comp.stdout) >= len(payload) {
			t.Errorf("%s: compressed size %d is not smaller than %d", level, len(comp.stdout), len(payload))
		}
		assertStdout(t, runBuiltinStdin(t, "gunzip", comp.stdout), payload)
		assertStdout(t, runBuiltinStdin(t, "zcat", comp.stdout), payload)
		assertStdout(t, runBuiltinStdin(t, "gzip", comp.stdout, "-d"), payload)
		assertStdout(t, runBuiltinStdin(t, "gzip", comp.stdout, "-dc"), payload)
	}
}

// TestGzipLevelAffectsSize checks that -9 really compresses better than -1.
func TestGzipLevelAffectsSize(t *testing.T) {
	var b strings.Builder
	x := uint32(12345)
	for i := 0; i < 40000; i++ {
		x = x*1664525 + 1013904223
		b.WriteByte(byte(x>>24)%26 + 'a')
	}
	payload := b.String()
	fast := runBuiltinStdin(t, "gzip", payload, "-1")
	best := runBuiltinStdin(t, "gzip", payload, "-9")
	assertExitCode(t, fast, 0)
	assertExitCode(t, best, 0)
	if len(best.stdout) >= len(fast.stdout) {
		t.Errorf("-9 (%d bytes) is not smaller than -1 (%d bytes)", len(best.stdout), len(fast.stdout))
	}
}

func TestGzipEmptyStdin(t *testing.T) {
	comp := runBuiltinStdin(t, "gzip", "")
	assertExitCode(t, comp, 0)
	assertStdout(t, runBuiltinStdin(t, "gunzip", comp.stdout), "")
}

func TestGzipDashesMeanStdin(t *testing.T) {
	comp := runBuiltinStdin(t, "gzip", "hello\n", "-")
	assertExitCode(t, comp, 0)
	assertStdout(t, runBuiltinStdin(t, "gunzip", comp.stdout, "-"), "hello\n")
}

// TestGzipFileRoundTrip replaces a file and puts it back.
func TestGzipFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "hello\n")
	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "f.txt"), 0)
	mustNotExist(t, filepath.Join(dir, "f.txt"))
	header := headerOf(t, filepath.Join(dir, "f.txt.gz"))
	if header.Name != "f.txt" {
		t.Errorf("stored name = %q, want f.txt", header.Name)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "f.txt.gz"), 0)
	mustNotExist(t, filepath.Join(dir, "f.txt.gz"))
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "hello\n" {
		t.Errorf("round trip content = %q", got)
	}
}

func TestGzipMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a", "A\n")
	writeFile(t, dir, "b", "B\n")
	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "a", "b"), 0)
	mustNotExist(t, filepath.Join(dir, "a"))
	mustNotExist(t, filepath.Join(dir, "b"))
	r := runBuiltinIn(t, dir, "zcat", "a.gz", "b.gz")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "A\nB\n")
}

func TestGzipKeep(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "hello\n")
	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "-k", "f.txt"), 0)
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "hello\n" {
		t.Errorf("input changed: %q", got)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "-k", "f.txt.gz"), 1)
	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "-kf", "f.txt.gz"), 0)
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "hello\n" {
		t.Errorf("input changed: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "f.txt.gz")); err != nil {
		t.Errorf("-k removed the compressed input: %v", err)
	}
}

func TestGzipStdoutKeepsInput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "hello\n")
	r := runBuiltinIn(t, dir, "gzip", "-c", "f.txt")
	assertExitCode(t, r, 0)
	mustNotExist(t, filepath.Join(dir, "f.txt.gz"))
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "hello\n" {
		t.Errorf("input changed: %q", got)
	}
	assertStdout(t, runBuiltinStdin(t, "gunzip", r.stdout, "-c"), "hello\n")

	// zcat is gzip -dc and also leaves the file alone.
	writeFile(t, dir, "z.gz", r.stdout)
	assertStdout(t, runBuiltinIn(t, dir, "zcat", "z.gz"), "hello\n")
	if _, err := os.Lstat(filepath.Join(dir, "z.gz")); err != nil {
		t.Errorf("zcat removed its input: %v", err)
	}
}

// TestGzipRefusesExistingOutput is the no-prompt overwrite policy.
func TestGzipRefusesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "new data\n")
	writeFile(t, dir, "f.txt.gz", "precious\n")

	r := runBuiltinIn(t, dir, "gzip", "f.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "already exists")
	if got := fileContent(t, filepath.Join(dir, "f.txt.gz")); got != "precious\n" {
		t.Errorf("existing output changed: %q", got)
	}
	if got := fileContent(t, filepath.Join(dir, "f.txt")); got != "new data\n" {
		t.Errorf("input removed after refusal: %q", got)
	}

	r = runBuiltinIn(t, dir, "gzip", "-f", "f.txt")
	assertExitCode(t, r, 0)
	mustNotExist(t, filepath.Join(dir, "f.txt"))
	assertStdout(t, runBuiltinIn(t, dir, "zcat", "f.txt.gz"), "new data\n")

	// Decompression refuses to clobber too.
	writeFile(t, dir, "g.txt", "keep me\n")
	writeFile(t, dir, "g.txt.gz", "")
	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "-k", "g.txt"), 1)
	comp := runBuiltinStdin(t, "gzip", "from archive\n")
	writeFile(t, dir, "g.txt.gz", comp.stdout)
	r = runBuiltinIn(t, dir, "gunzip", "g.txt.gz")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "already exists")
	if got := fileContent(t, filepath.Join(dir, "g.txt")); got != "keep me\n" {
		t.Errorf("existing file changed: %q", got)
	}

	// A directory in the way is not replaced either.
	writeFile(t, dir, "d/f", "")
	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "d"), 1)
}

// TestGzipConcatenatedMembers decompresses a stream of several gzip members.
func TestGzipConcatenatedMembers(t *testing.T) {
	var buf bytes.Buffer
	for _, part := range []string{"one\n", "two\n", "three\n"} {
		comp := runBuiltinStdin(t, "gzip", part)
		assertExitCode(t, comp, 0)
		buf.WriteString(comp.stdout)
	}
	assertStdout(t, runBuiltinStdin(t, "gunzip", buf.String()), "one\ntwo\nthree\n")

	dir := t.TempDir()
	writeFile(t, dir, "multi.gz", buf.String())
	assertStdout(t, runBuiltinIn(t, dir, "zcat", "multi.gz"), "one\ntwo\nthree\n")
}

func TestGzipDamagedStreams(t *testing.T) {
	comp := runBuiltinStdin(t, "gzip", strings.Repeat("data\n", 100))
	assertExitCode(t, comp, 0)
	good := []byte(comp.stdout)

	corruptCRC := append([]byte(nil), good...)
	corruptCRC[len(corruptCRC)-6] ^= 0xff // inside the CRC32 trailer

	truncated := good[:len(good)/2]

	tests := []struct {
		name  string
		data  []byte
		subst string
	}{
		{"crc", corruptCRC, "invalid checksum"},
		{"truncated", truncated, "unexpected end of file"},
		{"not gzip", []byte("plain text\n"), "not in gzip format"},
		{"empty", nil, "not in gzip format"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := runBuiltinStdin(t, "gunzip", string(tc.data))
			assertExitCode(t, r, 1)
			assertStderrContains(t, r, tc.subst)

			r = runBuiltinStdin(t, "gunzip", string(tc.data), "-t")
			assertExitCode(t, r, 1)
			assertStdout(t, r, "")

			// A file survives a failed decompression untouched, with no output.
			dir := t.TempDir()
			writeFile(t, dir, "bad.gz", string(tc.data))
			r = runBuiltinIn(t, dir, "gunzip", "bad.gz")
			assertExitCode(t, r, 1)
			if _, err := os.Lstat(filepath.Join(dir, "bad.gz")); err != nil {
				t.Errorf("input removed on failure: %v", err)
			}
			mustNotExist(t, filepath.Join(dir, "bad"))
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

func TestGzipTestFlag(t *testing.T) {
	dir := t.TempDir()
	comp := runBuiltinStdin(t, "gzip", "payload\n")
	writeFile(t, dir, "ok.gz", comp.stdout)

	for _, name := range []string{"gzip", "gunzip", "zcat"} {
		r := runBuiltinIn(t, dir, name, "-t", "ok.gz")
		assertExitCode(t, r, 0)
		assertStdout(t, r, "")
	}
	// -t never writes a file and never removes the input.
	if _, err := os.Lstat(filepath.Join(dir, "ok")); err == nil {
		t.Errorf("-t created an output file")
	}
	if _, err := os.Lstat(filepath.Join(dir, "ok.gz")); err != nil {
		t.Errorf("-t removed the input: %v", err)
	}
}

func TestGzipUnknownSuffix(t *testing.T) {
	dir := t.TempDir()
	comp := runBuiltinStdin(t, "gzip", "payload\n")
	writeFile(t, dir, "data.bin", comp.stdout)

	r := runBuiltinIn(t, dir, "gunzip", "data.bin")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown suffix")
	if _, err := os.Lstat(filepath.Join(dir, "data.bin")); err != nil {
		t.Errorf("input removed on unknown suffix: %v", err)
	}

	// -c does not need a suffix.
	assertStdout(t, runBuiltinIn(t, dir, "gunzip", "-c", "data.bin"), "payload\n")
	assertStdout(t, runBuiltinIn(t, dir, "zcat", "data.bin"), "payload\n")
}

func TestGzipSuffixes(t *testing.T) {
	dir := t.TempDir()
	comp := runBuiltinStdin(t, "gzip", "tar-ish\n")
	writeFile(t, dir, "a.gz", comp.stdout)
	writeFile(t, dir, "b.tgz", comp.stdout)
	writeFile(t, dir, "c.z", comp.stdout)

	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "a.gz"), 0)
	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "b.tgz"), 0)
	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "c.z"), 0)
	for _, name := range []string{"a", "b.tar", "c"} {
		if got := fileContent(t, filepath.Join(dir, name)); got != "tar-ish\n" {
			t.Errorf("%s content = %q", name, got)
		}
	}
}

func TestGzipPreservesModeAndTime(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "m.txt", "content\n")
	if err := os.Chmod(p, 0755); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(1999, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "m.txt"), 0)
	info, err := os.Stat(filepath.Join(dir, "m.txt.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0755 {
		t.Errorf("compressed mode = %o, want 755", perm)
	}
	if !info.ModTime().Equal(mtime) {
		t.Errorf("compressed mtime = %s, want %s", info.ModTime(), mtime)
	}
	if h := headerOf(t, filepath.Join(dir, "m.txt.gz")); !h.ModTime.Equal(mtime) {
		t.Errorf("header mtime = %s, want %s", h.ModTime, mtime)
	}

	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "m.txt.gz"), 0)
	info, err = os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0755 {
		t.Errorf("restored mode = %o, want 755", perm)
	}
	if !info.ModTime().Equal(mtime) {
		t.Errorf("restored mtime = %s, want %s", info.ModTime(), mtime)
	}
}

func TestGzipNoNameFlag(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "n.txt", "content\n")
	mtime := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "-k", "-n", "n.txt"), 0)
	h := headerOf(t, filepath.Join(dir, "n.txt.gz"))
	if h.Name != "" {
		t.Errorf("-n stored a name: %q", h.Name)
	}
	if !h.ModTime.IsZero() {
		t.Errorf("-n stored an mtime: %s", h.ModTime)
	}
}

// TestGzipNameFlagRestores covers -N: mtime from the header, and the header
// name as the output name when the file has no gzip suffix.
func TestGzipNameFlagRestores(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "orig.txt", "restored\n")
	mtime := time.Date(1987, 4, 3, 2, 1, 0, 0, time.UTC)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "gzip", "orig.txt"), 0)

	// Move it to a suffixless name and give it a different file mtime.
	blob := fileContent(t, filepath.Join(dir, "orig.txt.gz"))
	os.Remove(filepath.Join(dir, "orig.txt.gz"))
	writeFile(t, dir, "blob.bin", blob)
	later := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(dir, "blob.bin"), later, later); err != nil {
		t.Fatal(err)
	}

	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "-N", "blob.bin"), 0)
	info, err := os.Stat(filepath.Join(dir, "orig.txt"))
	if err != nil {
		t.Fatalf("-N did not use the stored name: %v", err)
	}
	if !info.ModTime().Equal(mtime) {
		t.Errorf("-N mtime = %s, want %s", info.ModTime(), mtime)
	}
	if got := fileContent(t, filepath.Join(dir, "orig.txt")); got != "restored\n" {
		t.Errorf("-N content = %q", got)
	}

	// -N never lets a stored path escape the input's directory.
	writeFile(t, dir, "sub/keep", "")
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, 1)
	if err != nil {
		t.Fatal(err)
	}
	zw.Name = "../../escape.txt"
	zw.Write([]byte("nope\n"))
	zw.Close()
	writeFile(t, dir, "sub/evil.gz", buf.String())
	os.Remove(filepath.Join(dir, "sub/evil.gz"))
	writeFile(t, dir, "sub/evil.bin", buf.String())
	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "-N", "sub/evil.bin"), 0)
	if _, err := os.Lstat(filepath.Join(dir, "escape.txt")); err == nil {
		t.Errorf("stored name escaped the directory")
	}
	if got := fileContent(t, filepath.Join(dir, "sub/escape.txt")); got != "nope\n" {
		t.Errorf("escaped-name content = %q", got)
	}
}

func TestGzipUnknownOptions(t *testing.T) {
	tests := [][]string{
		{"-l"},
		{"--list"},
		{"-Z"},
		{"-1", "-9"},
		{"--bogus"},
	}
	for _, args := range tests {
		r := runBuiltinStdin(t, "gzip", "x", args...)
		assertExitCode(t, r, 1)
		if !strings.Contains(r.stderr, "unknown option") && !strings.Contains(r.stderr, "one compression level") {
			t.Errorf("%v: unexpected stderr %q", args, r.stderr)
		}
	}
	// -n is gzip's, not gunzip's/zcat's, like toybox.
	r := runBuiltinStdin(t, "gunzip", "x", "-n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown option")
	r = runBuiltinStdin(t, "zcat", "x", "-n")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "unknown option")
}

func TestGzipDirectoryAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	r := runBuiltinIn(t, dir, "gzip", "sub")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Is a directory")

	r = runBuiltinIn(t, dir, "gzip", "nope")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "nope")

	// A missing operand does not stop the remaining ones.
	writeFile(t, dir, "ok.txt", "ok\n")
	r = runBuiltinIn(t, dir, "gzip", "nope", "ok.txt")
	assertExitCode(t, r, 1)
	if _, err := os.Lstat(filepath.Join(dir, "ok.txt.gz")); err != nil {
		t.Errorf("valid operand skipped after an error: %v", err)
	}
	assertExitCode(t, runBuiltinIn(t, dir, "gunzip", "nope.gz", "ok.txt.gz"), 1)
}

func TestGzipHelp(t *testing.T) {
	for _, name := range []string{"gzip", "gunzip", "zcat"} {
		r := runBuiltin(t, name, "--help")
		assertExitCode(t, r, 0)
		if !strings.Contains(r.stdout, "Usage: "+name) {
			t.Errorf("%s --help = %q", name, r.stdout)
		}
	}
}

// TestGzipStdinIsNotGzipWhenReadingTerminalDocuments the -f requirement only
// through the helper, since faking a tty needs a pty.
func TestGzipTerminalHelpers(t *testing.T) {
	var buf bytes.Buffer
	if writerIsTerminal(&buf) {
		t.Error("a bytes.Buffer must not look like a terminal")
	}
	if isTerminal(strings.NewReader("")) {
		t.Error("a strings.Reader must not look like a terminal")
	}
}

func TestGunzipStreamReader(t *testing.T) {
	comp := runBuiltinStdin(t, "gzip", "streamed\n")
	var out bytes.Buffer
	if err := gunzipStream(bytes.NewReader([]byte(comp.stdout)), &out); err != nil {
		t.Fatalf("gunzipStream: %v", err)
	}
	if out.String() != "streamed\n" {
		t.Errorf("gunzipStream = %q", out.String())
	}
	if err := gunzipStream(bytes.NewReader(nil), io.Discard); err == nil {
		t.Error("gunzipStream accepted empty input")
	}
}
