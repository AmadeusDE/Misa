package vi

import "sort"

// Structural motions (H03): paragraphs, sentences, sections, unmatched
// brackets, counted screen lines and the column motion. Original code; the
// behaviour was pinned by black-box replays (see docs/plans/vi-compat.md).

func posBefore(a, b pos) bool { return a.row < b.row || (a.row == b.row && a.col < b.col) }

// isStructuralKey marks the motions whose operator ranges follow the full
// exclusive-to-linewise rules: an exclusive range ending in column 0 stops at
// the previous line's end (linewise from a blank prefix), and a multi-line d
// that starts after only blanks and ends before only blanks is linewise.
// Other motions keep their F03 ranges.
func isStructuralKey(key string) bool {
	switch key {
	case "{", "}", "(", ")", "[[", "]]", "[]", "][", "[(", "[{", "])", "]}":
		return true
	}
	return false
}

// blanksBefore reports whether only spaces and tabs precede col on row.
func (e *Editor) blanksBefore(row, col int) bool {
	runes := e.lineRunes(row)
	for i := 0; i < col && i < len(runes); i++ {
		if runes[i] != ' ' && runes[i] != '\t' {
			return false
		}
	}
	return true
}

// blanksFrom reports whether only spaces and tabs remain from col on row.
func (e *Editor) blanksFrom(row, col int) bool {
	runes := e.lineRunes(row)
	for i := max(col, 0); i < len(runes); i++ {
		if runes[i] != ' ' && runes[i] != '\t' {
			return false
		}
	}
	return true
}

// bufferEnd is the last character of the buffer. Reaching it by a forward
// motion makes that motion inclusive, unless the last line is empty.
func (e *Editor) bufferEnd() (motionResult, bool) {
	row := len(e.lines) - 1
	n := len(e.lineRunes(row))
	if n == 0 {
		return motionResult{row, 0, rangeExclusive}, false
	}
	return motionResult{row, n - 1, rangeInclusive}, true
}

func (e *Editor) isParagraphBoundary(row int) bool {
	l := e.lines[row]
	return l == "" || l[0] == '\f'
}

// paragraphMotion implements { and }. A count larger than the number of
// paragraphs left fails; the last step may stop at the buffer edge.
func (e *Editor) paragraphMotion(count int, forward bool) (motionResult, bool) {
	step := 1
	if !forward {
		step = -1
	}
	row := e.row
	for ; count > 0; count-- {
		skipped := false
		for first := true; ; first = false {
			if e.lines[row] != "" {
				skipped = true
			}
			if !first && skipped && e.isParagraphBoundary(row) {
				break
			}
			next := row + step
			if next < 0 || next >= len(e.lines) {
				if count > 1 {
					return motionResult{}, false
				}
				break
			}
			row = next
		}
	}
	if forward && row == len(e.lines)-1 {
		if m, ok := e.bufferEnd(); ok {
			return m, true
		}
	}
	return motionResult{row, 0, rangeExclusive}, true
}

func isSentenceEnd(r rune) bool { return r == '.' || r == '!' || r == '?' }
func isSentenceCloser(r rune) bool {
	return r == ')' || r == ']' || r == '"' || r == '\''
}

// sentenceStop is a place sentence motions can stop. A paragraph's first
// sentence is not reached by ) from the blanks in front of it.
type sentenceStop struct {
	pos
	paragraphStart bool
	blanksFrom     pos // where the blanks in front of a paragraph's first sentence begin
}

// sentenceStops lists where sentence motions can stop, in buffer order. A run
// of empty lines is one boundary: its first line when moving forward, its last
// when moving backward. The ends of the buffer add one more: text that ends
// after a sentence or an empty line stops at the very end, and a buffer that
// starts with empty lines stops at its first line.
func (e *Editor) sentenceStops(forward bool) []sentenceStop {
	var stops []sentenceStop
	var paragraph pos
	add := func(p pos, paragraphStart bool) {
		if len(stops) == 0 || stops[len(stops)-1].pos != p {
			stops = append(stops, sentenceStop{p, paragraphStart, paragraph})
		}
	}
	needStart, afterBlank := false, false
	for row := 0; row < len(e.lines); row++ {
		if e.lines[row] == "" {
			first := row
			for row+1 < len(e.lines) && e.lines[row+1] == "" {
				row++
			}
			if forward {
				add(pos{first, 0}, false)
			} else {
				if first == 0 {
					add(pos{0, 0}, false)
				}
				add(pos{row, 0}, false)
			}
			needStart, afterBlank = true, true
			paragraph = pos{row + 1, 0}
			continue
		}
		runes := e.lineRunes(row)
		for i := 0; i < len(runes); i++ {
			c := runes[i]
			if needStart {
				if c == ' ' || c == '\t' {
					continue
				}
				add(pos{row, i}, afterBlank)
				needStart, afterBlank = false, false
			}
			if !isSentenceEnd(c) {
				continue
			}
			j := i + 1
			for j < len(runes) && isSentenceCloser(runes[j]) {
				j++
			}
			if j == len(runes) || runes[j] == ' ' || runes[j] == '\t' {
				needStart, afterBlank = true, false
				i = j - 1
			}
		}
	}
	if last := len(e.lines) - 1; forward && needStart {
		add(pos{last, len(e.lineRunes(last))}, afterBlank)
	}
	return stops
}

// sentenceMotion implements ( and ). Like paragraphs, only the last counted
// step may run into the buffer edge.
func (e *Editor) sentenceMotion(count int, forward bool) (motionResult, bool) {
	stops := e.sentenceStops(forward)
	cur := pos{e.row, e.col}
	for ; count > 0; count-- {
		if forward {
			i := sort.Search(len(stops), func(i int) bool { return posBefore(cur, stops[i].pos) })
			if i < len(stops) && stops[i].paragraphStart && !posBefore(cur, stops[i].blanksFrom) {
				i++
			}
			if i < len(stops) {
				cur = stops[i].pos
				continue
			}
			if count > 1 {
				return motionResult{}, false
			}
			m, _ := e.bufferEnd()
			return m, true
		}
		i := sort.Search(len(stops), func(i int) bool { return !posBefore(stops[i].pos, cur) })
		if i > 0 {
			cur = stops[i-1].pos
			continue
		}
		if count > 1 {
			return motionResult{}, false
		}
		cur = pos{0, 0}
	}
	return motionResult{cur.row, cur.col, rangeExclusive}, true
}

// sectionMotion implements [[ ]] [] ][. A section starts at a form feed or at
// the brace in column 0; nroff macros are not recognised. After an operator,
// ]] also stops at a closing brace in column 0 and takes that line with it.
// Like }, a forward ]] that ends on the last line takes its last character,
// while ][ ends at the start of the last line.
func (e *Editor) sectionMotion(forward bool, brace byte, count int) (motionResult, bool) {
	step := 1
	if !forward {
		step = -1
	}
	closer := e.inOperator && forward && brace == '{'
	row := e.row
	for ; count > 0; count-- {
		for {
			row += step
			if row < 0 || row >= len(e.lines) {
				if count > 1 {
					return motionResult{}, false
				}
				row -= step
				break
			}
			l := e.lines[row]
			if l != "" && (l[0] == '\f' || l[0] == brace || (closer && l[0] == '}')) {
				break
			}
		}
	}
	last := len(e.lines) - 1
	if !e.inOperator {
		return motionResult{row, e.firstNonBlank(row), rangeExclusive}, true
	}
	if closer && e.lines[row] != "" && e.lines[row][0] == '}' {
		row++
	}
	if row > last {
		return motionResult{last, 0, rangeExclusive}, true
	}
	if forward && brace == '{' && row == last {
		if m, ok := e.bufferEnd(); ok {
			return m, true
		}
	}
	return motionResult{row, 0, rangeExclusive}, true
}

// unmatchedBracket implements [( [{ ]) ]}: the count-th enclosing delimiter.
// A count larger than the nesting depth goes to the outermost one.
func (e *Editor) unmatchedBracket(open, close rune, forward bool, count int) (motionResult, bool) {
	same, other := open, close
	step := -1
	if forward {
		same, other = close, open
		step = 1
	}
	row, col := e.row, e.col
	found := false
	var at pos
	depth := 0
	for count > 0 {
		col += step
		for col < 0 || col >= len(e.lineRunes(row)) {
			row += step
			if row < 0 || row >= len(e.lines) {
				if !found {
					return motionResult{}, false
				}
				return motionResult{at.row, at.col, rangeExclusive}, true
			}
			col = 0
			if !forward {
				col = len(e.lineRunes(row)) - 1
			}
		}
		switch e.lineRunes(row)[col] {
		case other:
			depth++
		case same:
			if depth > 0 {
				depth--
				continue
			}
			found, at = true, pos{row, col}
			count--
		}
	}
	return motionResult{at.row, at.col, rangeExclusive}, true
}

// bracketPrefixMotion resolves the second key of [ and ].
func (e *Editor) bracketPrefixMotion(forward bool, second rune, count int) (motionResult, bool) {
	switch {
	case forward && second == ']':
		return e.sectionMotion(true, '{', count)
	case forward && second == '[':
		return e.sectionMotion(true, '}', count)
	case !forward && second == '[':
		return e.sectionMotion(false, '{', count)
	case !forward && second == ']':
		return e.sectionMotion(false, '}', count)
	case !forward && second == '(', forward && second == ')':
		return e.unmatchedBracket('(', ')', forward, count)
	case !forward && second == '{', forward && second == '}':
		return e.unmatchedBracket('{', '}', forward, count)
	}
	return motionResult{}, false
}

// screenLineMotion implements counted H, M and L. A cursor motion stays inside
// the visible window; after an operator the count may reach past it.
func (e *Editor) screenLineMotion(key string, count int) motionResult {
	top := min(max(e.topLine, 0), len(e.lines)-1)
	bottom := min(top+e.height-1, len(e.lines)-1)
	var row int
	switch key {
	case "H":
		row = min(top+count-1, bottom)
		if e.inOperator {
			row = min(top+count-1, len(e.lines)-1)
		}
	case "L":
		row = max(bottom-count+1, top)
		if e.inOperator {
			row = max(bottom-count+1, 0)
		}
	default:
		row = top + (bottom-top)/2
	}
	return motionResult{row, e.firstNonBlank(row), rangeLinewise}
}

// columnMotion implements | : the character covering display column count.
func (e *Editor) columnMotion(count int) motionResult {
	runes := e.lineRunes(e.row)
	col := e.rowLayout(e.row).runeAt(count - 1)
	return motionResult{e.row, min(col, max(len(runes)-1, 0)), rangeExclusive}
}
