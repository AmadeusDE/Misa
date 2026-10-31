package vi

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// defaultTabstop is the tab width until a 'tabstop' option exists; everything
// that maps text to screen columns goes through Editor.tabstop.
const defaultTabstop = 8

func (e *Editor) tabstop() int { return defaultTabstop }

type cellKind uint8

const (
	cellText cellKind = iota // one grapheme cluster drawn as-is
	cellTab                  // spaces up to the next tab stop
	cellCtrl                 // visible ASCII form of a control character: ^X, <80>
	cellNone                 // zero-width: continuation rune of a cluster, or an invisible mark
)

// cellSpan is the screen footprint of one rune. Continuation runes of a grapheme
// cluster (combining marks, ZWJ parts) share the lead rune's column and have width 0.
type cellSpan struct {
	col, width, lead int
	kind             cellKind
	text             string
}

// lineLayout maps rune indexes to display columns and back. Invalid UTF-8 bytes
// are one rune each (U+FFFD, one cell), matching []rune(line).
type lineLayout struct {
	spans []cellSpan
	width int
}

func newLayout(line string, tabstop int) lineLayout {
	if tabstop < 1 {
		tabstop = defaultTabstop
	}
	l := lineLayout{spans: make([]cellSpan, 0, len(line))}
	col := 0
	for rest, state := line, -1; rest != ""; {
		var cluster string
		var w int
		cluster, rest, w, state = uniseg.FirstGraphemeClusterInString(rest, state)
		idx := len(l.spans)
		first, _ := utf8.DecodeRuneInString(cluster)
		span := cellSpan{col: col, lead: idx, kind: cellText, text: cluster, width: w}
		switch {
		case first == '\t':
			span.kind, span.width, span.text = cellTab, tabstop-col%tabstop, ""
		case first < 0x20 || first == 0x7f:
			span.kind, span.text = cellCtrl, ctrlText(first)
			span.width = len(span.text)
		case first >= 0x80 && first < 0xa0:
			span.kind, span.text = cellCtrl, fmt.Sprintf("<%02x>", first)
			span.width = len(span.text)
		case first == utf8.RuneError:
			span.text, span.width = "�", 1
		case w <= 0:
			span.kind, span.text, span.width = cellNone, "", 0
		case w > 2:
			span.width = 2
		}
		l.spans = append(l.spans, span)
		col += span.width
		for n := utf8.RuneCountInString(cluster) - 1; n > 0; n-- {
			l.spans = append(l.spans, cellSpan{col: span.col, lead: idx, kind: cellNone})
		}
	}
	l.width = col
	return l
}

func ctrlText(r rune) string {
	if r == 0x7f {
		return "^?"
	}
	return "^" + string(r+64)
}

// startCol is the first display column of rune i; past the end it is the line width.
func (l lineLayout) startCol(i int) int {
	if i < 0 {
		return 0
	}
	if i >= len(l.spans) {
		return l.width
	}
	return l.spans[i].col
}

// runeAt is the rune index whose cells contain display column col, or the rune
// count when col is past the end of the line.
func (l lineLayout) runeAt(col int) int {
	i := sort.Search(len(l.spans), func(i int) bool { return l.spans[i].col+l.spans[i].width > col })
	if i < len(l.spans) {
		return l.spans[i].lead
	}
	return i
}

// cursorCells is the display columns [start, start+n) the cursor covers on rune
// col. Normal mode sits on the last cell of a tab (as Vim does) and the first of
// everything else; insert mode sits before the character, on its first cell.
func (l lineLayout) cursorCells(col int, insert bool) (start, n int) {
	if col >= len(l.spans) {
		return l.width, 1
	}
	s := l.spans[l.spans[col].lead]
	switch {
	case s.kind == cellTab && !insert:
		return s.col + s.width - 1, 1
	case s.kind == cellTab || s.kind == cellNone:
		return s.col, 1
	}
	return s.col, s.width
}

type cellStyle uint8

const (
	stylePlain cellStyle = iota
	styleSearch
	styleSearchCurrent
	styleSelected
	styleCursor
)

// lineMarks says which cells of a rendered line are highlighted. selected is
// asked with the lead rune index of the cluster under the cell.
type lineMarks struct {
	cursorStart, cursorN int
	selected             func(lead int) bool
	search               func(lead int) bool
	searchCurrent        func(lead int) bool
	eolSelected          bool
}

type styledRun struct {
	style cellStyle
	text  string
}

// window renders display columns [left, left+w) as runs of (style, text); it
// never produces more than w cells. A wide cluster cut by either edge becomes a
// space, so every emitted cell lines up with the layout.
func (l lineLayout) window(left, w int, m lineMarks) []styledRun {
	var out []styledRun
	emit := func(col int, lead int, text string) {
		st := stylePlain
		if m.search != nil && lead >= 0 && m.search(lead) {
			st = styleSearch
		}
		if m.searchCurrent != nil && lead >= 0 && m.searchCurrent(lead) {
			st = styleSearchCurrent
		}
		if m.selected != nil && lead >= 0 && m.selected(lead) {
			st = styleSelected
		} else if lead < 0 && m.eolSelected {
			st = styleSelected
		}
		if col >= m.cursorStart && col < m.cursorStart+m.cursorN {
			st = styleCursor
		}
		if k := len(out) - 1; k >= 0 && out[k].style == st {
			out[k].text += text
			return
		}
		out = append(out, styledRun{st, text})
	}
	// Search by start column, which remains monotonic across continuation
	// runes; col+width is not monotonic within a grapheme cluster.
	start := sort.Search(len(l.spans), func(i int) bool { return l.spans[i].col >= left })
	if start > 0 {
		lead := l.spans[start-1].lead
		if s := l.spans[lead]; s.col+s.width > left {
			start = lead
		}
	}
	for i := start; i < len(l.spans); i++ {
		s := l.spans[i]
		if s.col >= left+w {
			break
		}
		if s.width == 0 {
			continue
		}
		whole := s.col >= left && s.col+s.width <= left+w
		for k := 0; k < s.width; k++ {
			c := s.col + k
			if c < left || c >= left+w {
				continue
			}
			switch {
			case s.kind == cellTab:
				emit(c, i, " ")
			case s.kind == cellCtrl:
				emit(c, i, s.text[k:k+1])
			case s.width == 1:
				emit(c, i, s.text)
			case whole && k == 0:
				emit(c, i, s.text)
			case whole:
				emit(c, i, "")
			default:
				emit(c, i, " ")
			}
		}
	}
	if end := l.width; end >= left && end < left+w && (m.eolSelected || (m.cursorN > 0 && m.cursorStart >= end)) {
		emit(end, -1, " ")
	}
	return out
}
