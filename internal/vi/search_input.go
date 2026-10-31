package vi

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// searchPreviewState keeps the exact view from which an interactive search began.
type searchPreviewState struct {
	row, col, topLine, leftCol     int
	pattern                        string
	matchRow, matchStart, matchEnd int
	matched                        bool
}

// previewHighlight exposes byte bounds of the selected tentative match. An
// active but unmatched preview must hide committed highlights.
func (e *Editor) previewHighlight() (pattern string, row, start, end int, active, matched bool) {
	if e.mode != ModeSearch || !e.incSearch || e.cmdBuf == "" {
		return "", 0, 0, 0, false, false
	}
	s := e.searchPreview
	return s.pattern, s.matchRow, s.matchStart, s.matchEnd, true, s.matched
}

func (e *Editor) restoreSearchOrigin() {
	s := e.searchPreview
	e.row, e.col, e.topLine, e.leftCol = s.row, s.col, s.topLine, s.leftCol
}

func (e *Editor) previewSearch() {
	e.restoreSearchOrigin()
	pat := e.cmdBuf
	if e.searchInputDir == -1 {
		pat = unescapeQuestion(pat)
	}
	e.searchPreview.pattern = pat
	e.searchPreview.matched = false
	if pat == "" {
		return
	}
	// A preview is speculative: leave the committed search, direction, and
	// diagnostics alone, including when the pattern is incomplete or missing.
	oldPat, oldDir, oldStatus := e.searchPat, e.searchDir, e.statusMsg
	e.searchPat, e.searchDir = pat, e.searchInputDir
	count, _ := e.commandMotionCount(max(1, e.count), e.countGiven)
	m, ok := e.searchMotion(count, false)
	e.searchPat, e.searchDir, e.statusMsg = oldPat, oldDir, oldStatus
	if ok {
		// Locate the same match start used by searchMotion, retaining its byte
		// extent for rendering (cursor columns are rune indices).
		re, compiled := e.compilePattern(pat)
		if compiled {
			line := e.lines[m.row]
			for from := 0; from <= len(line); {
				start, end, found, err := re.Find(line, from)
				if err != nil || !found {
					break
				}
				col := utf8.RuneCountInString(line[:start])
				if start == len(line) && col > 0 {
					col--
				}
				if col == m.col {
					e.searchPreview.matchRow, e.searchPreview.matchStart, e.searchPreview.matchEnd = m.row, start, end
					e.searchPreview.matched = true
					break
				}
				if start == len(line) {
					break
				}
				_, width := utf8.DecodeRuneInString(line[start:])
				from = start + width
			}
		}
		e.statusMsg = oldStatus
		e.row, e.col = m.row, m.col
		e.clampCursor()
		e.ensureVisible()
	}
}

func (e *Editor) startSearch(key string) {
	e.searchPreview = searchPreviewState{row: e.row, col: e.col, topLine: e.topLine, leftCol: e.leftCol}
	e.searchInputDir = 1
	if key == "?" {
		e.searchInputDir = -1
	}
	e.mode = ModeSearch
	e.cmdBuf = ""
}

func (e *Editor) cancelSearch() {
	if e.incSearch {
		e.restoreSearchOrigin()
	}
	e.mode = ModeNormal
	e.cmdBuf = ""
	e.clearCommandState()
}

func (e *Editor) handleSearch(msg tea.KeyPressMsg) tea.Cmd {
	key := typedKey(msg)
	switch key {
	case "esc", "ctrl+[", "ctrl+c":
		e.cancelSearch()
	case "enter":
		if e.incSearch {
			e.restoreSearchOrigin()
		}
		pat := e.cmdBuf
		if e.searchInputDir == -1 {
			pat = unescapeQuestion(pat)
		}
		e.cmdBuf = ""
		e.mode = ModeNormal
		if pat != "" {
			if _, ok := e.compilePattern(pat); !ok {
				e.clearCommandState()
				return nil
			}
			e.searchPat = pat
		}
		e.searchDir = e.searchInputDir
		e.searchHighlightSuppressed = false
		count, _ := e.commandMotionCount(max(1, e.count), e.countGiven)
		if m, ok := e.searchMotion(count, false); ok {
			if e.pendingOp != 0 {
				e.executeOperatorResult(e.pendingOp, "n", count, m)
			} else {
				e.addJump()
				e.row, e.col = m.row, m.col
				e.clampCursor()
				e.ensureVisible()
			}
		}
		e.clearCommandState()
	case "backspace", "ctrl+h":
		if e.cmdBuf != "" {
			e.cmdBuf = dropLastRune(e.cmdBuf)
			if e.incSearch {
				e.previewSearch()
			}
		} else {
			e.cancelSearch()
		}
	default:
		if len([]rune(key)) == 1 {
			e.cmdBuf += key
			if e.incSearch {
				e.previewSearch()
			}
		}
	}
	return nil
}

// dropLastRune removes the last character, not the last byte.
func dropLastRune(s string) string {
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}
