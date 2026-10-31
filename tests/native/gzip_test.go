package native

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gzip/gunzip/zcat are RFC 1952 streams implemented with compress/gzip; these
// checks run through the multicall registry, mirroring the deeper table tests in
// internal/builtins/gzip_test.go.
func TestGzip_RoundTripFile(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "hello\n")

	wantSuccess(t, runIn(t, dir, "gzip", "f.txt"))
	if _, err := os.Lstat(filepath.Join(dir, "f.txt")); err == nil {
		t.Error("gzip kept the input")
	}
	if _, err := os.Lstat(filepath.Join(dir, "f.txt.gz")); err != nil {
		t.Fatalf("gzip did not create f.txt.gz: %v", err)
	}
	wantOut(t, runIn(t, dir, "zcat", "f.txt.gz"), "hello\n")

	wantSuccess(t, runIn(t, dir, "gunzip", "f.txt.gz"))
	if got := readFile(t, filepath.Join(dir, "f.txt")); got != "hello\n" {
		t.Errorf("round trip = %q", got)
	}
}

func TestGzip_StdinRoundTrip(t *testing.T) {
	code, comp, stderr := runStdoutBytes(t, "gzip", "stream me\n")
	if code != 0 {
		t.Fatalf("gzip exit = %d (%s)", code, stderr)
	}
	if !strings.HasPrefix(string(comp), "\x1f\x8b\x08") {
		t.Fatalf("not a gzip stream: %q", comp)
	}
	r := runStdinBytes(t, "gunzip", comp)
	wantSuccess(t, r)
	wantOut(t, r, "stream me\n")
}

func TestGzip_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "new\n")
	mkFile(t, dir, "f.txt.gz", "old\n")

	r := runIn(t, dir, "gzip", "f.txt")
	wantFail(t, r)
	wantErrContains(t, r, "already exists")
	if got := readFile(t, filepath.Join(dir, "f.txt.gz")); got != "old\n" {
		t.Errorf("output overwritten without -f: %q", got)
	}
	wantSuccess(t, runIn(t, dir, "gzip", "-f", "f.txt"))
	wantOut(t, runIn(t, dir, "zcat", "f.txt.gz"), "new\n")
}

func TestGzip_KeepAndStdout(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.txt", "keep\n")

	wantSuccess(t, runIn(t, dir, "gzip", "-k", "f.txt"))
	if got := readFile(t, filepath.Join(dir, "f.txt")); got != "keep\n" {
		t.Errorf("-k lost the input: %q", got)
	}
	os.Remove(filepath.Join(dir, "f.txt.gz"))

	r := runIn(t, dir, "gzip", "-c", "f.txt")
	wantSuccess(t, r)
	if _, err := os.Lstat(filepath.Join(dir, "f.txt.gz")); err == nil {
		t.Error("-c created f.txt.gz")
	}
	if !strings.HasPrefix(r.stdout, "\x1f\x8b") {
		t.Errorf("-c output is not compressed: %q", r.stdout)
	}
}

func TestGzip_ConcatenatedMembers(t *testing.T) {
	dir := t.TempDir()
	var buf strings.Builder
	for _, part := range []string{"one\n", "two\n"} {
		code, comp, stderr := runStdoutBytes(t, "gzip", part)
		if code != 0 {
			t.Fatalf("gzip exit = %d (%s)", code, stderr)
		}
		buf.Write(comp)
	}
	mkFile(t, dir, "both.gz", buf.String())
	wantOut(t, runIn(t, dir, "zcat", "both.gz"), "one\ntwo\n")
}

func TestGzip_UnknownOptions(t *testing.T) {
	for _, args := range [][]string{{"-l"}, {"-Z"}} {
		r := run(t, "gzip", args...)
		wantFail(t, r)
		wantErrContains(t, r, "unknown option")
	}
	r := run(t, "gunzip", "-n")
	wantFail(t, r)
	wantErrContains(t, r, "unknown option")
}

func TestGzip_BadStream(t *testing.T) {
	r := runStdinBytes(t, "gunzip", []byte("not compressed\n"))
	wantFail(t, r)
	wantErrContains(t, r, "not in gzip format")
}
