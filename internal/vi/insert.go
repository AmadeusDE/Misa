package vi

import tea "charm.land/bubbletea/v2"

// insertKeys preserves editing actions, not just their resulting text.
func insertKeys(text string) []string {
	var keys []string
	for _, ch := range text {
		switch ch {
		case '\b':
			keys = append(keys, "backspace")
		case '\n':
			keys = append(keys, "enter")
		case '\t':
			keys = append(keys, "tab")
		default:
			keys = append(keys, string(ch))
		}
	}
	return keys
}

// replayInsertInput applies the recorded stream at the current insert/replace
// position. Entry and Escape are owned by the semantic repeat dispatcher.
func (e *Editor) replayInsertInput(keys []string) {
	for _, key := range keys {
		switch e.mode {
		case ModeReplace:
			e.handleReplace(keyFromName(key))
		case ModeReplaceChar:
			e.handleReplaceChar(keyFromName(key))
		case ModeInsert:
			e.handleInsert(keyFromName(key))
		}
	}
}

func (e *Editor) handleInsert(msg tea.KeyPressMsg) tea.Cmd {
	key := typedKey(msg)
	switch key {
	case "esc", "ctrl+[", "ctrl+c":
		keys := insertKeys(e.insertBuf.String())
		count := min(e.insertCount, maxContentRepeats)
		if key != "ctrl+c" {
			for i := 1; i < count; i++ {
				if e.insertEntry == "o" {
					e.insertLineAfter(e.row, "")
					e.row++
					e.col = 0
				} else if e.insertEntry == "O" {
					e.insertLineBefore(e.row, "")
					e.col = 0
				}
				e.replayInsertInput(keys)
			}
			if e.editDepth > 0 && !sameLines(e.editBefore.lines, e.lines) {
				e.finishRepeatInsert(keys)
			}
		}
		e.repeatPending = semanticChange{}
		e.insertBuf.Reset()
		e.insertCount = 0
		e.mode = ModeNormal
		if e.col > 0 {
			e.col--
		}
		e.clampCursor()
	case "backspace", "ctrl+h":
		e.insertBuf.WriteRune('\b')
		if e.col > 0 {
			e.deleteCharAt(e.row, e.col-1)
			e.col--
		} else if e.row > 0 {
			prevLen := len(e.lineRunes(e.row - 1))
			e.noteEdit(e.row-1, prevLen)
			e.lines[e.row-1] += e.lines[e.row]
			e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
			e.row--
			e.col = prevLen
			e.modified = true
		}
	case "enter":
		e.expandAbbrev()
		e.splitLine(e.row, e.col)
		e.row++
		e.col = 0
		e.insertBuf.WriteRune('\n')
	case "tab":
		e.insertCharAt(e.row, e.col, '\t')
		e.col++
		e.insertBuf.WriteRune('\t')
	default:
		runes := []rune(key)
		if len(runes) == 1 {
			if runes[0] == ' ' || runes[0] == '.' || runes[0] == ',' || runes[0] == ';' || runes[0] == ':' || runes[0] == '!' || runes[0] == '?' {
				e.expandAbbrev()
			}
			e.insertCharAt(e.row, e.col, runes[0])
			e.col++
			e.insertBuf.WriteRune(runes[0])
		}
	}
	return nil
}

func (e *Editor) expandAbbrev() {
	if len(e.abbrevs) == 0 {
		return
	}
	runes := e.lineRunes(e.row)
	if e.col == 0 {
		return
	}
	end := e.col
	start := end - 1
	for start > 0 && isWordChar(runes[start-1]) {
		start--
	}
	word := string(runes[start:end])
	if repl, ok := e.abbrevs[word]; ok {
		nr := make([]rune, 0, len(runes)-len(word)+len([]rune(repl)))
		nr = append(nr, runes[:start]...)
		nr = append(nr, []rune(repl)...)
		nr = append(nr, runes[end:]...)
		e.setLine(e.row, string(nr))
		e.col = start + len([]rune(repl))
	}
}
