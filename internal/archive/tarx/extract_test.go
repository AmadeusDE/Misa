// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fixture struct {
	h    tar.Header
	data string
}

func archive(t *testing.T, entries ...fixture) []byte {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for _, e := range entries {
		e.h.Size = int64(len(e.data))
		if err := w.WriteHeader(&e.h); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func reg(n, s string) fixture {
	return fixture{tar.Header{Name: n, Mode: 0640, Typeflag: tar.TypeReg}, s}
}
func TestExtractSafety(t *testing.T) {
	d := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(d, "pre")); err != nil {
		t.Skip(err)
	}
	b := archive(t, reg("../escape", "bad"), reg("pre/bad", "bad"), fixture{tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: outside}, ""}, reg("link/bad", "bad"), fixture{tar.Header{Name: "hard", Typeflag: tar.TypeLink, Linkname: "link"}, ""}, reg("/good", "ok"))
	var failures, warnings int
	err := Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{OnError: func(string, error) { failures++ }, Warn: func(string) { warnings++ }})
	if err != nil || failures != 4 || warnings != 1 {
		t.Fatalf("err %v failures %d warnings %d", err, failures, warnings)
	}
	if _, err := os.Stat(filepath.Join(outside, "bad")); !os.IsNotExist(err) {
		t.Fatal("escaped", err)
	}
	if got, err := os.ReadFile(filepath.Join(d, "good")); err != nil || string(got) != "ok" {
		t.Fatal(string(got), err)
	}
}
func TestExtractOptionsAndMetadata(t *testing.T) {
	d := t.TempDir()
	tm := time.Unix(1000000000, 0)
	b := archive(t, fixture{tar.Header{Name: "top/dir", Typeflag: tar.TypeDir, Mode: 0751, ModTime: tm}, ""}, reg("top/dir/file", "hi"), fixture{tar.Header{Name: "top/dir/hard", Typeflag: tar.TypeLink, Linkname: "top/dir/file"}, ""})
	err := Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{StripComponents: 1, Umask: 0027, NoSameOwner: true})
	if err != nil {
		t.Fatal(err)
	}
	a, e := os.Stat(filepath.Join(d, "dir/file"))
	if e != nil || a.Mode().Perm() != 0640 {
		t.Fatal(a, e)
	}
	c, e := os.Stat(filepath.Join(d, "dir/hard"))
	if e != nil || !os.SameFile(a, c) {
		t.Fatal(e)
	}
	di, e := os.Stat(filepath.Join(d, "dir"))
	if e != nil || di.Mode().Perm() != 0750 || !di.ModTime().Equal(tm) {
		t.Fatal(di, e)
	}
	var failures int
	err = Extract(context.Background(), bytes.NewReader(archive(t, reg("dir/file", "new"))), d, ExtractOptions{KeepOld: true, OnError: func(string, error) { failures++ }})
	got, _ := os.ReadFile(filepath.Join(d, "dir/file"))
	if err != nil || failures != 1 || string(got) != "hi" {
		t.Fatal(err, failures, string(got))
	}
	var out bytes.Buffer
	err = Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{ToStdout: &out, Select: func(h *tar.Header) bool { return h.Typeflag == tar.TypeReg }})
	if err != nil || out.String() != "hi" {
		t.Fatal(err, out.String())
	}
}
func TestExtractTruncatedAndCancel(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "file"), []byte("old"), 0600)
	b := archive(t, reg("file", string(bytes.Repeat([]byte("x"), 2048))))
	err := Extract(context.Background(), bytes.NewReader(b[:600]), d, ExtractOptions{})
	if err == nil {
		t.Fatal("truncated")
	}
	got, _ := os.ReadFile(filepath.Join(d, "file"))
	if string(got) != "old" {
		t.Fatal(string(got))
	}
	entries, _ := os.ReadDir(d)
	if len(entries) != 1 {
		t.Fatal(entries)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(Extract(ctx, bytes.NewReader(b), d, ExtractOptions{}), context.Canceled) {
		t.Fatal("cancel")
	}
}

type cancelReader struct {
	r      io.Reader
	cancel context.CancelFunc
	read   int
}

func (r *cancelReader) Read(p []byte) (int, error) {
	n, e := r.r.Read(p)
	r.read += n
	if r.read > 512 {
		r.cancel()
	}
	return n, e
}
func TestExtractCancellationDuringFile(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "f"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := archive(t, reg("f", string(bytes.Repeat([]byte("a"), 100000))))
	err := Extract(ctx, &cancelReader{r: bytes.NewReader(b), cancel: cancel}, d, ExtractOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(d, "f"))
	if string(got) != "old" {
		t.Fatal("overwritten")
	}
	all, _ := os.ReadDir(d)
	if len(all) != 1 {
		t.Fatal(all)
	}
}
func TestExtractCallbacksAndPermissions(t *testing.T) {
	d := t.TempDir()
	tm := time.Unix(1000000000, 0)
	a := reg("f", "data")
	a.h.Mode = 0777
	a.h.ModTime = tm
	b := archive(t, a, reg("skip", "x"))
	calls := 0
	err := Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{SamePermissions: true, Umask: 077, Touch: true, NoSameOwner: true, Select: func(h *tar.Header) bool { return h.Name != "skip" }, Verbose: func(*tar.Header) error { calls++; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(d, "f"))
	if err != nil || st.Mode().Perm() != 0777 || st.ModTime().Equal(tm) || calls != 1 {
		t.Fatal(st, err, calls)
	}
	sentinel := errors.New("verbose")
	err = Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{Verbose: func(*tar.Header) error { return sentinel }})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	err = Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{ToStdout: badWriter{}})
	if err == nil {
		t.Fatal("stdout write")
	}
}
func TestExtractHardlinkSelfAndUnsafeTarget(t *testing.T) {
	d := t.TempDir()
	b := archive(t, reg("f", "ok"), fixture{tar.Header{Name: "f", Typeflag: tar.TypeLink, Linkname: "f"}, ""}, fixture{tar.Header{Name: "bad", Typeflag: tar.TypeLink, Linkname: "../f"}, ""})
	failures := 0
	if err := Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{OnError: func(string, error) { failures++ }}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(d, "f"))
	if err != nil || string(got) != "ok" || failures != 1 {
		t.Fatal(string(got), err, failures)
	}
}

func TestDeferredDirectoryMetadataDoesNotFollowReplacement(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	x := extractor{root: root, opt: ExtractOptions{NoSameOwner: true, SamePermissions: true}}
	h := &tar.Header{Name: "d", Typeflag: tar.TypeDir, Mode: 0700, ModTime: time.Unix(1000000000, 0)}
	if _, err := x.entry(context.Background(), tar.NewReader(bytes.NewReader(nil)), h, "d"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "d"), filepath.Join(dir, "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "d"), 0755); err != nil {
		t.Fatal(err)
	}
	x.finishDirs(context.Background())
	st, err := os.Stat(filepath.Join(dir, "d"))
	if err != nil || st.Mode().Perm() != 0755 || st.ModTime().Equal(h.ModTime) {
		t.Fatalf("replacement changed: %v %v", st, err)
	}
}

func TestDeferredDirectoryMetadataCancellation(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	x := extractor{root: root, opt: ExtractOptions{NoSameOwner: true, SamePermissions: true}}
	h := &tar.Header{Name: "d", Typeflag: tar.TypeDir, Mode: 0755}
	if _, err := x.entry(context.Background(), tar.NewReader(bytes.NewReader(nil)), h, "d"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	x.finishDirs(ctx)
	st, err := os.Stat(filepath.Join(dir, "d"))
	if err != nil || st.Mode().Perm() != 0700 {
		t.Fatalf("metadata applied after cancellation: %v %v", st, err)
	}
}
