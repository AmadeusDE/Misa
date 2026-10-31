package lscore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingMetadataRendersQuestionMarks(t *testing.T) {
	fi := &FileInfo{Name: "f", Size: 3, Nlink: 9, Inode: 7, Blocks: 8, Owner: "o", Group: "g",
		Missing: FieldOwner | FieldGroup | FieldNlink | FieldInode | FieldBlocks}
	opts := FormatOpts{Long: true, Inode: true, ShowBlocks: true}
	for _, row := range [][]string{fileRow(fi, opts), fileRowPlain(fi, opts)} {
		got := strings.Join(row[:6], " ")
		if want := "? ? " + FormatModeString(fi.Mode) + " ? ? ?"; got != want {
			t.Errorf("row = %q, want %q", got, want)
		}
	}
	opts.NumericUID = true
	if row := fileRow(fi, opts); row[4] != "?" || row[5] != "?" {
		t.Errorf("numeric ids = %q %q, want ? ?", row[4], row[5])
	}
	if got := entryPrefix(fi, FormatOpts{Inode: true, ShowBlocks: true}); got != "? ? " {
		t.Errorf("entryPrefix = %q, want %q", got, "? ? ")
	}
}

func TestPresentMetadataRendersValues(t *testing.T) {
	fi := &FileInfo{Name: "f", Nlink: 2, Inode: 7, Owner: "o", Group: "g"}
	row := fileRow(fi, FormatOpts{Long: true, Inode: true})
	if row[0] != "7" || row[2] != "2" || row[3] != "o" || row[4] != "g" {
		t.Errorf("row = %q", row)
	}
}

func TestStatFileReportsAvailableFields(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi := statFile(f, "f", false)
	if fi.Err != nil {
		t.Fatal(fi.Err)
	}
	if fi.lacks(FieldNlink) || fi.lacks(FieldInode) || fi.Nlink == 0 || fi.Inode == 0 {
		t.Errorf("link count and inode must be available: %+v", fi)
	}
	if fi.lacks(FieldAccessTime) || fi.AccessTime.IsZero() {
		t.Errorf("access time must be available: %+v", fi)
	}
}

func TestStatFileMissingPathKeepsError(t *testing.T) {
	fi := statFile(filepath.Join(t.TempDir(), "gone"), "gone", false)
	if fi.Err == nil {
		t.Fatal("want an error for a missing path")
	}
}

func TestIdentifyHardLinkSharesIdentity(t *testing.T) {
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	for _, p := range []string{a, c} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Link(a, b); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	ida, ok1 := Identify(a)
	idb, ok2 := Identify(b)
	idc, ok3 := Identify(c)
	if !ok1 || !ok2 || !ok3 {
		t.Fatal("Identify failed on existing files")
	}
	if ida != idb {
		t.Errorf("hard links differ: %+v vs %+v", ida, idb)
	}
	if ida == idc {
		t.Errorf("distinct files share identity %+v", ida)
	}
	if fi := statFile(a, "a", false); fi.Nlink != 2 {
		t.Errorf("Nlink = %d, want 2", fi.Nlink)
	}
}

func TestIdentifyMissingPath(t *testing.T) {
	if _, ok := Identify(filepath.Join(t.TempDir(), "gone")); ok {
		t.Error("Identify of a missing path must report false")
	}
}
