// Tar integration tests informed by Toybox tar.c (0BSD).
package builtins

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTarExtractCompression(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.Mkdir(filepath.Join(src, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "file"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-c", "-cz"} {
		code, data, e := tarRun(src, nil, flag, "sub")
		if code != 0 {
			t.Fatalf("create %s: %d %s", flag, code, e)
		}
		code, out, e := tarRun(dst, bytes.NewReader(data), "-t")
		if code != 0 || !strings.Contains(string(out), "sub/file") {
			t.Fatalf("list: %d %q %s", code, out, e)
		}
		code, _, e = tarRun(dst, bytes.NewReader(data), "-x", "--strip-components=1")
		if code != 0 {
			t.Fatalf("extract: %d %s", code, e)
		}
		got, err := os.ReadFile(filepath.Join(dst, "file"))
		if err != nil || string(got) != "payload" {
			t.Fatalf("file %q %v", got, err)
		}
		os.WriteFile(filepath.Join(dst, "file"), []byte("old"), 0644)
		code, _, _ = tarRun(dst, bytes.NewReader(data), "-xk", "--strip-components=1")
		if code == 0 {
			t.Fatal("keep existing succeeded")
		}
		got, _ = os.ReadFile(filepath.Join(dst, "file"))
		if string(got) != "old" {
			t.Fatalf("keep: %q", got)
		}
		code, out, e = tarRun(dst, bytes.NewReader(data), "-xOv")
		if code != 0 || string(out) != "payload" || !strings.Contains(e, "sub/file") {
			t.Fatalf("stdout: %d %q %s", code, out, e)
		}
	}
}

func TestTarDamagedGzipNoExtraction(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file"), []byte("data"), 0644)
	code, data, e := tarRun(src, nil, "-cz", "file")
	if code != 0 {
		t.Fatalf("%d %s", code, e)
	}
	for _, cut := range []bool{false, true} {
		dst := t.TempDir()
		bad := append([]byte(nil), data...)
		if cut {
			bad = bad[:len(bad)-3]
		} else {
			bad[len(bad)-8] ^= 1
		}
		code, _, _ := tarRun(dst, bytes.NewReader(bad), "-x")
		if code != 2 {
			t.Fatalf("damaged: %d", code)
		}
		if _, err := os.Stat(filepath.Join(dst, "file")); !os.IsNotExist(err) {
			t.Fatalf("file extracted: %v", err)
		}
	}
	_, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
}

func TestTarCancelled(t *testing.T) {
	c, cancel := context.WithCancel(context.Background())
	cancel()
	ctx := &Context{Dir: t.TempDir(), Args: []string{"-t"}, Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard, Ctx: c}
	if code := runTar(ctx); code != 130 {
		t.Fatalf("cancel status %d", code)
	}
}

type tarOwnedReader struct {
	io.Reader
	closed bool
}

func (r *tarOwnedReader) Close() error { r.closed = true; return nil }

type tarOwnedWriter struct {
	bytes.Buffer
	closed bool
}

func (w *tarOwnedWriter) Close() error { w.closed = true; return nil }

func TestTarCallerStreamsAndFinalDirectory(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file"), []byte("data"), 0644)
	output := new(tarOwnedWriter)
	ctx := &Context{Dir: src, Args: []string{"-cz", "file"}, Stdout: output, Stderr: io.Discard}
	if n := runTar(ctx); n != 0 || output.closed {
		t.Fatalf("create %d closed %v", n, output.closed)
	}
	dst := t.TempDir()
	os.Mkdir(filepath.Join(dst, "a"), 0755)
	os.Mkdir(filepath.Join(dst, "b"), 0755)
	input := &tarOwnedReader{Reader: bytes.NewReader(output.Bytes())}
	ctx = &Context{Dir: dst, Args: []string{"-x", "-C", "a", "file", "-C", "../b"}, Stdin: input, Stdout: io.Discard, Stderr: io.Discard}
	if n := runTar(ctx); n != 0 || input.closed {
		t.Fatalf("extract %d closed %v", n, input.closed)
	}
	if b, e := os.ReadFile(filepath.Join(dst, "b", "file")); e != nil || string(b) != "data" {
		t.Fatalf("final -C: %q %v", b, e)
	}
	if _, e := os.Stat(filepath.Join(dst, "a", "file")); !os.IsNotExist(e) {
		t.Fatalf("earlier -C: %v", e)
	}
}

type tarEndReader struct{ data []byte }

func (r *tarEndReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.ErrClosedPipe
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}
func TestTarListEndAndGzipCRC(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file"), []byte("data"), 0644)
	code, plain, e := tarRun(src, nil, "-c", "file")
	if code != 0 {
		t.Fatalf("%d %s", code, e)
	}
	code, _, e = tarRun(src, &tarEndReader{data: plain}, "-t")
	if code != 0 {
		t.Fatalf("plain list read beyond footer: %d %s", code, e)
	}
	code, gz, e := tarRun(src, nil, "-cz", "file")
	if code != 0 {
		t.Fatalf("%d %s", code, e)
	}
	gz[len(gz)-8] ^= 1
	code, _, _ = tarRun(src, bytes.NewReader(gz), "-t")
	if code != 2 {
		t.Fatalf("bad gzip list status %d", code)
	}
}
