package vi

import "strings"

// searchMotion resolves a wrapping literal search without moving the editor.
// Enumerating match starts also permits overlapping backward matches.
func (e *Editor) searchMotion(count int, reverse bool) (motionResult, bool) {
	if e.searchPat == "" {
		e.statusMsg = "No previous search pattern"
		return motionResult{}, false
	}
	pat := e.cleanPattern(e.searchPat)
	if pat == "" {
		e.statusMsg = "Pattern not found: " + e.searchPat
		return motionResult{}, false
	}
	var matches []pos
	for row, line := range e.lines {
		col := 0
		for offset := range line {
			suffix := line[offset:]
			match := strings.HasPrefix(suffix, pat)
			if e.shouldIgnoreCase(e.searchPat) {
				_, match = foldPrefix(suffix, pat)
			}
			if match {
				matches = append(matches, pos{row, col})
			}
			col++
		}
	}
	if len(matches) == 0 {
		e.statusMsg = "Pattern not found: " + e.searchPat
		return motionResult{}, false
	}
	if count < 1 {
		count = 1
	}
	forward := e.searchDir == 1
	if reverse {
		forward = !forward
	}
	index := 0
	if forward {
		index = len(matches)
		for i, p := range matches {
			if p.row > e.row || (p.row == e.row && p.col > e.col) {
				index = i
				break
			}
		}
		index = (index + (count-1)%len(matches)) % len(matches)
	} else {
		index = len(matches) - 1
		for i := len(matches) - 1; i >= 0; i-- {
			p := matches[i]
			if p.row < e.row || (p.row == e.row && p.col < e.col) {
				index = i
				break
			}
		}
		index = (index - (count-1)%len(matches) + len(matches)) % len(matches)
	}
	p := matches[index]
	return motionResult{p.row, p.col, rangeExclusive}, true
}

func (e *Editor) searchAt(pat string, forward bool) bool {
	oldPat, oldDir := e.searchPat, e.searchDir
	e.searchPat = pat
	if forward {
		e.searchDir = 1
	} else {
		e.searchDir = -1
	}
	m, ok := e.searchMotion(1, false)
	e.searchPat, e.searchDir = oldPat, oldDir
	if ok {
		e.row, e.col = m.row, m.col
		e.ensureVisible()
	}
	return ok
}
func (e *Editor) searchForward(pat string) bool  { return e.searchAt(pat, true) }
func (e *Editor) searchBackward(pat string) bool { return e.searchAt(pat, false) }
func (e *Editor) searchNext() bool               { return e.searchAt(e.searchPat, e.searchDir == 1) }
func (e *Editor) searchPrev() bool               { return e.searchAt(e.searchPat, e.searchDir != 1) }
func (e *Editor) searchCounted(count int, reverse bool) {
	m, ok := e.searchMotion(count, reverse)
	if ok {
		e.addJump()
		e.row, e.col = m.row, m.col
		e.ensureVisible()
	}
}
