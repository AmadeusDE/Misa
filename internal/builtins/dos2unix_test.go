// Behaviour adapted from toybox dos2unix.c (0BSD).
package builtins

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// oneByteReader returns a single byte per Read call, so a CR and its LF land in
// separate reads.
type oneByteReader struct{ data []byte }

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}

// errorReader hands out data, then fails instead of reporting EOF.
type errorReader struct {
	data []byte
	err  error
}

func (r *errorReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

// alwaysFailWriter fails every write.
type alwaysFailWriter struct{}

func (alwaysFailWriter) Write(p []byte) (int, error) { return 0, errors.New("write failed") }

func runDosUnixCtx(t *testing.T, name string, in io.Reader, out io.Writer, args ...string) (int, string) {
	t.Helper()
	cmd, ok := Lookup(name)
	if !ok {
		t.Fatalf("builtin %q not registered", name)
	}
	var stderr bytes.Buffer
	ctx := &Context{Dir: t.TempDir(), Args: args, Stdout: out, Stderr: &stderr, Stdin: in}
	return cmd.Run(ctx), stderr.String()
}

func convertString(t *testing.T, toDOS bool, input string) string {
	t.Helper()
	var out bytes.Buffer
	if err := convertNewlines(&out, strings.NewReader(input), toDOS); err != nil {
		t.Fatalf("convertNewlines: %v", err)
	}
	return out.String()
}

func TestDos2UnixRegistered(t *testing.T) {
	for _, name := range []string{"dos2unix", "unix2dos"} {
		if _, ok := Lookup(name); !ok {
			t.Errorf("%s not registered", name)
		}
	}
}

func TestDos2UnixConversion(t *testing.T) {
	tests := []struct {
		name  string
		toDOS bool
		in    string
		want  string
	}{
		{"dos empty", false, "", ""},
		{"dos crlf", false, "\r\n", "\n"},
		{"dos pairs", false, "a\r\nb\r\n", "a\nb\n"},
		{"dos already unix", false, "a\nb\n", "a\nb\n"},
		{"dos no final newline", false, "abc", "abc"},
		// A CR that is not part of CRLF survives, including at end of input.
		{"dos lone cr", false, "a\rb", "a\rb"},
		{"dos trailing cr", false, "a\r", "a\r"},
		{"dos doubled cr", false, "\r\r", "\r\r"},
		{"dos cr cr lf", false, "\r\r\n", "\r\n"},
		{"dos cr lf cr lf", false, "\r\n\r\n", "\n\n"},
		{"dos mixed", false, "one\r\ntwo\rthree\r\n", "one\ntwo\rthree\n"},
		{"unix empty", true, "", ""},
		{"unix lf", true, "\n", "\r\n"},
		{"unix lfs", true, "a\nb\n", "a\r\nb\r\n"},
		// Existing CRLF pairs are not doubled.
		{"unix existing crlf", true, "\r\n", "\r\n"},
		{"unix existing crlfs", true, "a\r\nb\r\n", "a\r\nb\r\n"},
		{"unix no final newline", true, "abc", "abc"},
		{"unix lone cr", true, "a\rb", "a\rb"},
		{"unix trailing cr", true, "a\r", "a\r"},
		{"unix cr cr lf", true, "\r\r\n", "\r\r\n"},
		{"unix mixed", true, "one\r\ntwo\nthree\n", "one\r\ntwo\r\nthree\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertString(t, tt.toDOS, tt.in); got != tt.want {
				t.Errorf("convert(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestDos2UnixSplitCRLF feeds one byte per read so CR and LF never share a buffer.
func TestDos2UnixSplitCRLF(t *testing.T) {
	tests := []struct {
		name  string
		toDOS bool
		in    string
		want  string
	}{
		{"dos", false, "a\r\nb\r\n\r\r\nc\r", "a\nb\n\r\nc\r"},
		{"unix", true, "a\nb\r\n\n\r", "a\r\nb\r\n\r\n\r"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := convertNewlines(&out, &oneByteReader{data: []byte(tt.in)}, tt.toDOS); err != nil {
				t.Fatalf("convertNewlines: %v", err)
			}
			if out.String() != tt.want {
				t.Errorf("got %q, want %q", out.String(), tt.want)
			}
		})
	}
}

func TestDos2UnixByteTransparency(t *testing.T) {
	// NUL, invalid UTF-8, a UTF-8 BOM and arbitrary high bytes pass through.
	input := "\x00\xff\xfe\xef\xbb\xbf\r\n\x80abc\x00\r\n"
	want := "\x00\xff\xfe\xef\xbb\xbf\n\x80abc\x00\n"
	if got := convertString(t, false, input); got != want {
		t.Errorf("dos2unix got %q, want %q", got, want)
	}
	wantBack := "\x00\xff\xfe\xef\xbb\xbf\r\n\x80abc\x00\r\n"
	input2 := "\x00\xff\xfe\xef\xbb\xbf\n\x80abc\x00\n"
	if got := convertString(t, true, input2); got != wantBack {
		t.Errorf("unix2dos got %q, want %q", got, wantBack)
	}
}

func TestDos2UnixStdinFilter(t *testing.T) {
	r := runBuiltinStdin(t, "dos2unix", "a\r\nb\r\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\nb\n")
	if r.stderr != "" {
		t.Errorf("stderr = %q", r.stderr)
	}

	r = runBuiltinStdin(t, "unix2dos", "a\nb\n")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "a\r\nb\r\n")
}

func TestDos2UnixStdinFromOneByteReader(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"dos2unix", "a\r\nb\r\n", "a\nb\n"},
		{"unix2dos", "a\nb\n", "a\r\nb\r\n"},
	} {
		var out bytes.Buffer
		code, stderr := runDosUnixCtx(t, tc.name, &oneByteReader{data: []byte(tc.in)}, &out)
		if code != 0 {
			t.Fatalf("%s: exit %d (stderr %q)", tc.name, code, stderr)
		}
		if out.String() != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, out.String(), tc.want)
		}
	}
}

func TestDos2UnixDashOperand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\r\nb\r\n")
	r := runBuiltinStdinIn(t, dir, "dos2unix", "x\r\ny\r\n", "-", "f.txt")
	assertExitCode(t, r, 0)
	assertStdout(t, r, "x\ny\n")
	data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\nb\n" {
		t.Errorf("file = %q, want %q", data, "a\nb\n")
	}
}

func TestDos2UnixInPlacePreservesMode(t *testing.T) {
	for _, mode := range []os.FileMode{0600, 0644, 0755} {
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		if err := os.WriteFile(path, []byte("a\r\nb\r\n"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		r := runBuiltinIn(t, dir, "dos2unix", "f.txt")
		assertExitCode(t, r, 0)
		if r.stdout != "" || r.stderr != "" {
			t.Errorf("mode %o: stdout %q stderr %q", mode, r.stdout, r.stderr)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "a\nb\n" {
			t.Errorf("mode %o: content %q", mode, data)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("mode %o: got %o", mode, info.Mode().Perm())
		}
		if entries := dirEntries(t, dir); len(entries) != 1 || entries[0] != "f.txt" {
			t.Errorf("mode %o: leftover temp files %v", mode, entries)
		}
	}
}

func TestDos2UnixInPlaceEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "empty.txt", "")
	r := runBuiltinIn(t, dir, "unix2dos", "empty.txt")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(filepath.Join(dir, "empty.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("content %q", data)
	}
}

// TestDos2UnixInPlaceSymlink rewrites the link's target, never the link.
func TestDos2UnixInPlaceSymlink(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "real.txt", "a\r\nb\r\n")
	if err := os.Symlink(target, filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	r := runBuiltinIn(t, dir, "dos2unix", "link.txt")
	assertExitCode(t, r, 0)
	li, err := os.Lstat(filepath.Join(dir, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if li.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link.txt is no longer a symlink (mode %v)", li.Mode())
	}
	if got, err := os.Readlink(filepath.Join(dir, "link.txt")); err != nil || got != target {
		t.Errorf("link target = %q, %v; want %q", got, err, target)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\nb\n" {
		t.Errorf("target content = %q", data)
	}
}

func TestDos2UnixMissingFileContinues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "a\r\n")
	writeFile(t, dir, "b.txt", "b\r\n")
	r := runBuiltinIn(t, dir, "dos2unix", "a.txt", "missing.txt", "b.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "missing.txt")
	assertStderrContains(t, r, "No such file or directory")
	for name, want := range map[string]string{"a.txt": "a\n", "b.txt": "b\n"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("%s = %q, want %q", name, data, want)
		}
	}
	if _, err := os.Lstat(filepath.Join(dir, "missing.txt")); !os.IsNotExist(err) {
		t.Errorf("missing.txt was created: %v", err)
	}
	if entries := dirEntries(t, dir); len(entries) != 2 {
		t.Errorf("leftover files: %v", entries)
	}
}

func TestDos2UnixDirectoryOperand(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	before := dirEntries(t, dir)
	r := runBuiltinIn(t, dir, "unix2dos", "sub")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "sub")
	after := dirEntries(t, dir)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("directory changed: %v -> %v", before, after)
	}
}

func TestDos2UnixUnknownOptionsFail(t *testing.T) {
	for _, args := range [][]string{{"-q"}, {"-k"}, {"--keep-date"}, {"-o", "out.txt"}} {
		dir := t.TempDir()
		writeFile(t, dir, "f.txt", "a\r\n")
		for _, name := range []string{"dos2unix", "unix2dos"} {
			r := runBuiltinIn(t, dir, name, append(args, "f.txt")...)
			assertExitCode(t, r, 1)
			assertStderrContains(t, r, "unknown option")
			assertStderrContains(t, r, name+":")
			data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "a\r\n" {
				t.Errorf("%s %v: file was modified: %q", name, args, data)
			}
		}
	}
}

func TestDos2UnixHelpAndVersion(t *testing.T) {
	for _, name := range []string{"dos2unix", "unix2dos"} {
		for _, flag := range []string{"--help", "-h"} {
			r := runBuiltin(t, name, flag)
			assertExitCode(t, r, 0)
			assertStdoutContains(t, r, "Usage: "+name)
		}
		r := runBuiltin(t, name, "--version")
		assertExitCode(t, r, 0)
		assertStdoutContains(t, r, name)
	}
}

func TestDos2UnixStdinReadError(t *testing.T) {
	var out bytes.Buffer
	code, stderr := runDosUnixCtx(t, "dos2unix", &errorReader{data: []byte("a\r\n"), err: errors.New("boom")}, &out)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "dos2unix: -:") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestDos2UnixStdoutWriteError(t *testing.T) {
	code, stderr := runDosUnixCtx(t, "unix2dos", strings.NewReader("a\n"), alwaysFailWriter{})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unix2dos: -:") {
		t.Errorf("stderr = %q", stderr)
	}
}

// bigReader produces size bytes of "\r\n" and rejects oversized read buffers.
type bigReader struct {
	pair []byte
	left int
	max  int
}

func (r *bigReader) Read(p []byte) (int, error) {
	if len(p) > r.max {
		r.max = len(p)
	}
	if r.left == 0 {
		return 0, io.EOF
	}
	n := 0
	for n+2 <= len(p) && r.left > 0 {
		p[n], p[n+1] = '\r', '\n'
		n += 2
		r.left--
	}
	if n == 0 {
		return 0, nil
	}
	return n, nil
}

// TestDos2UnixStreamsLargeInput checks bounded memory: the read chunk stays
// small while the total output is megabytes.
func TestDos2UnixStreamsLargeInput(t *testing.T) {
	const pairs = 1 << 19 // 1 MiB of CRLF
	src := &bigReader{left: pairs, max: 0}
	var out bytes.Buffer
	if err := convertNewlines(&out, src, false); err != nil {
		t.Fatalf("convertNewlines: %v", err)
	}
	if out.Len() != pairs {
		t.Errorf("output length = %d, want %d", out.Len(), pairs)
	}
	if strings.Count(out.String(), "\n") != pairs || strings.Contains(out.String(), "\r") {
		t.Errorf("output is not all LF")
	}
	if src.max > 128<<10 {
		t.Errorf("read buffer grew to %d bytes; conversion must stream", src.max)
	}
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestDos2UnixReadOnlyDirLeavesFileAlone covers a failed temp-file creation:
// the original file keeps its bytes and no temporary file is left behind.
func TestDos2UnixReadOnlyDirLeavesFileAlone(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root ignores directory permissions")
	}
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "a\r\n")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	r := runBuiltinIn(t, dir, "dos2unix", "f.txt")
	assertExitCode(t, r, 1)
	assertStderrContains(t, r, "Permission denied")
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\r\n" {
		t.Errorf("file was modified: %q", data)
	}
	if entries := dirEntries(t, dir); len(entries) != 1 || entries[0] != "f.txt" {
		t.Errorf("leftover temp files: %v", entries)
	}
}

// TestDos2UnixChunkBoundary puts the CR of a CRLF pair exactly on the read
// buffer edge, so the pair is only correct if the CR is carried over.
func TestDos2UnixChunkBoundary(t *testing.T) {
	filler := strings.Repeat("a", newlineChunk-1)
	in := filler + "\r\nb\r\n"
	var out bytes.Buffer
	if err := convertNewlines(&out, strings.NewReader(in), false); err != nil {
		t.Fatalf("convertNewlines: %v", err)
	}
	if want := filler + "\nb\n"; out.String() != want {
		t.Errorf("dos2unix chunk boundary mismatch (len %d, want %d)", out.Len(), len(want))
	}

	in = filler + "\nb\n"
	out.Reset()
	if err := convertNewlines(&out, strings.NewReader(in), true); err != nil {
		t.Fatalf("convertNewlines: %v", err)
	}
	if want := filler + "\r\nb\r\n"; out.String() != want {
		t.Errorf("unix2dos chunk boundary mismatch (len %d, want %d)", out.Len(), len(want))
	}
}
