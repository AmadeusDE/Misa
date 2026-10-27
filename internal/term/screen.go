package term

import (
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// blank is an erased cell. Erasures take the current background (bce), which
// TERM=xterm-256color advertises and ncurses relies on.
func (t *Terminal) blank() Cell {
	return Cell{Ch: ' ', BG: t.attr.BG, HasBG: t.attr.HasBG}
}

// blankRow returns an erased row, reusing a recycled one when it can.
func (t *Terminal) blankRow() []Cell {
	var row []Cell
	if n := len(t.spare); n > 0 {
		row, t.spare = t.spare[n-1], t.spare[:n-1]
	} else {
		row = make([]Cell, t.Cols)
	}
	b := t.blank()
	for i := range row {
		row[i] = b
	}
	return row
}

// fill erases columns [from, to) of row.
func (t *Terminal) fill(row, from, to int) {
	if row < 0 || row >= t.Rows {
		return
	}
	from, to = max(from, 0), min(to, t.Cols)
	if from >= to {
		return
	}
	t.splitWideAt(row, from)
	t.splitWideAt(row, to)
	b := t.blank()
	for c := from; c < to; c++ {
		t.cells[row][c] = b
	}
}

// splitWideAt blanks a double-width character straddling the boundary before
// column col, so no half of it is left orphaned.
func (t *Terminal) splitWideAt(row, col int) {
	if col <= 0 || col >= t.Cols {
		return
	}
	r := t.cells[row]
	if r[col].cont && r[col-1].Wide {
		b := t.blank()
		r[col-1], r[col] = b, b
	}
}

// runeWidth is ansi's display width for r, cached per terminal: measuring
// each character as a grapheme dominated parsing non-ASCII output.
func (t *Terminal) runeWidth(r rune) int {
	if r < 0x300 {
		return 1
	}
	if w, ok := t.widths[r]; ok {
		return int(w)
	}
	w := ansi.StringWidth(string(r))
	if t.widths == nil {
		t.widths = map[rune]int8{}
	}
	if len(t.widths) < maxCachedWidths {
		t.widths[r] = int8(w)
	}
	return w
}

// asciiFastPath lets tests compare putASCII against putChar.
var asciiFastPath = true

// putASCII writes the run of printable ASCII at the start of data that ends
// before the last column, where wrapping starts, in one pass: the common case
// of plain output. It returns the bytes consumed; 0 leaves them to putChar.
func (t *Terminal) putASCII(data []byte) int {
	if !asciiFastPath || t.wrapNext || t.charsets.graphics() || t.curRow < 0 || t.curRow >= t.Rows {
		return 0
	}
	limit := min(len(data), t.Cols-1-t.curCol)
	n := 0
	for n < limit && data[n] >= 0x20 && data[n] < 0x7f {
		n++
	}
	if n == 0 {
		return 0
	}
	t.splitWideAt(t.curRow, t.curCol)
	t.splitWideAt(t.curRow, t.curCol+n)
	row := t.cells[t.curRow][t.curCol : t.curCol+n]
	c := t.attr
	c.marks, c.Wide, c.cont = 0, false, false
	for i, b := range data[:n] {
		c.Ch = rune(b)
		row[i] = c
	}
	t.curCol += n
	t.lastChar = rune(data[n-1])
	return n
}

func (t *Terminal) putChar(ch rune) {
	w := t.runeWidth(ch)
	if w == 0 {
		t.appendCombining(ch)
		return
	}
	if w > 2 {
		w = 2
	}
	if t.wrapNext {
		t.curCol = 0
		t.linefeed()
	}
	if w == 2 && t.curCol == t.Cols-1 {
		if t.noAutowrap || t.Cols < 2 {
			ch, w = ' ', 1
		} else {
			t.fill(t.curRow, t.curCol, t.Cols)
			t.curCol = 0
			t.linefeed()
		}
	}
	t.writeCell(ch, w)
	t.lastChar = ch
	t.curCol += w
	if t.curCol >= t.Cols {
		t.curCol = t.Cols - 1
		t.wrapNext = !t.noAutowrap
	}
}

func (t *Terminal) writeCell(ch rune, w int) {
	row := t.curRow
	if row < 0 || row >= t.Rows || t.curCol < 0 || t.curCol+w > t.Cols {
		return
	}
	t.splitWideAt(row, t.curCol)
	t.splitWideAt(row, t.curCol+w)
	c := t.attr
	c.Ch, c.marks, c.Wide, c.cont = ch, 0, w == 2, false
	t.cells[row][t.curCol] = c
	if w == 2 {
		c.Ch, c.Wide, c.cont = ' ', false, true
		t.cells[row][t.curCol+1] = c
	}
}

func (t *Terminal) appendCombining(ch rune) {
	col := t.curCol
	if !t.wrapNext {
		col--
	}
	if col < 0 || t.curRow < 0 || t.curRow >= t.Rows {
		return
	}
	r := t.cells[t.curRow]
	if r[col].cont && col > 0 {
		col--
	}
	marks := marksOf(r[col].marks)
	if utf8.RuneCountInString(marks) >= maxMarksPerCell {
		return
	}
	r[col].marks = internMarks(marks + string(ch))
}

func (t *Terminal) repeatLast(n int) {
	if t.lastChar == 0 {
		return
	}
	n = min(n, t.Rows*t.Cols)
	for i := 0; i < n; i++ {
		t.putChar(t.lastChar)
	}
}

func (t *Terminal) tabForward(n int) {
	for ; n > 0 && t.curCol < t.Cols-1; n-- {
		t.curCol = min((t.curCol+8)&^7, t.Cols-1)
	}
	t.wrapNext = false
}

func (t *Terminal) tabBackward(n int) {
	for ; n > 0 && t.curCol > 0; n-- {
		t.curCol = (t.curCol - 1) &^ 7
	}
	t.wrapNext = false
}

func (t *Terminal) linefeed() {
	t.wrapNext = false
	if t.curRow == t.scrollBot {
		t.scrollUp()
	} else if t.curRow < t.Rows-1 {
		t.curRow++
	}
}

func (t *Terminal) reverseLinefeed() {
	t.wrapNext = false
	if t.curRow == t.scrollTop {
		t.scrollDown()
	} else if t.curRow > 0 {
		t.curRow--
	}
}

func (t *Terminal) scrollUp() {
	t.scrollRegionUp(t.scrollTop, 1, true)
}

func (t *Terminal) scrollDown() {
	t.scrollRegionDown(t.scrollTop, 1)
}

func (t *Terminal) scrollUpN(n int) {
	t.scrollRegionUp(t.scrollTop, n, true)
}

func (t *Terminal) scrollDownN(n int) {
	t.scrollRegionDown(t.scrollTop, n)
}

// scrollRegionUp shifts rows [top, scrollBot] up by n and blanks the vacated
// bottom rows. Rows leaving the top of the main screen go to scrollback.
func (t *Terminal) scrollRegionUp(top, n int, toScrollback bool) {
	bot := t.scrollBot
	if top > bot {
		return
	}
	n = min(n, bot-top+1)
	keep := toScrollback && !t.altScreen && top == 0
	for _, row := range t.cells[top : top+n] {
		if keep {
			t.appendScrollback(row)
		} else {
			t.recycle(row)
		}
	}
	copy(t.cells[top:], t.cells[top+n:bot+1])
	for r := bot - n + 1; r <= bot; r++ {
		t.cells[r] = t.blankRow()
	}
}

func (t *Terminal) scrollRegionDown(top, n int) {
	bot := t.scrollBot
	if top > bot {
		return
	}
	n = min(n, bot-top+1)
	for _, row := range t.cells[bot+1-n : bot+1] {
		t.recycle(row)
	}
	copy(t.cells[top+n:bot+1], t.cells[top:bot+1-n])
	for r := top; r < top+n; r++ {
		t.cells[r] = t.blankRow()
	}
}

func (t *Terminal) insertLines(n int) {
	if t.curRow < t.scrollTop || t.curRow > t.scrollBot {
		return
	}
	t.scrollRegionDown(t.curRow, n)
	t.curCol = 0
	t.wrapNext = false
}

func (t *Terminal) deleteLines(n int) {
	if t.curRow < t.scrollTop || t.curRow > t.scrollBot {
		return
	}
	t.scrollRegionUp(t.curRow, n, false)
	t.curCol = 0
	t.wrapNext = false
}

func (t *Terminal) deleteChars(n int) {
	n = min(n, t.Cols-t.curCol)
	row := t.cells[t.curRow]
	t.splitWideAt(t.curRow, t.curCol)
	t.splitWideAt(t.curRow, t.curCol+n)
	copy(row[t.curCol:], row[t.curCol+n:])
	t.fill(t.curRow, t.Cols-n, t.Cols)
	t.wrapNext = false
}

func (t *Terminal) insertChars(n int) {
	n = min(n, t.Cols-t.curCol)
	row := t.cells[t.curRow]
	t.splitWideAt(t.curRow, t.curCol)
	t.splitWideAt(t.curRow, t.Cols-n)
	copy(row[t.curCol+n:], row[t.curCol:t.Cols-n])
	t.fill(t.curRow, t.curCol, t.curCol+n)
	t.wrapNext = false
}

func (t *Terminal) eraseDisplay(mode int) {
	switch mode {
	case 0:
		t.clearRange(t.curRow, t.curCol, t.Rows-1, t.Cols-1)
	case 1:
		t.clearRange(0, 0, t.curRow, t.curCol)
	case 2:
		t.clearRange(0, 0, t.Rows-1, t.Cols-1)
	case 3:
		t.scrollback = nil
	}
}

func (t *Terminal) eraseLine(mode int) {
	switch mode {
	case 0:
		t.fill(t.curRow, t.curCol, t.Cols)
	case 1:
		t.fill(t.curRow, 0, t.curCol+1)
	case 2:
		t.fill(t.curRow, 0, t.Cols)
	}
}

func (t *Terminal) clearRange(r1, c1, r2, c2 int) {
	for r := r1; r <= r2 && r < t.Rows; r++ {
		sc, ec := 0, t.Cols-1
		if r == r1 {
			sc = c1
		}
		if r == r2 {
			ec = c2
		}
		t.fill(r, sc, ec+1)
	}
}

func (t *Terminal) clamp() {
	t.curRow, t.curCol = t.clampPos(t.curRow, t.curCol)
}

// reset is RIS (ESC c): back to the main screen with default modes.
func (t *Terminal) reset() {
	t.exitAlt()
	t.attr = Cell{}
	t.charsets = charsets{}
	t.saved = savedCursor{}
	t.curRow, t.curCol = 0, 0
	t.scrollTop = 0
	t.scrollBot = t.Rows - 1
	t.wrapNext = false
	t.noAutowrap = false
	t.appCursorKeys = false
	t.bracketedPaste = false
	t.cursorHidden = false
	t.mouse = MouseMode{}
	t.clearRange(0, 0, t.Rows-1, t.Cols-1)
}
