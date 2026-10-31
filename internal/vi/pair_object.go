package vi

// Paired text objects: i( a( ib ab, i[ a[, i{ a{ iB aB and i< a<.
//
// A pair is found by counting delimiters of one kind only, so brackets of
// other kinds, quotes and comments are plain text. This keeps the scan free of
// any language rules. The cursor is inside a pair when an unmatched opening
// delimiter lies before it and its match lies after it; a cursor on either
// delimiter belongs to that pair. A count selects the n-th enclosing pair.
//
// The inner object excludes the delimiters. When the opening delimiter ends
// its line the object starts on the next line, and when the closing delimiter
// has only white space in front of it the object ends with the line before.
// An object that has both properties is made of whole lines.

// pairAt is an opening delimiter and the closing delimiter that matches it.
type pairAt struct{ open, close textPos }

// pairSpans is an object of one pair in the two forms the callers need. The
// operator form is exact. The Visual form differs in one case: an inner object
// that ends before a closing delimiter on its own line stops at the start of
// that line, so a Visual selection takes the line break as Vim does.
type pairSpans struct {
	span     textSpan
	visualTo textPos
}

// runeAt reports the character at p, false for a line break or past the end.
func (e *Editor) runeAt(p textPos) (rune, bool) {
	if p.row < 0 || p.row >= len(e.lines) {
		return 0, false
	}
	runes := e.lineRunes(p.row)
	if p.col < 0 || p.col >= len(runes) {
		return 0, false
	}
	return runes[p.col], true
}

// unmatchedOpen scans backwards from before p for an opening delimiter that
// no closing delimiter between it and p has used up.
func (e *Editor) unmatchedOpen(p textPos, d delimObject) (textPos, bool) {
	depth := 0
	for row := p.row; row >= 0; row-- {
		runes := e.lineRunes(row)
		from := len(runes)
		if row == p.row {
			from = min(p.col, len(runes))
		}
		for col := from - 1; col >= 0; col-- {
			switch runes[col] {
			case d.close:
				depth++
			case d.open:
				if depth == 0 {
					return textPos{row, col}, true
				}
				depth--
			}
		}
	}
	return p, false
}

// matchingClose scans forwards from the opening delimiter at open.
func (e *Editor) matchingClose(open textPos, d delimObject) (textPos, bool) {
	depth := 0
	for row := open.row; row < len(e.lines); row++ {
		runes := e.lineRunes(row)
		from := 0
		if row == open.row {
			from = open.col + 1
		}
		for col := from; col < len(runes); col++ {
			switch runes[col] {
			case d.open:
				depth++
			case d.close:
				if depth == 0 {
					return textPos{row, col}, true
				}
				depth--
			}
		}
	}
	return open, false
}

// nthPair finds the n-th pair around at, counting outwards from the innermost.
func (e *Editor) nthPair(at textPos, d delimObject, n int) (pairAt, bool) {
	open := at
	if r, _ := e.runeAt(at); r != d.open {
		var ok bool
		if open, ok = e.unmatchedOpen(at, d); !ok {
			return pairAt{}, false
		}
	}
	for ; n > 1; n-- {
		var ok bool
		if open, ok = e.unmatchedOpen(open, d); !ok {
			return pairAt{}, false
		}
	}
	close, ok := e.matchingClose(open, d)
	return pairAt{open, close}, ok
}

// blankBefore reports whether only white space precedes p on its line.
func (e *Editor) blankBefore(p textPos) bool {
	for _, r := range e.lineRunes(p.row)[:p.col] {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

// pairObject builds the inner or around object of a pair.
func (e *Editor) pairObject(p pairAt, around bool) pairSpans {
	if around {
		to := textPos{p.close.row, p.close.col + 1}
		return pairSpans{span: textSpan{from: p.open, to: to}, visualTo: to}
	}
	start := textPos{p.open.row, p.open.col + 1}
	startsNextLine := p.close.row > p.open.row && start.col >= len(e.lineRunes(p.open.row))
	if startsNextLine {
		start = textPos{p.open.row + 1, 0}
	}
	end, visualTo := p.close, p.close
	endsPrevLine := p.close.row > p.open.row && e.blankBefore(p.close)
	empty := false
	if endsPrevLine {
		last := p.close.row - 1
		end = textPos{last, len(e.lineRunes(last))}
		visualTo = textPos{p.close.row, 0}
		if posLess(end, start) {
			// The delimiters are adjacent lines: nothing lies between them.
			end, empty = start, true
		}
	}
	if startsNextLine && endsPrevLine && !empty {
		return pairSpans{
			span:     textSpan{from: start, to: textPos{end.row, 0}, whole: true},
			visualTo: visualTo,
		}
	}
	return pairSpans{span: textSpan{from: start, to: end}, visualTo: visualTo}
}
