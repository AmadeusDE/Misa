package webui

import (
	"strings"

	"github.com/AmadeusDE/Misa/internal/term"
)

const (
	attrBold   = 1
	attrItalic = 2
	attrUnder  = 4
	attrStrike = 8
	attrWide   = 16
)

// Run is a stretch of cells in one style. A wide run holds exactly one
// double-width character.
type Run struct {
	T  string `json:"t"`
	FG string `json:"f,omitempty"`
	BG string `json:"b,omitempty"`
	A  int    `json:"a,omitempty"`
}

type RowUpdate struct {
	Y    int   `json:"y"`
	Runs []Run `json:"r"`
}

type Cursor struct {
	Row     int  `json:"r"`
	Col     int  `json:"c"`
	Visible bool `json:"v"`
}

// Frame is one SSE "frame" event: type "full" carries every line, "rows" only
// the changed ones.
type Frame struct {
	Type  string      `json:"type"`
	Rows  int         `json:"rows"`
	Cols  int         `json:"cols"`
	Seq   uint64      `json:"seq,omitempty"`
	FG    string      `json:"fg,omitempty"`
	BG    string      `json:"bg,omitempty"`
	Lines [][]Run     `json:"lines,omitempty"`
	Dirty []RowUpdate `json:"dirty,omitempty"`
	Cur   *Cursor     `json:"cursor,omitempty"`
	Mouse *bool       `json:"mouse,omitempty"`
	Title *string     `json:"title,omitempty"`
}

func cellAttrs(c *term.Cell) int {
	a := 0
	if c.Bold {
		a |= attrBold
	}
	if c.Italic {
		a |= attrItalic
	}
	if c.Under {
		a |= attrUnder
	}
	if c.Strike {
		a |= attrStrike
	}
	return a
}

func cellText(c *term.Cell) string {
	if c.Hidden {
		return " "
	}
	if c.Ch == 0 {
		return " " + c.Marks()
	}
	return string(c.Ch) + c.Marks()
}

func plainStyle(r Run) bool { return r.A == 0 && r.FG == "" && r.BG == "" }

// trimBlanks drops the default-styled spaces that end a row.
func trimBlanks(runs []Run) []Run {
	for len(runs) > 0 {
		last := &runs[len(runs)-1]
		if !plainStyle(*last) {
			break
		}
		last.T = strings.TrimRight(last.T, " ")
		if last.T != "" {
			break
		}
		runs = runs[:len(runs)-1]
	}
	return runs
}

// encodeRow compacts a row into runs of identical style with combining marks
// joined to their base and wide characters carrying no continuation cell.
func encodeRow(line []term.Cell, pal *termPalette) []Run {
	runs := []Run{}
	for i := range line {
		c := &line[i]
		if c.Continuation() {
			continue
		}
		fg, bg := pal.resolve(c)
		r := Run{T: cellText(c), FG: fg, BG: bg, A: cellAttrs(c)}
		if c.Wide && i+1 < len(line) && line[i+1].Continuation() {
			r.A |= attrWide
			runs = append(runs, r)
			continue
		}
		if n := len(runs); n > 0 && runs[n-1].A == r.A && runs[n-1].FG == r.FG && runs[n-1].BG == r.BG {
			runs[n-1].T += r.T
			continue
		}
		runs = append(runs, r)
	}
	return trimBlanks(runs)
}

// frameEncoder remembers what one connection was last sent.
type frameEncoder struct {
	pal    *termPalette
	prev   []term.Cell
	rows   int
	cols   int
	cursor Cursor
	mouse  bool
	title  string
	valid  bool
}

func newFrameEncoder() *frameEncoder { return &frameEncoder{pal: newTermPalette()} }

func (e *frameEncoder) remember(snap *term.Snapshot, mouse bool, title string) {
	e.prev = append(e.prev[:0], snap.Cells...)
	e.rows, e.cols, e.valid = snap.Rows, snap.Cols, true
	e.cursor = Cursor{snap.CursorRow, snap.CursorCol, snap.CursorVisible}
	e.mouse, e.title = mouse, title
}

func (e *frameEncoder) full(snap *term.Snapshot, mouse bool, title string, seq uint64) Frame {
	f := Frame{Type: "full", Rows: snap.Rows, Cols: snap.Cols, Seq: seq,
		FG: e.pal.fg.hex(), BG: e.pal.bg.hex(), Lines: make([][]Run, snap.Rows),
		Cur: &Cursor{snap.CursorRow, snap.CursorCol, snap.CursorVisible}, Mouse: &mouse, Title: &title}
	for y := 0; y < snap.Rows; y++ {
		f.Lines[y] = encodeRow(snap.Cells[y*snap.Cols:(y+1)*snap.Cols], e.pal)
	}
	e.remember(snap, mouse, title)
	return f
}

// diff describes how snap differs from what this connection last saw, or
// reports false when nothing changed. A size change sends everything.
func (e *frameEncoder) diff(snap *term.Snapshot, mouse bool, title string, seq uint64) (Frame, bool) {
	if !e.valid || e.rows != snap.Rows || e.cols != snap.Cols {
		return e.full(snap, mouse, title, seq), true
	}
	f := Frame{Type: "rows", Rows: snap.Rows, Cols: snap.Cols}
	for y := 0; y < snap.Rows; y++ {
		line := snap.Cells[y*snap.Cols : (y+1)*snap.Cols]
		if sameCells(line, e.prev[y*e.cols:(y+1)*e.cols]) {
			continue
		}
		f.Dirty = append(f.Dirty, RowUpdate{Y: y, Runs: encodeRow(line, e.pal)})
	}
	changed := len(f.Dirty) > 0
	if cur := (Cursor{snap.CursorRow, snap.CursorCol, snap.CursorVisible}); cur != e.cursor {
		f.Cur, changed = &cur, true
	}
	if mouse != e.mouse {
		f.Mouse, changed = &mouse, true
	}
	if title != e.title {
		f.Title, changed = &title, true
	}
	if changed {
		e.remember(snap, mouse, title)
	}
	return f, changed
}

func sameCells(a, b []term.Cell) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
