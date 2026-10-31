package vi

import tea "charm.land/bubbletea/v2"

// One reversible replacement operation; newline steps join their split on BS.
type replaceStep struct {
	row, col         int
	old              rune
	existed, newline bool
}

func (e *Editor) handleReplace(msg tea.KeyPressMsg) tea.Cmd {
	key := typedKey(msg)
	switch key {
	case "esc", "ctrl+[", "ctrl+c":
		keys := insertKeys(e.insertBuf.String())
		if key != "ctrl+c" {
			for i := 1; i < min(e.insertCount, maxContentRepeats); i++ {
				e.replayInsertInput(keys)
			}
		}
		if key != "ctrl+c" && e.editDepth > 0 && !sameLines(e.editBefore.lines, e.lines) {
			e.finishRepeatInsert(keys)
		}
		e.repeatPending = semanticChange{}
		e.insertCount = 0
		e.insertBuf.Reset()
		e.replaceSteps = nil
		e.mode = ModeNormal
		if e.col > 0 {
			e.col--
		}
		e.clampCursor()
	case "backspace", "ctrl+h":
		e.insertBuf.WriteRune('\b')
		if len(e.replaceSteps) == 0 {
			break
		}
		step := e.replaceSteps[len(e.replaceSteps)-1]
		e.replaceSteps = e.replaceSteps[:len(e.replaceSteps)-1]
		e.row, e.col = step.row, step.col
		if step.newline {
			e.noteEdit(e.row, e.col)
			e.lines[e.row] += e.lines[e.row+1]
			e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
			e.modified = true
		} else if step.existed {
			lr := e.lineRunes(e.row)
			lr[e.col] = step.old
			e.setLine(e.row, string(lr))
		} else {
			e.deleteCharAt(e.row, e.col)
		}
	case "enter":
		e.replaceSteps = append(e.replaceSteps, replaceStep{row: e.row, col: e.col, newline: true})
		e.splitLine(e.row, e.col)
		e.row++
		e.col = 0
		e.insertBuf.WriteRune('\n')
	default:
		if key == "tab" {
			key = "\t"
		}
		runes := []rune(key)
		if len(runes) == 1 {
			lr := e.lineRunes(e.row)
			step := replaceStep{row: e.row, col: e.col}
			if e.col < len(lr) {
				step.old = lr[e.col]
				step.existed = true
				lr[e.col] = runes[0]
				e.setLine(e.row, string(lr))
			} else {
				e.insertCharAt(e.row, e.col, runes[0])
			}
			e.replaceSteps = append(e.replaceSteps, step)
			e.col++
			e.insertBuf.WriteRune(runes[0])
		}
	}
	return nil
}
