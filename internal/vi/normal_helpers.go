package vi

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (e *Editor) handlePendingKey(key string) tea.Cmd {
	pending := e.pendingKey
	e.pendingKey = ""
	if pending != "\"" {
		defer e.clearCommandState()
	}

	switch pending {
	case "\"":
		valid := false
		if len(key) == 1 {
			ch := key[0]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '%' || ch == ':' || ch == '"' {
				e.pendingReg = ch
				e.hasReg = true
				valid = true
			}
		}
		if !valid {
			e.clearCommandState()
		}
		return nil

	case "f", "F", "t", "T", "'", "`":
		if len([]rune(key)) == 1 {
			e.completePendingMotion(pending + key)
		}
		return nil

	case "Z":
		switch key {
		case "Z":
			if e.filename == "" {
				e.statusMsg = "No filename"
				return nil
			}
			if err := e.save(); err != nil {
				e.statusMsg = err.Error()
			} else {
				e.Quit = true
			}
		case "Q":
			e.Quit = true
		}

	case "z":
		switch key {
		case "enter", "t":
			e.topLine = e.row
		case ".", "z":
			e.topLine = e.row - e.height/2
			if e.topLine < 0 {
				e.topLine = 0
			}
		case "-", "b":
			e.topLine = e.row - e.height + 1
			if e.topLine < 0 {
				e.topLine = 0
			}
		}

	case "[":
		if key == "[" {
			count := e.count
			if count < 1 {
				count = 1
			}
			e.count = 0
			e.countGiven = false
			e.addJump()
			e.row = e.sectionBack(e.motionLimit(count))
			e.col = e.firstNonBlank(e.row)
			e.clampCursor()
			e.ensureVisible()
		}

	case "]":
		if key == "]" {
			count := e.count
			if count < 1 {
				count = 1
			}
			e.count = 0
			e.countGiven = false
			e.addJump()
			e.row = e.sectionForward(e.motionLimit(count))
			e.col = e.firstNonBlank(e.row)
			e.clampCursor()
			e.ensureVisible()
		}

	case "g":
		if e.pendingOp != 0 && key != "g" && key != "e" && key != "E" && key != "*" && key != "#" {
			e.clearCommandState()
			return nil
		}
		switch key {
		case "g", "e", "E", "*", "#":
			e.completePendingMotion("g" + key)
		case "d":
			// gd - go to local declaration (just go to first occurrence of word under cursor)
			word := e.wordUnderCursor()
			if word != "" {
				e.addJump()
				saved := e.row
				savedCol := e.col
				e.row = 0
				e.col = 0
				if !e.searchForward(word) {
					e.row = saved
					e.col = savedCol
					e.statusMsg = "Pattern not found: " + word
				}
			}
		case "~":
			// g~ operator - not implemented as standalone but acknowledge
		}

	case "q":
		if len(key) == 1 {
			ch := key[0]
			if ch >= 'a' && ch <= 'z' {
				e.recording = true
				e.recordReg = ch
				e.recordBuf = nil
				e.statusMsg = fmt.Sprintf("Recording @%c", ch)
			}
		}
		return nil

	case "@":
		if len(key) == 1 {
			ch := key[0]
			if ch == '@' {
				ch = e.lastMacro
			}
			if ch >= 'a' && ch <= 'z' {
				e.lastMacro = ch
				e.playMacro(ch)
			}
		}
		return nil

	case "m":
		if len(key) == 1 {
			ch := key[0]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
				e.marks[ch] = pos{e.row, e.col}
				e.statusMsg = fmt.Sprintf("Mark '%c' set", ch)
			}
		}
		return nil

	}

	return nil
}

func (e *Editor) wordUnderCursor() string {
	runes := e.lineRunes(e.row)
	if e.col >= len(runes) {
		return ""
	}
	if !isWordChar(runes[e.col]) {
		return ""
	}
	start := e.col
	for start > 0 && isWordChar(runes[start-1]) {
		start--
	}
	end := e.col
	for end < len(runes)-1 && isWordChar(runes[end+1]) {
		end++
	}
	return string(runes[start : end+1])
}

func (e *Editor) playMacro(reg byte) {
	r, ok := e.regs[reg]
	if !ok || (len(r.keys) == 0 && len(r.text) == 0) {
		e.statusMsg = fmt.Sprintf("Register '%c' is empty", reg)
		return
	}

	e.macroDepth++
	if e.macroDepth > 100 {
		e.statusMsg = "Macro recursion limit"
		e.macroDepth--
		return
	}
	defer func() { e.macroDepth-- }()

	keys := r.keys
	if len(keys) == 0 && len(r.text) > 0 {
		text := strings.Join(r.text, "\n")
		for _, ch := range text {
			keys = append(keys, string(ch))
		}
	}

	for _, k := range keys {
		if e.Quit {
			break
		}
		e.processKey(k)
	}
}

func (e *Editor) processKey(key string) {
	defer e.finishEditEvent(e.row)
	e.dispatchInput(inputEvent{kind: inputKey, key: keyFromName(key)})
}

// --- Put ---

func (e *Editor) doPut(before bool) {
	if e.regLine {
		if before {
			for i := len(e.register) - 1; i >= 0; i-- {
				e.insertLineBefore(e.row, e.register[i])
			}
			e.col = e.firstNonBlank(e.row)
		} else {
			for i, line := range e.register {
				e.insertLineAfter(e.row+i, line)
			}
			e.row++
			e.col = e.firstNonBlank(e.row)
		}
	} else {
		text := strings.Join(e.register, "\n")
		runes := e.lineRunes(e.row)
		pos := e.col
		if !before {
			pos = e.col + 1
			if pos > len(runes) {
				pos = len(runes)
			}
		}
		if strings.Contains(text, "\n") {
			e.noteEdit(e.row, pos)
			parts := strings.Split(text, "\n")
			lineRunes := e.lineRunes(e.row)
			before := string(lineRunes[:pos])
			after := string(lineRunes[pos:])
			e.setLine(e.row, before+parts[0])
			for i := 1; i < len(parts)-1; i++ {
				e.insertLineAfter(e.row+i-1, parts[i])
			}
			lastIdx := e.row + len(parts) - 1
			e.insertLineAfter(lastIdx-1, parts[len(parts)-1]+after)
			e.row = lastIdx
			e.col = len([]rune(parts[len(parts)-1])) - 1
			if e.col < 0 {
				e.col = 0
			}
		} else {
			for _, ch := range text {
				e.insertCharAt(e.row, pos, ch)
				pos++
			}
			e.col = pos - 1
			if e.col < 0 {
				e.col = 0
			}
		}
	}
}

func (e *Editor) clampTop() {
	maxTop := len(e.lines) - e.height
	if maxTop < 0 {
		maxTop = 0
	}
	if e.topLine > maxTop {
		e.topLine = maxTop
	}
	if e.topLine < 0 {
		e.topLine = 0
	}
}
