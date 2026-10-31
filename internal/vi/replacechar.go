package vi

import tea "charm.land/bubbletea/v2"

func (e *Editor) handleReplaceChar(msg tea.KeyPressMsg) tea.Cmd {
	key := typedKey(msg)
	count := max(1, e.replaceCount)
	e.replaceCount = 0
	if key != "esc" && key != "ctrl+[" && key != "ctrl+c" {
		runes := []rune(key)
		lr := e.lineRunes(e.row)
		if len(runes) == 1 && count <= len(lr)-e.col {
			e.saveUndo()
			for i := 0; i < count; i++ {
				lr[e.col+i] = runes[0]
			}
			e.setLine(e.row, string(lr))
			ch := runes[0]
			e.lastRepeat = func(ed *Editor) {
				r := ed.lineRunes(ed.row)
				if count > len(r)-ed.col {
					return
				}
				ed.saveUndo()
				for i := 0; i < count; i++ {
					r[ed.col+i] = ch
				}
				ed.setLine(ed.row, string(r))
				ed.col += count - 1
			}
			e.col += count - 1
		}
	}
	e.mode = ModeNormal
	return nil
}
