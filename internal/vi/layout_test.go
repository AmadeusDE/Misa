package vi

import (
	"strings"
	"testing"
)

func TestLayoutColumns(t *testing.T) {
	tests := []struct {
		name, line string
		width      int
		starts     []int // startCol of each rune
	}{
		{"ascii", "abc", 3, []int{0, 1, 2}},
		{"tab", "a\tb", 9, []int{0, 1, 8}},
		{"tab at stop", "12345678\tx", 17, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 16}},
		{"cjk", "a世b", 4, []int{0, 1, 3}},
		{"combining", "éx", 2, []int{0, 0, 1}},
		{"emoji zwj", "👨‍👩‍👧x", 3, []int{0, 0, 0, 0, 0, 2}},
		{"control", "a\x01b", 4, []int{0, 1, 3}},
		{"del", "\x7f", 2, []int{0}},
		{"c1", "\u0085", 4, []int{0}},
		{"invalid", "a\xffb\xfe", 4, []int{0, 1, 2, 3}},
		{"lone combining", "́a", 1, []int{0, 0}},
		{"zero width space", "a​b", 2, []int{0, 1, 1}},
	}
	for _, tc := range tests {
		l := newLayout(tc.line, 8)
		if l.width != tc.width {
			t.Errorf("%s: width %d, want %d", tc.name, l.width, tc.width)
		}
		if len(l.spans) != len(tc.starts) {
			t.Errorf("%s: %d runes, want %d", tc.name, len(l.spans), len(tc.starts))
			continue
		}
		for i, want := range tc.starts {
			if got := l.startCol(i); got != want {
				t.Errorf("%s: startCol(%d) = %d, want %d", tc.name, i, got, want)
			}
		}
	}
}

func TestLayoutRuneAtInvertsStartCol(t *testing.T) {
	l := newLayout("a\t世é\x01z", 8)
	for i := range l.spans {
		if l.spans[i].width == 0 {
			continue
		}
		for c := l.spans[i].col; c < l.spans[i].col+l.spans[i].width; c++ {
			if got := l.runeAt(c); got != i {
				t.Errorf("runeAt(%d) = %d, want %d", c, got, i)
			}
		}
	}
	if got := l.runeAt(l.width); got != len(l.spans) {
		t.Errorf("runeAt(width) = %d, want %d", got, len(l.spans))
	}
}

func TestCursorCells(t *testing.T) {
	l := newLayout("ab\t世", 8)
	tests := []struct {
		col      int
		insert   bool
		start, n int
	}{
		{0, false, 0, 1},
		{2, false, 7, 1}, // normal mode: last cell of the tab
		{2, true, 2, 1},  // insert mode: before the tab
		{3, false, 8, 2}, // wide: both cells
		{4, true, 10, 1}, // end of line
	}
	for _, tc := range tests {
		s, n := l.cursorCells(tc.col, tc.insert)
		if s != tc.start || n != tc.n {
			t.Errorf("cursorCells(%d,%v) = %d,%d want %d,%d", tc.col, tc.insert, s, n, tc.start, tc.n)
		}
	}
}

func render(l lineLayout, left, w int, m lineMarks) string {
	var b strings.Builder
	for _, r := range l.window(left, w, m) {
		b.WriteString(r.text)
	}
	return b.String()
}

func TestWindowClipsWideAndTabs(t *testing.T) {
	l := newLayout("a世b\tc", 8)
	tests := []struct {
		left, w int
		want    string
	}{
		{0, 20, "a世b    c"},
		{0, 2, "a "},  // wide char cut on the right edge
		{2, 3, " b "}, // wide char cut on the left edge, tab starts
		{4, 3, "   "}, // inside the tab
		{8, 5, "c"},   // clipped by the end of the line
		{20, 5, ""},   // scrolled past the end
		{1, 2, "世"},   // exactly one wide cell pair
		{0, 0, ""},    // degenerate pane
		{3, 100, "b    c"},
	}
	for _, tc := range tests {
		if got := render(l, tc.left, tc.w, lineMarks{}); got != tc.want {
			t.Errorf("window(%d,%d) = %q, want %q", tc.left, tc.w, got, tc.want)
		}
	}
}

func TestWindowMarksFollowCells(t *testing.T) {
	l := newLayout("a世\tb", 8)
	runs := l.window(0, 20, lineMarks{
		cursorStart: 1, cursorN: 2,
		selected: func(lead int) bool { return lead == 2 },
	})
	var got []cellStyle
	for _, r := range runs {
		got = append(got, r.style)
	}
	want := []cellStyle{stylePlain, styleCursor, styleSelected, stylePlain}
	if len(got) != len(want) {
		t.Fatalf("runs %+v", runs)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("styles %v want %v", got, want)
		}
	}
	if runs[2].text != strings.Repeat(" ", 5) {
		t.Errorf("tab selection = %q", runs[2].text)
	}
}
