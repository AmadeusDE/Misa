// Tests informed by Toybox tar.test, copyright Toybox contributors, 0BSD.
package tarx

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateRead(t *testing.T) {
	d := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(filepath.Join(d, "z"), 0751))
	must(os.WriteFile(filepath.Join(d, "z", "file"), []byte("data"), 0640))
	must(os.Mkdir(filepath.Join(d, "a"), 0755))
	must(os.Link(filepath.Join(d, "z", "file"), filepath.Join(d, "a", "hard")))
	must(os.Symlink("z/file", filepath.Join(d, "link")))
	long := strings.Repeat("x", 150)
	must(os.Mkdir(filepath.Join(d, long), 0755))
	must(os.WriteFile(filepath.Join(d, long, strings.Repeat("y", 150)), nil, 0600))
	var b bytes.Buffer
	var failures []string
	must(Create(context.Background(), &b, []Source{{d, "missing"}, {d, "."}}, CreateOptions{OnError: func(n string, _ error) { failures = append(failures, n) }}))
	if len(failures) != 1 {
		t.Fatalf("failures: %v", failures)
	}
	var names []string
	headers := map[string]*tar.Header{}
	contents := map[string]string{}
	must(Read(&b, func(h *tar.Header, r io.Reader) error {
		c, e := io.ReadAll(r)
		names = append(names, h.Name)
		copy := *h
		headers[h.Name] = &copy
		contents[h.Name] = string(c)
		return e
	}))
	if headers["a/hard"] == nil || contents["a/hard"] != "data" || headers["z/file"].Typeflag != tar.TypeLink || headers["z/file"].Linkname != "a/hard" {
		t.Fatalf("hard links: %#v", headers)
	}
	if headers["link"].Typeflag != tar.TypeSymlink || headers["link"].Linkname != "z/file" || headers["a/hard"].Mode != 0640 {
		t.Fatal("metadata")
	}
	if headers[long+"/"+strings.Repeat("y", 150)] == nil {
		t.Fatal("long name missing")
	}
	if strings.Join(names[:3], ",") != "./,a/,a/hard" {
		t.Fatalf("order: %v", names)
	}
}

func TestDereferenceExcludeSelfAndNames(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "file"), []byte("x"), 0600)
	if err := os.Symlink("file", filepath.Join(d, "link")); err != nil {
		t.Skip(err)
	}
	out, err := os.Create(filepath.Join(d, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	info, _ := out.Stat()
	err = Create(context.Background(), out, []Source{{d, "."}}, CreateOptions{Dereference: true, ExcludeFile: info, Exclude: []string{"file"}})
	if err != nil {
		t.Fatal(err)
	}
	out.Seek(0, 0)
	var names []string
	err = Read(out, func(h *tar.Header, r io.Reader) error {
		names = append(names, h.Name)
		if h.Name == "link" && h.Typeflag != tar.TypeReg {
			t.Fatal("not dereferenced")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "./,link" {
		t.Fatal(names)
	}
}
func TestErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(Create(ctx, io.Discard, []Source{{t.TempDir(), "."}}, CreateOptions{}), context.Canceled) {
		t.Fatal("cancellation")
	}
	if Create(context.Background(), badWriter{}, nil, CreateOptions{}) == nil {
		t.Fatal("write error")
	}
	if Read(bytes.NewReader([]byte("bad")), func(*tar.Header, io.Reader) error { return nil }) == nil {
		t.Fatal("read error")
	}
}

type badWriter struct{}

func (badWriter) Write([]byte) (int, error) { return 0, errors.New("write") }

func TestFormatsAndSafety(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "f"), []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	// Resolve the parent path on disk, but store only a relative member name.
	if err := os.Mkdir(filepath.Join(d, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	if err := Create(context.Background(), &b, []Source{{filepath.Join(d, "sub"), "../f"}}, CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(&b)
	h, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != "f" || h.Format != tar.FormatUSTAR {
		t.Fatalf("name/format: %q %v", h.Name, h.Format)
	}
	for _, format := range []tar.Format{tar.FormatPAX, tar.FormatGNU} {
		b.Reset()
		tw := tar.NewWriter(&b)
		long := strings.Repeat("x", 300)
		if err := tw.WriteHeader(&tar.Header{Name: long, Mode: 0600, Format: format}); err != nil {
			t.Fatal(err)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := Read(&b, func(h *tar.Header, _ io.Reader) error {
			if h.Name != long {
				t.Fatalf("%v name %q", format, h.Name)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExcluded(t *testing.T) {
	for _, tc := range []struct {
		name, pattern string
		want          bool
	}{{"a/b/c", "b", true}, {"a/b/c", "a/*", true}, {"a/b", "*.txt", false}, {"a/x.txt", "*.txt", true}} {
		if got := Excluded(tc.name, []string{tc.pattern}); got != tc.want {
			t.Errorf("Excluded(%q,%q)=%v", tc.name, tc.pattern, got)
		}
	}
}

func TestLongLinkAndCallbacks(t *testing.T) {
	d := t.TempDir()
	target := strings.Repeat("t", 150)
	if err := os.Symlink(target, filepath.Join(d, "link")); err != nil {
		t.Skip(err)
	}
	var b bytes.Buffer
	count := 0
	err := Create(context.Background(), &b, []Source{{d, "link"}}, CreateOptions{Verbose: func(*tar.Header) error { count++; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	err = Read(&b, func(h *tar.Header, _ io.Reader) error {
		if h.Linkname != target || h.Format != tar.FormatPAX {
			t.Fatalf("link %q format %v", h.Linkname, h.Format)
		}
		return nil
	})
	if err != nil || count != 1 {
		t.Fatalf("read %v count %d", err, count)
	}
	sentinel := errors.New("stop")
	b.Reset()
	err = Create(context.Background(), &b, []Source{{d, "link"}}, CreateOptions{Verbose: func(*tar.Header) error { return sentinel }})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

func TestDuplicateSourceAndBrokenLink(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "f"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(d, "f"), filepath.Join(d, "other")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink("nonexistent", filepath.Join(d, "broken")); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	failures := 0
	if err := Create(context.Background(), &b, []Source{{d, "broken"}, {d, "f"}, {d, "f"}}, CreateOptions{Dereference: true, OnError: func(string, error) { failures++ }}); err != nil {
		t.Fatal(err)
	}
	count := 0
	if err := Read(&b, func(h *tar.Header, r io.Reader) error {
		count++
		data, err := io.ReadAll(r)
		if h.Typeflag != tar.TypeReg || string(data) != "x" {
			t.Fatalf("duplicate: %#v %q", h, data)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if failures != 1 || count != 2 {
		t.Fatalf("failures %d count %d", failures, count)
	}
}

func TestReplacedHardlinkTarget(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(a, "file"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(a, "file"), filepath.Join(a, "other")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(b, "file"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := Create(context.Background(), &archive, []Source{{a, "file"}, {b, "file"}, {a, "other"}}, CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	n := 0
	if err := Read(&archive, func(h *tar.Header, r io.Reader) error {
		n++
		data, err := io.ReadAll(r)
		if n == 3 && (h.Typeflag != tar.TypeReg || string(data) != "old") {
			t.Fatalf("stale hardlink: %#v data %q", h, data)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal(n)
	}
}
