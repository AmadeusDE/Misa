package lscore

import (
	"strings"
	"testing"
)

func TestEntryNameShowsArrowOnlyInLongFormat(t *testing.T) {
	fi := &FileInfo{Name: "link", IsSymlink: true, LinkTarget: "target"}

	plain := entryNamePlain(fi, FormatOpts{})
	if strings.Contains(plain, "->") {
		t.Errorf("non-long entry name should not show link target, got %q", plain)
	}

	long := entryNamePlain(fi, FormatOpts{Long: true})
	if !strings.Contains(long, "-> target") {
		t.Errorf("long entry name should show link target, got %q", long)
	}
}

func TestIndicatorStyles(t *testing.T) {
	dir := &FileInfo{Name: "d", IsDir: true}
	exe := &FileInfo{Name: "e", Mode: 0111}
	link := &FileInfo{Name: "l", IsSymlink: true}

	cases := []struct {
		style string
		fi    *FileInfo
		want  string
	}{
		{"", dir, ""},
		{"slash", dir, "/"},
		{"slash", exe, ""},
		{"classify", exe, "*"},
		{"classify", dir, "/"},
		{"classify", link, "@"},
		{"file-type", exe, ""}, // file-type never marks executables
		{"file-type", link, "@"},
	}
	for _, c := range cases {
		opts := FormatOpts{IndicatorStyle: c.style}
		if got := entryNamePlain(c.fi, opts); !strings.HasSuffix(got, c.fi.Name+c.want) {
			t.Errorf("indicator style %q on %+v: got %q, want suffix %q", c.style, c.fi, got, c.want)
		}
	}
}

func TestFormatPrintsTotalForEmptyDirectoryWhenRelevant(t *testing.T) {
	var buf strings.Builder
	Format(&buf, nil, FormatOpts{Long: true, ShowTotal: true}, 80)
	if buf.String() != "total 0\n" {
		t.Errorf("empty long listing = %q, want %q", buf.String(), "total 0\n")
	}
}

func TestFormatSkipsTotalWithoutLongOrBlocks(t *testing.T) {
	var buf strings.Builder
	Format(&buf, nil, FormatOpts{ShowTotal: true}, 80)
	if buf.String() != "" {
		t.Errorf("plain empty listing should print nothing, got %q", buf.String())
	}
}

func TestFormatCommaWrapping(t *testing.T) {
	files := []*FileInfo{
		{Name: "a"},
		{Name: "bb"},
		{Name: "c"},
	}
	var buf strings.Builder
	Format(&buf, files, FormatOpts{Comma: true, NoColor: true}, 5)
	if got, want := buf.String(), "a,\nbb, c\n"; got != want {
		t.Errorf("comma wrap at width 5 = %q, want %q", got, want)
	}
}

func TestFormatCommaExactFit(t *testing.T) {
	files := []*FileInfo{
		{Name: "a"},
		{Name: "bb"},
	}
	var buf strings.Builder
	Format(&buf, files, FormatOpts{Comma: true, NoColor: true}, 5)
	if got, want := buf.String(), "a, bb\n"; got != want {
		t.Errorf("comma exact fit at width 5 = %q, want %q", got, want)
	}
}

func TestFormatCommaSanitizesEmbeddedNewlineWhenBounded(t *testing.T) {
	files := []*FileInfo{{Name: "n\nl"}}
	var buf strings.Builder
	Format(&buf, files, FormatOpts{Comma: true, NoColor: true}, 80)
	if got, want := buf.String(), "n?l\n"; got != want {
		t.Errorf("bounded width should sanitize control chars, got %q want %q", got, want)
	}
}

func TestFormatCommaUnboundedWidthPreservesRawBytes(t *testing.T) {
	files := []*FileInfo{{Name: "n\nl"}}
	var buf strings.Builder
	Format(&buf, files, FormatOpts{Comma: true, NoColor: true}, 0)
	if got, want := buf.String(), "n\nl\n"; got != want {
		t.Errorf("unbounded width (-w0) should preserve raw bytes, got %q want %q", got, want)
	}
}

func TestFormatZeroSeparatesEntriesWithNUL(t *testing.T) {
	files := []*FileInfo{{Name: "a"}, {Name: "b"}}
	var buf strings.Builder
	Format(&buf, files, FormatOpts{OnePerLine: true, Zero: true, NoColor: true}, 80)
	if got, want := buf.String(), "a\x00b\x00"; got != want {
		t.Errorf("zero-separated onePerLine = %q, want %q", got, want)
	}
}

func TestEntryPrefixShowsQuestionMarkForBrokenDereferencedLink(t *testing.T) {
	fi := &FileInfo{Name: "dangle", IsSymlink: true, LinkBroken: true, Inode: 42, Blocks: 8}

	withDeref := entryPrefix(fi, FormatOpts{Inode: true, ShowBlocks: true, Dereference: true})
	if withDeref != "? ? " {
		t.Errorf("entryPrefix with dereference on broken link = %q, want %q", withDeref, "? ? ")
	}

	withoutDeref := entryPrefix(fi, FormatOpts{Inode: true, ShowBlocks: true})
	if withoutDeref != "42 4 " {
		t.Errorf("entryPrefix without dereference = %q, want %q", withoutDeref, "42 4 ")
	}
}

func TestQuotingAppliesToNameAndLinkTarget(t *testing.T) {
	fi := &FileInfo{Name: "symlink", IsSymlink: true, LinkTarget: "needs quoting"}
	opts := FormatOpts{Long: true, Quoting: "shell-escape", NoColor: true}
	got := entryNamePlain(fi, opts)
	if want := "symlink -> 'needs quoting'"; got != want {
		t.Errorf("quoted entry name = %q, want %q", got, want)
	}
}
