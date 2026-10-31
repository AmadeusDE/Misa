package vi

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func (e *Editor) startSearch(key string) {
	e.searchInputDir = 1
	if key == "?" {
		e.searchInputDir = -1
	}
	e.mode = ModeSearch
	e.cmdBuf = ""
}

func (e *Editor) cancelSearch() {
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
		} else {
			e.cancelSearch()
		}
	default:
		if len([]rune(key)) == 1 {
			e.cmdBuf += key
		}
	}
	return nil
}

// dropLastRune removes the last character, not the last byte.
func dropLastRune(s string) string {
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}
