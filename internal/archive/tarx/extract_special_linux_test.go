//go:build linux

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFIFO(t *testing.T) {
	d := t.TempDir()
	b := archive(t, fixture{tar.Header{Name: "fifo", Typeflag: tar.TypeFifo, Mode: 0640}, ""})
	var failures []error
	err := Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{NoSameOwner: true, OnError: func(_ string, e error) { failures = append(failures, e) }})
	if err != nil {
		t.Fatal(err)
	}
	info, e := os.Lstat(filepath.Join(d, "fifo"))
	if os.Geteuid() != 0 {
		if !os.IsNotExist(e) || len(failures) != 1 {
			t.Fatal(info, e, failures)
		}
		return
	}
	if e != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatal(info, e, failures)
	}
	if len(failures) == 0 && info.Mode().Perm() != 0640 {
		t.Fatal(info.Mode())
	}
}

func TestDeferredDirectoryDescriptorBound(t *testing.T) {
	fdCount := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Skip(err)
		}
		return len(entries)
	}
	d := t.TempDir()
	root, err := os.OpenRoot(d)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	x := extractor{root: root, opt: ExtractOptions{NoSameOwner: true}}
	before := fdCount()
	for i := 0; i < 80; i++ {
		n := fmt.Sprintf("d%d", i)
		h := &tar.Header{Name: n, Typeflag: tar.TypeDir, Mode: 0755}
		if _, err := x.entry(context.Background(), tar.NewReader(bytes.NewReader(nil)), h, n); err != nil {
			t.Fatal(err)
		}
	}
	after := fdCount()
	x.finishDirs(context.Background())
	if after-before > 8 {
		t.Fatalf("deferred 80 directories retained %d descriptors", after-before)
	}
}
