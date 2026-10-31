// Tar command tests; behavior informed by Toybox tar.c and tar.test (0BSD).
package builtins

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tarRun(dir string, in io.Reader, args ...string) (int, []byte, string) {
	var out, errout bytes.Buffer
	code := runTar(&Context{Dir: dir, Args: args, Stdin: in, Stdout: &out, Stderr: &errout})
	return code, out.Bytes(), errout.String()
}
func TestTarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "file"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	code, data, stderr := tarRun(dir, nil, "-cv", "sub")
	if code != 0 {
		t.Fatalf("create: %d %s", code, stderr)
	}
	if !strings.Contains(stderr, "sub/file") {
		t.Fatalf("verbose: %q", stderr)
	}
	r := tar.NewReader(bytes.NewReader(data))
	found := false
	for {
		h, e := r.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if h.Name == "sub/file" {
			b, e := io.ReadAll(r)
			if e != nil || string(b) != "hello" {
				t.Fatalf("body %q %v", b, e)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing member")
	}
	code, out, stderr := tarRun(dir, bytes.NewReader(data), "-t", "sub/file")
	if code != 0 || string(out) != "sub/file\n" {
		t.Fatalf("list: %d %q %s", code, out, stderr)
	}
}
func TestTarOrderedDirectoriesAndFilesFrom(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"a", "b"} {
		os.Mkdir(filepath.Join(dir, d), 0755)
		os.WriteFile(filepath.Join(dir, d, "file"), []byte(d), 0644)
	}
	code, data, stderr := tarRun(dir, strings.NewReader("file\x00"), "-c", "-C", "a", "-T", "-", "--null", "-C", "../b", "file")
	if code != 0 {
		t.Fatalf("%d %s", code, stderr)
	}
	r := tar.NewReader(bytes.NewReader(data))
	for _, want := range []string{"a", "b"} {
		h, e := r.Next()
		if e != nil || h.Name != "file" {
			t.Fatalf("header %v %v", h, e)
		}
		b, _ := io.ReadAll(r)
		if string(b) != want {
			t.Fatalf("got %q want %q", b, want)
		}
	}
}
func TestTarFailures(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ok"), []byte("ok"), 0644)
	for _, args := range [][]string{{"-c"}, {"-c", "--owner=alice", "ok"}, {"-ct", "ok"}, {"-cO", "ok"}} {
		code, _, _ := tarRun(dir, nil, args...)
		if code != 1 {
			t.Errorf("%v: %d", args, code)
		}
	}
	code, data, _ := tarRun(dir, nil, "-c", "missing", "ok")
	if code != 2 || len(data) == 0 {
		t.Fatalf("partial: %d %d", code, len(data))
	}
	code, _, _ = tarRun(dir, strings.NewReader("broken"), "-t")
	if code != 2 {
		t.Fatalf("bad archive: %d", code)
	}
	ctx := &Context{Dir: dir, Args: []string{"-c", "ok"}, Stdout: failTarWriter{}, Stderr: io.Discard}
	if runTar(ctx) != 2 {
		t.Fatal("write failure ignored")
	}
}

type failTarWriter struct{}

func (failTarWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestTarExcludeLongNamesAndLinks(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("a", 110)
	if err := os.WriteFile(filepath.Join(dir, name), []byte("long"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(name, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	code, data, errout := tarRun(dir, nil, "-c", "--exclude=link", name, "link")
	if code != 0 {
		t.Fatalf("%d %s", code, errout)
	}
	r := tar.NewReader(bytes.NewReader(data))
	h, e := r.Next()
	if e != nil || h.Name != name || h.Mode != 0640 {
		t.Fatalf("header %v %v", h, e)
	}
	if _, e = r.Next(); e != io.EOF {
		t.Fatalf("excluded: %v", e)
	}
	code, data, errout = tarRun(dir, nil, "-ch", "link")
	if code != 0 {
		t.Fatalf("%d %s", code, errout)
	}
	h, e = tar.NewReader(bytes.NewReader(data)).Next()
	if e != nil || h.Typeflag != tar.TypeReg {
		t.Fatalf("dereference: %v %v", h, e)
	}
	code, _, _ = tarRun(dir, bytes.NewReader(data), "-t", "--exclude=link")
	if code != 0 {
		t.Fatal(code)
	}
}
func TestTarArchiveFileAndNoTruncate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(dir, "archive"), []byte("keep"), 0644)
	code, _, _ := tarRun(dir, nil, "-cf", "archive")
	b, _ := os.ReadFile(filepath.Join(dir, "archive"))
	if code != 1 || string(b) != "keep" {
		t.Fatalf("%d %q", code, b)
	}
	code, out, e := tarRun(dir, nil, "cfv", "archive", "file")
	if code != 0 || string(out) != "file\n" {
		t.Fatalf("%d %q %s", code, out, e)
	}
	code, out, e = tarRun(dir, nil, "-tf", "archive")
	if code != 0 || string(out) != "file\n" {
		t.Fatalf("%d %q %s", code, out, e)
	}
}
func TestTarListWriteErrorAndDeferred(t *testing.T) {
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	w.WriteHeader(&tar.Header{Name: "file", Mode: 0644})
	w.Close()
	ctx := &Context{Dir: t.TempDir(), Args: []string{"-t"}, Stdin: &b, Stdout: failTarWriter{}, Stderr: io.Discard}
	if runTar(ctx) != 2 {
		t.Fatal("list write error ignored")
	}
	for _, arg := range []string{"-j", "-J", "-a", "--auto-compress"} {
		code, _, errout := tarRun(ctx.Dir, nil, "-c", arg, "file")
		if code != 1 || !strings.Contains(errout, "not supported") {
			t.Errorf("%s: %d %s", arg, code, errout)
		}
	}
}

type failTarReader struct{}

func (failTarReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func TestTarInputErrors(t *testing.T) {
	dir := t.TempDir()
	for _, input := range []io.Reader{failTarReader{}, strings.NewReader("short")} {
		code, _, _ := tarRun(dir, input, "-t")
		if code != 2 {
			t.Fatalf("input error: %d", code)
		}
	}
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	w.WriteHeader(&tar.Header{Name: "file", Mode: 0644, Size: 10})
	w.Write([]byte("0123456789"))
	w.Close()
	data := b.Bytes()[:515]
	code, _, _ := tarRun(dir, bytes.NewReader(data), "-t")
	if code != 2 {
		t.Fatalf("truncated input: %d", code)
	}
	os.WriteFile(filepath.Join(dir, "ok"), []byte("x"), 0644)
	code, _, _ = tarRun(dir, failTarReader{}, "-c", "-T", "-", "ok")
	if code != 2 {
		t.Fatalf("files-from read error: %d", code)
	}
}
