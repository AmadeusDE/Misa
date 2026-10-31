package vi

import (
	"reflect"
	"testing"

	"github.com/AmadeusDE/Misa/internal/vi/vipat"
)

func TestSearchHighlightDisplayCells(t *testing.T) {
	re, err := vipat.Compile("\\t\\|界\\|é", false)
	if err != nil {
		t.Fatal(err)
	}
	line := "a\t界e\u0301 é"
	layout := newLayout(line, 8)
	marks := lineMarks{search: searchLineMarks(layout, line, re)}
	got := layout.window(5, 8, marks)
	want := []styledRun{{styleSearch, "   界"}, {stylePlain, "e\u0301 "}, {styleSearch, "é"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runs = %#v, want %#v", got, want)
	}
	// A partial wide glyph still highlights its visible placeholder cell.
	got = layout.window(9, 1, marks)
	want = []styledRun{{styleSearch, " "}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clipped runs = %#v, want %#v", got, want)
	}
}

func TestSearchHighlightCombiningAndPriority(t *testing.T) {
	re, _ := vipat.Compile("\u0301", false)
	line := "e\u0301x"
	layout := newLayout(line, 8)
	marks := lineMarks{search: searchLineMarks(layout, line, re), selected: func(i int) bool { return i == 0 }}
	got := layout.window(0, 2, marks)
	want := []styledRun{{styleSelected, "e\u0301"}, {stylePlain, "x"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected runs = %#v, want %#v", got, want)
	}
	marks.selected = nil
	marks.cursorN = 1
	got = layout.window(0, 2, marks)
	want = []styledRun{{styleCursor, "e\u0301"}, {stylePlain, "x"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cursor runs = %#v, want %#v", got, want)
	}
	marks.cursorN = 0
	got = layout.window(0, 2, marks)
	want = []styledRun{{styleSearch, "e\u0301"}, {stylePlain, "x"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("search runs = %#v, want %#v", got, want)
	}
}

func TestSearchHighlightEmptyMatches(t *testing.T) {
	re, _ := vipat.Compile("a*", false)
	line := "baa"
	layout := newLayout(line, 8)
	marks := lineMarks{search: searchLineMarks(layout, line, re)}
	got := layout.window(0, 4, marks)
	want := []styledRun{{stylePlain, "b"}, {styleSearch, "aa"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runs = %#v, want %#v", got, want)
	}
}

func TestPreviewHighlightStylePriority(t *testing.T) {
	l := newLayout("abc", 8)
	marks := lineMarks{
		search:        func(int) bool { return true },
		searchCurrent: func(i int) bool { return i < 2 },
		selected:      func(i int) bool { return i == 1 },
		cursorStart:   2, cursorN: 1,
	}
	got := l.window(0, 3, marks)
	want := []styledRun{{styleSearchCurrent, "a"}, {styleSelected, "b"}, {styleCursor, "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runs = %#v, want %#v", got, want)
	}
}
