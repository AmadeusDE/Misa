// Native equivalents of Toybox's extraction fixtures; no host archive tools.
package toybox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func toyboxExtractFixture(t *testing.T, compressed bool) []byte {
	t.Helper()
	var data bytes.Buffer
	var target = &data
	var gz *gzip.Writer
	if compressed {
		gz = gzip.NewWriter(&data)
	}
	var w *tar.Writer
	if gz != nil {
		w = tar.NewWriter(gz)
	} else {
		w = tar.NewWriter(target)
	}
	stamp := time.Unix(1494637555, 0)
	for _, h := range []*tar.Header{
		{Name: "dir/", Typeflag: tar.TypeDir, Mode: 0750, ModTime: stamp},
		{Name: "dir/file", Mode: 0640, Size: 12, ModTime: stamp},
	} {
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size != 0 {
			if _, err := w.Write([]byte("hello world\n")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if gz != nil {
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return data.Bytes()
}

func toyboxExtractDir(t *testing.T, compressed, stdin bool) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dd"), 0755); err != nil {
		t.Fatal(err)
	}
	data := toyboxExtractFixture(t, compressed)
	var got []byte
	if stdin {
		got = tarRun(t, dir, data, "xvC", "dd")
	} else {
		if err := os.WriteFile(filepath.Join(dir, "fixture.tar"), data, 0600); err != nil {
			t.Fatal(err)
		}
		got = tarRun(t, dir, nil, "xvCf", "dd", "fixture.tar")
	}
	if string(got) != "dir/\ndir/file\n" {
		t.Fatalf("verbose output: %q", got)
	}
	for _, item := range []struct {
		name string
		mode os.FileMode
	}{{"dir", 0750}, {"dir/file", 0640}} {
		info, err := os.Stat(filepath.Join(dir, "dd", item.name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != item.mode || info.ModTime().Unix() != 1494637555 {
			t.Errorf("%s: mode %o mtime %d", item.name, info.Mode().Perm(), info.ModTime().Unix())
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, "dd/dir/file"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello world\n" {
		t.Fatalf("content %q", content)
	}
}
