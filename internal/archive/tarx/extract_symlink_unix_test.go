//go:build unix

// Extraction behavior is informed by Toybox tar.c (0BSD); see NOTICE.
package tarx

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractSymlinkTimeNoFollow(t *testing.T) {
	d := t.TempDir()
	target := filepath.Join(d, "target")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	before := time.Unix(1100000000, 0)
	if err := os.Chtimes(target, before, before); err != nil {
		t.Fatal(err)
	}
	mt := time.Unix(1200000000, 0)
	b := archive(t, fixture{tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "target", ModTime: mt}, ""})
	var errs []error
	if err := Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{NoSameOwner: true, OnError: func(_ string, e error) { errs = append(errs, e) }}); err != nil {
		t.Fatal(err)
	}
	link, err := os.Lstat(filepath.Join(d, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 || !link.ModTime().Equal(mt) {
		t.Fatalf("link time %v errors %v", link.ModTime(), errs)
	}
	st, err := os.Stat(target)
	if err != nil || !st.ModTime().Equal(before) {
		t.Fatalf("target time %v err %v", st, err)
	}
	b = archive(t, fixture{tar.Header{Name: "link2", Typeflag: tar.TypeSymlink, Linkname: "target", ModTime: mt}, ""})
	if err := Extract(context.Background(), bytes.NewReader(b), d, ExtractOptions{Touch: true, NoSameOwner: true}); err != nil {
		t.Fatal(err)
	}
	link, err = os.Lstat(filepath.Join(d, "link2"))
	if err != nil || link.ModTime().Equal(mt) {
		t.Fatalf("touch: %v %v", link, err)
	}
}
