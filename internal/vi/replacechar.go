package vi

import tea "charm.land/bubbletea/v2"

// replaceChars performs a single r operation. A newline replaces the whole
// counted range with one line break, rather than count line breaks.
func (e *Editor) replaceChars(key string, count int) bool {
	lr := e.lineRunes(e.row)
	if count > len(lr)-e.col {
		return false
	}
	if key == "enter" {
		e.saveUndo()
		e.noteEdit(e.row, e.col)
		tail := string(lr[e.col+count:])
		e.setLine(e.row, string(lr[:e.col]))
		e.insertLineAfter(e.row, tail)
		e.row++
		e.col = 0
		return true
	}
	if key == "tab" {
		key = "\t"
	}
	chars := []rune(key)
	if len(chars) != 1 {
		return false
	}
	changed := false
	for i := 0; i < count; i++ {
		if lr[e.col+i] != chars[0] {
			changed = true
		}
		lr[e.col+i] = chars[0]
	}
	if changed {
		e.saveUndo()
		e.setLine(e.row, string(lr))
	}
	e.col += count - 1
	return changed
}

func (e *Editor) handleReplaceChar(msg tea.KeyPressMsg) tea.Cmd {
	key := typedKey(msg)
	count := max(1, e.replaceCount)
	e.replaceCount = 0
	if key != "esc" && key != "ctrl+[" && key != "ctrl+c" {
		if e.replaceChars(key, count) {
			e.finishRepeatInsert([]string{key})
		}
	}
	e.repeatPending = semanticChange{}
	e.mode = ModeNormal
	return nil
}
