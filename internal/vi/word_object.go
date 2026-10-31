package vi

import "unicode"

// Word text objects: iw, aw, iW and aW. The paired and quoted objects of H05
// (delim_object.go) share the span form and the dispatch below.
//
// The buffer is described with runs: maximal stretches of one character class
// inside a single line. A line break separates the runs of two lines and never
// forms a run of its own. Three classes describe a word:
//
//	blank  spaces and tabs
//	word   letters, digits and underscore
//	punct  every other non-blank character
//
// The capital objects collapse word and punct, so "a.b" is one word for iW and
// three for iw.
//
// An object is a half-open span [from, to) of character positions. A column
// equal to the row length is the line break itself, so a span ending there
// leaves the break behind and one ending at the next row's column zero takes
// it. Selection, deletion and yanking therefore share one range form and need
// no inclusive flag.
//
// White space between words counts as a word for iw, which is why "2iw" takes
// a word and the space after it. aw takes the words plus the white space that
// surrounds them: the space after the last word or, when that word ends its
// line, the space in front of the first one.
//
// Scanning crosses line breaks. An empty line is the end of an object: it is
// taken as whole lines and the cursor of a counted object stops there, so
// "diw" on an empty line does nothing and "2iw" from the word above it takes
// that word and the empty line. Visual selections follow the same rule, but
// they stay character ranges: an object that reached an empty line ends on the
// line before it and the operator decides whether whole lines are removed.

// textPos addresses one character of the buffer. col runs from zero to the
// length of the row; the length itself is the line break (or the end of the
// last line), which lets a span include or exclude the break.
type textPos struct{ row, col int }

// textSpan is a half-open character span [from, to). whole marks a span that
// stands for the complete lines from.row through to.row inclusive; from and to
// are then both line starts.
type textSpan struct {
	from, to textPos
	whole    bool
}

// runKind is the character class of a run.
type runKind uint8

const (
	runBlank runKind = iota // white space
	runWord                 // letters, digits and underscore
	runPunct                // any other non-blank character
)

// run is a maximal stretch of one class inside one line, [from, to).
type run struct {
	kind     runKind
	from, to int
}

// classOf reports the class of one character. The capital objects treat every
// non-blank character alike.
func classOf(r rune, big bool) runKind {
	if unicode.IsSpace(r) {
		return runBlank
	}
	if big || isWordChar(r) {
		return runWord
	}
	return runPunct
}

// lineRuns splits a row into runs.
func (e *Editor) lineRuns(row int, big bool) []run {
	runes := e.lineRunes(row)
	runs := make([]run, 0, 8)
	for i := 0; i < len(runes); {
		kind := classOf(runes[i], big)
		j := i + 1
		for j < len(runes) && classOf(runes[j], big) == kind {
			j++
		}
		runs = append(runs, run{kind: kind, from: i, to: j})
		i = j
	}
	return runs
}

// runAt finds the run holding col, which must be inside the row.
func runAt(runs []run, col int) (int, bool) {
	for i, r := range runs {
		if col >= r.from && col < r.to {
			return i, true
		}
	}
	return 0, false
}

// firstWordRun finds the first non-blank run at or after col.
func firstWordRun(runs []run, col int) (int, bool) {
	for i, r := range runs {
		if r.to > col && r.kind != runBlank {
			return i, true
		}
	}
	return 0, false
}

// lastWordRun finds the last non-blank run that starts before col.
func lastWordRun(runs []run, col int) (int, bool) {
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].from < col && runs[i].kind != runBlank {
			return i, true
		}
	}
	return 0, false
}

// --- objects ---------------------------------------------------------------

// textObject computes the span of a word object for the cursor at. When the
// scan runs out of text it reports false and stop, the position it reached;
// the cursor is left there, as if the object had been counted past the end.
func (e *Editor) textObject(at textPos, count int, big, around bool) (span textSpan, stop textPos, ok bool) {
	if at.row < 0 || at.row >= len(e.lines) {
		return textSpan{}, at, false
	}
	if around {
		return e.aroundObject(at, count, big)
	}
	runs := e.lineRuns(at.row, big)
	idx, found := runAt(runs, at.col)
	if !found {
		// An empty line (or the position of its break) holds no word.
		return textSpan{}, at, false
	}
	return e.innerObject(at.row, runs, idx, count, big)
}

// innerObject selects count runs, starting with the run under the cursor.
func (e *Editor) innerObject(row int, runs []run, idx, count int, big bool) (textSpan, textPos, bool) {
	span := textSpan{from: textPos{row, runs[idx].from}, to: textPos{row, runs[idx].to}}
	for n := 1; n < count; n++ {
		next, whole, ok := e.nextRunEnd(span.to, big)
		if !ok {
			return textSpan{}, span.to, false
		}
		span.to, span.whole = next, whole
	}
	return span, span.to, true
}

// nextRunEnd extends a span by one run: the next run of the line, or, at the
// end of a line, the first run after the break. An empty line is taken as
// whole lines instead of a run.
func (e *Editor) nextRunEnd(from textPos, big bool) (textPos, bool, bool) {
	if from.row < 0 || from.row >= len(e.lines) {
		return from, false, false
	}
	if from.col < len(e.lineRunes(from.row)) {
		runs := e.lineRuns(from.row, big)
		if idx, ok := runAt(runs, from.col); ok {
			return textPos{from.row, runs[idx].to}, false, true
		}
		return from, false, false
	}
	next := from.row + 1
	if next >= len(e.lines) {
		return from, false, false
	}
	if len(e.lineRunes(next)) == 0 {
		return textPos{next, 0}, true, true
	}
	runs := e.lineRuns(next, big)
	return textPos{next, runs[0].to}, false, true
}

// aroundObject selects count words plus the white space that surrounds them.
func (e *Editor) aroundObject(at textPos, count int, big bool) (textSpan, textPos, bool) {
	runs := e.lineRuns(at.row, big)
	idx, found := runAt(runs, at.col)
	var from, to textPos
	leading := true
	if found && runs[idx].kind != runBlank {
		leading = false
		from = textPos{at.row, runs[idx].from}
		to = textPos{at.row, runs[idx].to}
	} else {
		// White space, an empty line or the break itself: the object starts
		// here and takes the word that follows, which keeps the white space.
		from = at
		if found {
			from = textPos{at.row, runs[idx].from}
		}
		_, wordTo, _, ok := e.nextWord(from, big, false)
		if !ok {
			return textSpan{}, at, false
		}
		to = wordTo
	}
	span := textSpan{from: from, to: to}
	for n := 1; n < count; n++ {
		wordFrom, wordTo, whole, ok := e.nextWord(span.to, big, true)
		if !ok {
			return textSpan{}, span.to, false
		}
		if whole {
			// The empty line is part of the object; a further count walks on
			// from there.
			span.to, span.whole = wordFrom, true
			continue
		}
		span.to, span.whole = wordTo, false
	}
	if !span.whole && !leading {
		// White space after the last word, else in front of the first one.
		if end, ok := e.trailingSpace(span.to, big); ok {
			span.to = end
		} else if front, ok := e.leadingSpace(span.from, big); ok {
			span.from = front
		}
	}
	return span, span.to, true
}

// nextWord finds the first word at or after pos, crossing line breaks. With
// stopEmpty an empty line ends the search instead and is reported as whole
// together with its start.
func (e *Editor) nextWord(pos textPos, big, stopEmpty bool) (from, to textPos, whole, ok bool) {
	row, col := pos.row, pos.col
	for {
		if row >= len(e.lines) {
			return textPos{}, textPos{}, false, false
		}
		runs := e.lineRuns(row, big)
		if idx, hit := firstWordRun(runs, col); hit {
			return textPos{row, runs[idx].from}, textPos{row, runs[idx].to}, false, true
		}
		row++
		if row >= len(e.lines) {
			return textPos{}, textPos{}, false, false
		}
		if stopEmpty && len(e.lineRunes(row)) == 0 {
			return textPos{row, 0}, textPos{row, 0}, true, true
		}
		col = 0
	}
}

// nonBlankAt reports whether p holds a word or punctuation character.
func (e *Editor) nonBlankAt(p textPos, big bool) bool {
	runs := e.lineRuns(p.row, big)
	idx, ok := runAt(runs, p.col)
	return ok && runs[idx].kind != runBlank
}

// trailingSpace reports where the blank run that follows pos ends.
func (e *Editor) trailingSpace(pos textPos, big bool) (textPos, bool) {
	runs := e.lineRuns(pos.row, big)
	if idx, ok := runAt(runs, pos.col); ok && runs[idx].kind == runBlank {
		return textPos{pos.row, runs[idx].to}, true
	}
	return pos, false
}

// leadingSpace reports where the blank run in front of pos starts.
func (e *Editor) leadingSpace(pos textPos, big bool) (textPos, bool) {
	runs := e.lineRuns(pos.row, big)
	if idx, ok := runAt(runs, pos.col); ok && idx > 0 && runs[idx-1].kind == runBlank {
		return textPos{pos.row, runs[idx-1].from}, true
	}
	return pos, false
}

// --- growing an existing Visual selection ----------------------------------

// growForward extends a Visual object to the right by count units.
func (e *Editor) growForward(span textSpan, count int, big, around bool) (textSpan, bool) {
	for n := 0; n < count; n++ {
		if around {
			_, to, whole, ok := e.nextWord(span.to, big, true)
			if !ok {
				return textSpan{}, false
			}
			if whole {
				span.to, span.whole = to, true
				continue
			}
			// White space in front of a word belongs to the word before it,
			// so a step that starts on a word also takes the space after the
			// next one, while a step that starts on space stops at the word.
			if e.nonBlankAt(span.to, big) {
				if end, ok := e.trailingSpace(to, big); ok {
					to = end
				}
			}
			span.to, span.whole = to, false
			continue
		}
		to, whole, ok := e.nextRunEnd(span.to, big)
		if !ok {
			return textSpan{}, false
		}
		span.to, span.whole = to, whole
	}
	return span, true
}

// growBackward extends a Visual object to the left by count units.
func (e *Editor) growBackward(span textSpan, count int, big, around bool) (textSpan, bool) {
	for n := 0; n < count; n++ {
		from, ok := e.prevUnitStart(span.from, big, around)
		if !ok {
			return textSpan{}, false
		}
		span.from = from
	}
	return span, true
}

// prevUnitStart is the start of the unit in front of from: the previous run
// for iw, the previous word (with the white space in front of it) for aw.
func (e *Editor) prevUnitStart(from textPos, big, around bool) (textPos, bool) {
	if around {
		if word, ok := e.prevWordStart(from, big); ok {
			if front, ok := e.leadingSpace(word, big); ok {
				return front, true
			}
			return word, true
		}
		return from, false
	}
	return e.prevRunStart(from, big)
}

// prevRunStart is the start of the run before from.
func (e *Editor) prevRunStart(from textPos, big bool) (textPos, bool) {
	runs := e.lineRuns(from.row, big)
	if idx, ok := runAt(runs, from.col); ok && idx > 0 {
		return textPos{from.row, runs[idx-1].from}, true
	}
	for row := from.row - 1; row >= 0; row-- {
		prev := e.lineRuns(row, big)
		if len(prev) == 0 {
			return textPos{row, 0}, true
		}
		return textPos{row, prev[len(prev)-1].from}, true
	}
	return from, false
}

// prevWordStart is the start of the last word before from.
func (e *Editor) prevWordStart(from textPos, big bool) (textPos, bool) {
	runs := e.lineRuns(from.row, big)
	if idx, ok := lastWordRun(runs, from.col); ok {
		return textPos{from.row, runs[idx].from}, true
	}
	for row := from.row - 1; row >= 0; row-- {
		prev := e.lineRuns(row, big)
		if idx, ok := lastWordRun(prev, len(e.lineRunes(row))+1); ok {
			return textPos{row, prev[idx].from}, true
		}
	}
	return from, false
}

// backwardObject computes the object that ends at the cursor. It is used when
// a Visual selection was started to the right of the cursor: the object then
// grows towards the anchor instead of away from it.
func (e *Editor) backwardObject(cursor textPos, count int, big, around bool) (textSpan, bool) {
	runs := e.lineRuns(cursor.row, big)
	idx, found := runAt(runs, cursor.col)
	var from textPos
	if found && runs[idx].kind != runBlank {
		from = textPos{cursor.row, runs[idx].from}
	} else {
		start := cursor
		if found {
			start = textPos{cursor.row, runs[idx].from}
		}
		word, ok := e.prevWordStart(start, big)
		if !ok {
			return textSpan{}, false
		}
		from = word
	}
	span := textSpan{from: from, to: cursor}
	if around {
		if front, ok := e.leadingSpace(from, big); ok {
			span.from = front
		}
	}
	return e.growBackward(span, count-1, big, around)
}

// --- operator dispatch -----------------------------------------------------

// executeObjectOperator applies d, c, y, >, < or ! to a word object.
func (e *Editor) executeObjectOperator(op byte, key string, count int) {
	if d, ok := delimObjectFor(key); ok {
		if span, ok := e.delimSpan(d, key[0] == 'a', textPos{e.row, e.col}, count); ok {
			e.applyDelimSpan(op, key, count, span)
		}
		return
	}
	big, around := key[1] == 'W', key[0] == 'a'
	span, stop, ok := e.textObject(textPos{e.row, e.col}, count, big, around)
	if !ok {
		// The scan ran out of text: the cursor stops where it stopped.
		e.row, e.col = stop.row, stop.col
		e.clampCursor()
		e.ensureVisible()
		return
	}
	// A delete of a span that covers whole lines removes them line by line;
	// yank and change keep exactly the characters, as vi does.
	if op == 'd' && !span.whole && span.to.row > span.from.row && e.atIndent(span.from) && e.atLineEnd(span.to) {
		span.whole = true
	}
	if op == '!' {
		last := span.to.row
		if !span.whole && span.to.col == 0 && last > span.from.row {
			last--
		}
		e.openShellFilter(span.from.row, last)
		return
	}
	if span.whole {
		if op != 'y' {
			e.saveUndo()
		}
		e.applyLineOp(op, span.from.row, span.to.row, key, count)
	} else if span.from != span.to {
		if op != 'y' {
			e.saveUndo()
		}
		e.applyCharOpText(op, span.from.row, span.from.col, span.to.row, span.to.col, key, count,
			e.extractText(span.from.row, span.from.col, span.to.row, span.to.col))
	}
	e.goalValid, e.goalEnd = false, false
	e.clampCursor()
	e.ensureVisible()
}

// atIndent reports whether p sits in the leading white space of its line.
func (e *Editor) atIndent(p textPos) bool {
	return p.col <= e.firstNonBlank(p.row)
}

// atLineEnd reports whether only white space follows p on its line.
func (e *Editor) atLineEnd(p textPos) bool {
	runes := e.lineRunes(p.row)
	for col := p.col; col < len(runes); col++ {
		if !unicode.IsSpace(runes[col]) {
			return false
		}
	}
	return true
}

// --- Visual dispatch -------------------------------------------------------

// visualObject applies a word object to the active Visual selection. A
// repeated press extends the selection instead of replacing it, so "viwiw"
// covers two words while the anchor stays where the selection started.
func (e *Editor) visualObject(key string, count int) {
	big, around := key[1] == 'W', key[0] == 'a'
	span, back, ok := e.visualObjectSpan(count, big, around)
	if !ok {
		return
	}
	// A Visual selection is a character range: an object that stopped at an
	// empty line ends on the line before it and leaves whole lines to the
	// operator.
	span.whole = false
	e.visObj, e.visBack, e.visObjValid = span, back, true
	if e.visual == visualLine {
		e.visual = visualChar
	}
	end := e.lastIncluded(span.from, span.to)
	e.row, e.col = end.row, end.col
	e.clampCursor()
	e.ensureVisible()
}

// visualObjectSpan computes the new selection and the direction it grows in.
func (e *Editor) visualObjectSpan(count int, big, around bool) (textSpan, bool, bool) {
	anchor := textPos{e.visRow, e.visCol}
	cursor := textPos{e.row, e.col}
	if e.visObjValid {
		if e.visBack {
			span, ok := e.growBackward(e.visObj, count, big, around)
			return span, true, ok
		}
		span, ok := e.growForward(e.visObj, count, big, around)
		return span, false, ok
	}
	if posLess(cursor, anchor) {
		// The selection was started to the right of the cursor, so the
		// object grows towards the anchor instead of away from it.
		back, ok := e.backwardObject(cursor, count, big, around)
		if !ok {
			return textSpan{}, false, false
		}
		return textSpan{from: back.from, to: e.after(anchor)}, true, true
	}
	span, _, ok := e.textObject(cursor, count, big, around)
	if !ok {
		return textSpan{}, false, false
	}
	if posLess(anchor, span.from) {
		span.from = anchor
	}
	return span, false, true
}

// visualHalfOpen is the Visual selection as a half-open range.
func (e *Editor) visualHalfOpen() (fromR, fromC, toR, toC int, linewise bool) {
	if e.visObjValid && e.visual != visualLine {
		return e.visObj.from.row, e.visObj.from.col, e.visObj.to.row, e.visObj.to.col, false
	}
	fromR, fromC, toR, toC = e.visualBounds()
	if e.visual == visualLine {
		return fromR, 0, toR, 0, true
	}
	toC = min(toC+1, len(e.lineRunes(toR)))
	return fromR, fromC, toR, toC, false
}

// visualObjectBounds maps the stored object span back to the inclusive cursor
// bounds the rest of the Visual code works with.
func (e *Editor) visualObjectBounds() (fromRow, fromCol, toRow, toCol int) {
	from, to := e.visObj.from, e.visObj.to
	if to.col == 0 && to.row > from.row {
		to.row--
		to.col = len(e.lineRunes(to.row))
	}
	if to.col > 0 {
		to.col--
	}
	return from.row, from.col, to.row, to.col
}

// lastIncluded is the position of the last character the half-open span
// [from, to) covers.
func (e *Editor) lastIncluded(from, to textPos) textPos {
	if to.col == 0 && to.row > from.row {
		to.row--
		to.col = len(e.lineRunes(to.row))
	}
	if to.col > 0 {
		to.col--
	}
	return to
}

// after is the position just past the character at p.
func (e *Editor) after(p textPos) textPos {
	if p.col < len(e.lineRunes(p.row)) {
		return textPos{p.row, p.col + 1}
	}
	return p
}

// posLess orders positions the way the buffer is read.
func posLess(a, b textPos) bool {
	return a.row < b.row || (a.row == b.row && a.col < b.col)
}
