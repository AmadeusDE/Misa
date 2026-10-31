package vi

import "strings"

// Counts saturate before arithmetic can overflow, including on 32-bit targets.
const maxCommandCount = 999999999

// Content-producing commands are limited to avoid unbounded allocations.
const maxContentRepeats = 10000

func countDigit(count, digit int) int {
	if count > (maxCommandCount-digit)/10 {
		return maxCommandCount
	}
	return count*10 + digit
}
func countProduct(a, b int) int {
	if a > maxCommandCount/b {
		return maxCommandCount
	}
	return a * b
}
func (e *Editor) clearCommandState() {
	e.pendingKey = ""
	e.pendingOp = 0
	e.count, e.opCount = 0, 0
	e.countGiven, e.opCountGiven = false, false
	e.hasReg, e.pendingReg = false, 0
}

// No non-wrapping motion can usefully traverse more positions than the buffer.
func (e *Editor) motionLimit(count int) int {
	limit := 1
	for _, line := range e.lines {
		if len(line) >= count-limit {
			return count
		}
		limit += len(line) + 1
	}
	return min(count, limit)
}

func (e *Editor) deleteToCountedEnd(count int, change bool) {
	end := e.row + min(count-1, len(e.lines)-1-e.row)
	endCol := len(e.lineRunes(end))
	e.saveUndo()
	e.applyCharOp('d', e.row, e.col, end, endCol, "$", count)
	if change {
		e.mode = ModeInsert
		e.insertEntry = "C"
		e.insertBuf.Reset()
		e.insertCount = 0
	}
}

func (e *Editor) putCounted(before bool, count int) {
	count = min(count, maxContentRepeats)
	original := e.register
	if e.regLine {
		repeated := make([]string, 0, len(original)*count)
		for i := 0; i < count; i++ {
			repeated = append(repeated, original...)
		}
		e.register = repeated
	} else {
		text := strings.Repeat(strings.Join(original, "\n"), count)
		e.register = strings.Split(text, "\n")
	}
	e.doPut(before)
	e.register = original
}

func (e *Editor) deleteCountedChars(count int, backwards, register bool) {
	r := e.lineRunes(e.row)
	start, end := e.col, e.col
	if backwards {
		start -= min(count, start)
	} else {
		end += min(count, len(r)-end)
	}
	e.saveUndo()
	if end == start {
		return
	}
	if register {
		e.deleteToReg([]string{string(r[start:end])}, false)
	}
	e.noteEdit(e.row, start)
	e.setLine(e.row, string(r[:start])+string(r[end:]))
	e.col = start
}

func (e *Editor) joinCounted(joins int) {
	var b strings.Builder
	b.WriteString(e.lines[e.row])
	col := 0
	width := len([]rune(e.lines[e.row]))
	for i := 1; i <= joins; i++ {
		col = width
		next := strings.TrimLeft(e.lines[e.row+i], " \t")
		if b.Len() > 0 && len(next) > 0 {
			b.WriteByte(' ')
			width++
		}
		b.WriteString(next)
		width += len([]rune(next))
	}
	e.noteEdit(e.row, len([]rune(e.lines[e.row])))
	e.lines[e.row] = b.String()
	e.lines = append(e.lines[:e.row+1], e.lines[e.row+joins+1:]...)
	e.col = col
	e.modified = true
}
