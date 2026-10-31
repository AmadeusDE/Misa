// Tar option and stream tests; behavior informed by Toybox tar.c and tar.test (0BSD).
package builtins

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTarEmptyFilesFromAndMissingSelection(t *testing.T) {
	dir := t.TempDir()
	code, data, e := tarRun(dir, strings.NewReader(""), "-c", "-T", "-")
	if code != 0 || len(data) == 0 {
		t.Fatalf("empty -T: %d %s", code, e)
	}
	code, _, e = tarRun(dir, bytes.NewReader(data), "-t", "missing")
	if code != 2 || !strings.Contains(e, "missing") {
		t.Fatalf("missing selection: %d %s", code, e)
	}
	code, _, _ = tarRun(dir, nil, "-c", "-C", "missing", filepath.Join(dir, "absolute"))
	if code != 2 {
		t.Fatal("invalid -C accepted")
	}
}
func TestTarRepeatedFilesFromAndOwner(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b"} {
		os.WriteFile(filepath.Join(dir, name), []byte(name), 0644)
		os.WriteFile(filepath.Join(dir, name+".list"), []byte(name+"\n"), 0644)
	}
	code, data, e := tarRun(dir, nil, "-c", "--numeric-owner", "--owner=123", "--group=456", "-T", "a.list", "--files-from=b.list")
	if code != 0 {
		t.Fatalf("%d %s", code, e)
	}
	r := tar.NewReader(bytes.NewReader(data))
	for _, name := range []string{"a", "b"} {
		h, err := r.Next()
		if err != nil || h.Name != name || h.Uid != 123 || h.Gid != 456 || h.Uname != "" || h.Gname != "" {
			t.Fatalf("%v %v", h, err)
		}
	}
}

type tarCloseBuffer struct {
	bytes.Buffer
	closed bool
}

func (b *tarCloseBuffer) Close() error { b.closed = true; return nil }
func TestTarConcurrentCwdAndStreams(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x"), []byte("x"), 0644)
	errs := make(chan string, 10)
	for range 10 {
		go func() {
			out := &tarCloseBuffer{}
			ctx := &Context{Dir: dir, Args: []string{"-c", "x"}, Stdout: out, Stderr: io.Discard}
			if runTar(ctx) != 0 || out.closed {
				errs <- "create or stream close"
				return
			}
			in := &tarCloseBuffer{}
			in.Write(out.Bytes())
			ctx = &Context{Dir: dir, Args: []string{"-t"}, Stdin: in, Stdout: io.Discard, Stderr: io.Discard}
			if runTar(ctx) != 0 || in.closed {
				errs <- "list or stream close"
				return
			}
			errs <- ""
		}()
	}
	for range 10 {
		if e := <-errs; e != "" {
			t.Error(e)
		}
	}
	now, _ := os.Getwd()
	if now != old {
		t.Fatalf("cwd changed %s -> %s", old, now)
	}
}

func TestTarRedirectedArchiveExcludesItself(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file"), []byte("x"), 0644)
	f, err := os.Create(filepath.Join(dir, "archive"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := &Context{Dir: dir, Args: []string{"-c", "."}, Stdout: f, Stderr: io.Discard}
	code := runTar(ctx)
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatal(code)
	}
	f, err = os.Open(filepath.Join(dir, "archive"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := tar.NewReader(f)
	for {
		h, e := r.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if strings.TrimPrefix(h.Name, "./") == "archive" {
			t.Fatal("archive contains itself")
		}
	}
	code, _, e := tarRun(dir, strings.NewReader("anything"), "-t", "-T", "-")
	if code != 2 || !strings.Contains(e, "stdin") {
		t.Fatalf("stdin collision: %d %s", code, e)
	}
}

func TestTarHelpAndOptions(t *testing.T) {
	dir := t.TempDir()
	code, out, _ := tarRun(dir, nil, "--help")
	if code != 0 || !strings.Contains(string(out), "Usage: tar") || !strings.Contains(string(out), "U33") || strings.Contains(string(out), "-h, --help") {
		t.Fatalf("help %d %q", code, out)
	}
	code, out, _ = tarRun(dir, nil, "--version")
	if code != 0 || !strings.Contains(string(out), "tar (Misa)") {
		t.Fatalf("version %d %q", code, out)
	}
	for _, args := range [][]string{{"-c", "-f"}, {"-c", "--owner"}, {"-c", "--bogus"}} {
		code, _, _ = tarRun(dir, nil, args...)
		if code != 2 {
			t.Errorf("%v: %d", args, code)
		}
	}
	os.WriteFile(filepath.Join(dir, "-file"), []byte("x"), 0644)
	code, data, e := tarRun(dir, nil, "-c", "--", "-file")
	if code != 0 {
		t.Fatalf("--: %d %s", code, e)
	}
	code, out, e = tarRun(dir, bytes.NewReader(data), "-tO")
	if code != 0 || string(out) != "-file\n" {
		t.Fatalf("-tO: %d %q %s", code, out, e)
	}
}
func TestTarListOwnersAndLinks(t *testing.T) {
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for _, h := range []*tar.Header{{Name: "sym", Typeflag: tar.TypeSymlink, Linkname: "target", Uid: 1, Gid: 2, Uname: "alice", Gname: "staff"}, {Name: "hard", Typeflag: tar.TypeLink, Linkname: "target", Uid: 1, Gid: 2, Uname: "alice", Gname: "staff"}} {
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	data := b.Bytes()
	for _, tc := range []struct {
		args []string
		want []string
	}{{[]string{"-t"}, []string{"sym -> target", "hard link to target"}}, {[]string{"-tv"}, []string{"alice/staff"}}, {[]string{"-tv", "--numeric-owner"}, []string{"1/2"}}, {[]string{"-tv", "--owner=12", "--group=34"}, []string{"12/34"}}} {
		code, out, e := tarRun(t.TempDir(), bytes.NewReader(data), tc.args...)
		if code != 0 {
			t.Fatalf("%v %d %s", tc.args, code, e)
		}
		for _, want := range tc.want {
			if !strings.Contains(string(out), want) {
				t.Errorf("%v: %q lacks %q", tc.args, out, want)
			}
		}
	}
}
