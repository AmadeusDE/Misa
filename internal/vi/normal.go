package vi

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

func (e *Editor) handleNormal(msg tea.KeyPressMsg) tea.Cmd {
	key := typedKey(msg)

	if key != "j" && key != "k" && key != "up" && key != "down" &&
		!(len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || e.countGiven)) {
		e.goalValid = false
		e.goalEnd = false
	}

	if e.recording {
		if key == "q" && e.pendingOp == 0 && e.pendingKey == "" {
			e.recording = false
			e.setReg(e.recordReg, nil, false)
			if e.regs[e.recordReg] == nil {
				e.regs[e.recordReg] = &regContent{}
			}
			e.regs[e.recordReg].keys = e.recordBuf
			e.statusMsg = fmt.Sprintf("Recorded @%c", e.recordReg)
			return nil
		}
		e.recordBuf = append(e.recordBuf, key)
	}

	if key == "esc" || key == "ctrl+[" || key == "ctrl+c" {
		e.clearCommandState()
		return nil
	}
	if e.pendingKey != "" {
		return e.handlePendingKey(key)
	}

	// Count prefix
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		e.count = countDigit(e.count, int(key[0]-'0'))
		e.countGiven = true
		return nil
	}
	if len(key) == 1 && key[0] == '0' && e.countGiven {
		e.count = countDigit(e.count, 0)
		return nil
	}

	count := e.count
	if count < 1 {
		count = 1
	}
	cg := e.countGiven

	// Register prefix
	if key == "\"" {
		e.pendingKey = "\""
		return nil
	}

	// Operators
	if key == "d" || key == "c" || key == "y" || key == ">" || key == "<" {
		op := key[0]
		if e.pendingOp == op {
			c := count
			if e.opCountGiven && cg {
				c = countProduct(e.opCount, count)
			} else if e.opCountGiven {
				c = e.opCount
			}
			e.executeLineOp(op, c)
			e.hasReg, e.pendingReg = false, 0
			e.pendingOp = 0
			e.opCount = 0
			e.opCountGiven = false
			e.count = 0
			e.countGiven = false
			return nil
		}
		if e.pendingOp != 0 {
			e.clearCommandState()
			return nil
		}
		e.pendingOp = op
		e.opCount = count
		e.opCountGiven = cg
		e.count = 0
		e.countGiven = false
		return nil
	}

	e.count = 0
	e.countGiven = false

	// Operator pending: next key must be a motion
	if e.pendingOp != 0 {
		if key == "g" {
			e.count, e.countGiven = count, cg
			e.pendingKey = "g"
			return nil
		}
		c := count
		if e.opCountGiven {
			c = countProduct(e.opCount, count)
		}
		op := e.pendingOp
		hadOpCount := e.opCountGiven
		e.pendingOp = 0
		e.opCount = 0
		e.opCountGiven = false
		e.executeOperatorMotion(op, key, c, cg || hadOpCount)
		e.hasReg, e.pendingReg = false, 0
		return nil
	}

	// Any completed command consumes an explicit register. Prefixes retain it.
	defer func() {
		if e.pendingKey == "" && e.pendingOp == 0 {
			e.hasReg, e.pendingReg = false, 0
		}
	}()

	// --- Standalone motions ---
	if m, ok := e.motionDest(key, count, cg); ok {
		if key == "G" || key == "gg" {
			e.addJump()
		}
		if key == "j" || key == "k" || key == "up" || key == "down" {
			if !e.goalValid {
				e.goalCol, e.goalValid = e.col, true
			}
			m.col = e.goalCol
			if e.goalEnd {
				m.col = len(e.lineRunes(m.row)) - 1
			}
		}
		e.row = m.row
		e.col = m.col
		e.clampCursor()
		if key == "$" || key == "end" {
			e.goalValid, e.goalEnd = true, true
		}
		e.ensureVisible()
		return nil
	}

	switch key {
	// Scroll
	case "ctrl+f":
		e.row += e.height
		e.topLine += e.height
		if e.row >= len(e.lines) {
			e.row = len(e.lines) - 1
		}
		e.clampTop()
	case "ctrl+b":
		e.row -= e.height
		e.topLine -= e.height
		if e.row < 0 {
			e.row = 0
		}
		if e.topLine < 0 {
			e.topLine = 0
		}
	case "ctrl+d":
		half := e.height / 2
		e.row += half
		e.topLine += half
		if e.row >= len(e.lines) {
			e.row = len(e.lines) - 1
		}
		e.clampTop()
	case "ctrl+u":
		half := e.height / 2
		e.row -= half
		e.topLine -= half
		if e.row < 0 {
			e.row = 0
		}
		if e.topLine < 0 {
			e.topLine = 0
		}
	case "ctrl+e":
		count = min(count, len(e.lines))
		for i := 0; i < count; i++ {
			if e.topLine < len(e.lines)-1 {
				e.topLine++
			}
		}
		if e.row < e.topLine {
			e.row = e.topLine
		}
	case "ctrl+y":
		count = min(count, len(e.lines))
		for i := 0; i < count; i++ {
			if e.topLine > 0 {
				e.topLine--
			}
		}
		if e.row >= e.topLine+e.height {
			e.row = e.topLine + e.height - 1
		}
	case "ctrl+l":
		// Redraw - no-op in TUI, but acknowledge the key
		e.ensureVisible()

	// Actions
	case "x", "X":
		backwards := key == "X"
		e.deleteCountedChars(count, backwards, true)
		e.lastRepeat = func(ed *Editor) { ed.deleteCountedChars(count, backwards, false); ed.clampCursor() }
	case "D":
		e.deleteToCountedEnd(count, false)
	case "C":
		e.deleteToCountedEnd(count, true)
		return nil
	case "S":
		e.executeLineOp('c', count)
		e.modified = true
		return nil
	case "s":
		e.saveUndo()
		r := e.lineRunes(e.row)
		end := e.col + min(count, len(r)-e.col)
		if end > e.col {
			e.deleteToReg([]string{string(r[e.col:end])}, false)
			e.setLine(e.row, string(r[:e.col])+string(r[end:]))
		}
		e.mode = ModeInsert
		e.insertEntry = "s"
		e.insertBuf.Reset()
		e.insertCount = 0
		return nil
	case "r":
		e.replaceCount = count
		e.mode = ModeReplaceChar
		return nil
	case "R":
		e.saveUndo()
		e.mode = ModeReplace
		e.insertBuf.Reset()
		return nil
	case "p":
		if e.hasReg {
			text, isLine, ok := e.getReg(e.pendingReg)
			if ok {
				e.register = text
				e.regLine = isLine
			}
			e.hasReg = false
			e.pendingReg = 0
		}
		if len(e.register) > 0 {
			e.saveUndo()
			e.putCounted(false, count)
			e.lastRepeat = func(ed *Editor) {
				if len(ed.register) > 0 {
					ed.saveUndo()
					ed.doPut(false)
				}
			}
		}
	case "P":
		if e.hasReg {
			text, isLine, ok := e.getReg(e.pendingReg)
			if ok {
				e.register = text
				e.regLine = isLine
			}
			e.hasReg = false
			e.pendingReg = 0
		}
		if len(e.register) > 0 {
			e.saveUndo()
			e.putCounted(true, count)
			e.lastRepeat = func(ed *Editor) {
				if len(ed.register) > 0 {
					ed.saveUndo()
					ed.doPut(true)
				}
			}
		}
	case "u":
		count = min(count, len(e.undoStack))
		for i := 0; i < count; i++ {
			e.undo()
		}
	case "U":
		e.undoLine()
	case "ctrl+r":
		count = min(count, len(e.redoStack))
		for i := 0; i < count; i++ {
			e.redo()
		}
	case "J":
		joins := min(max(2, count)-1, len(e.lines)-1-e.row)
		if joins > 0 {
			e.saveUndo()
			e.joinCounted(joins)
			e.lastRepeat = func(ed *Editor) {
				n := min(joins, len(ed.lines)-1-ed.row)
				if n > 0 {
					ed.saveUndo()
					ed.joinCounted(n)
				}
			}
		}
	case "~":
		count = min(count, len(e.lineRunes(e.row))-e.col)
		r := e.lineRunes(e.row)
		if e.col < len(r) {
			e.saveUndo()
			for i := 0; i < count; i++ {
				r = e.lineRunes(e.row)
				if e.col >= len(r) {
					break
				}
				ch := r[e.col]
				if unicode.IsLower(ch) {
					r[e.col] = unicode.ToUpper(ch)
				} else if unicode.IsUpper(ch) {
					r[e.col] = unicode.ToLower(ch)
				}
				e.setLine(e.row, string(r))
				if e.col < len(r)-1 {
					e.col++
				}
			}
			n := count
			e.lastRepeat = func(ed *Editor) {
				r := ed.lineRunes(ed.row)
				if ed.col >= len(r) {
					return
				}
				ed.saveUndo()
				for i := 0; i < n; i++ {
					r = ed.lineRunes(ed.row)
					if ed.col >= len(r) {
						break
					}
					ch := r[ed.col]
					if unicode.IsLower(ch) {
						r[ed.col] = unicode.ToUpper(ch)
					} else if unicode.IsUpper(ch) {
						r[ed.col] = unicode.ToLower(ch)
					}
					ed.setLine(ed.row, string(r))
					if ed.col < len(r)-1 {
						ed.col++
					}
				}
			}
		}
	case ".":
		if e.lastRepeat != nil {
			e.lastRepeat(e)
			e.clampCursor()
			e.ensureVisible()
		}
		return nil
	case "%":
		e.matchBracket()
		return nil
	case "n", "N":
		e.searchCounted(count, key == "N")

	case "ctrl+g":
		fname := e.filename
		if fname == "" {
			fname = "[No Name]"
		}
		modFlag := ""
		if e.modified {
			modFlag = "[Modified] "
		}
		pct := 0
		if len(e.lines) > 0 {
			pct = (e.row + 1) * 100 / len(e.lines)
		}
		e.statusMsg = fmt.Sprintf("\"%s\" %s%d lines --%d%%-- line %d, col %d",
			fname, modFlag, len(e.lines), pct, e.row+1, e.col+1)

	case "ctrl+o":
		e.jumpBack()
	case "ctrl+i":
		e.jumpForward()

	// Find char
	case "f", "F", "t", "T":
		e.opCount = count
		e.pendingKey = key
		return nil
	case ";":
		e.repeatFind(count, false)
	case ",":
		e.repeatFind(count, true)

	// Macros
	case "q":
		if !e.recording {
			e.pendingKey = "q"
			return nil
		}
	case "@":
		e.pendingKey = "@"
		return nil

	// Marks
	case "m":
		e.pendingKey = "m"
		return nil
	case "'", "`":
		e.pendingKey = key
		return nil

	// Insert mode entry
	case "i":
		e.saveUndo()
		e.mode = ModeInsert
		e.insertEntry = "i"
		e.insertBuf.Reset()
		e.insertCount = count
		return nil
	case "a":
		e.saveUndo()
		r := e.lineRunes(e.row)
		if e.col < len(r) {
			e.col++
		}
		e.mode = ModeInsert
		e.insertEntry = "a"
		e.insertBuf.Reset()
		e.insertCount = count
		return nil
	case "o":
		e.saveUndo()
		e.insertLineAfter(e.row, "")
		e.row++
		e.col = 0
		e.mode = ModeInsert
		e.insertEntry = "o"
		e.insertBuf.Reset()
		e.insertCount = count
		return nil
	case "A":
		e.saveUndo()
		e.col = len(e.lineRunes(e.row))
		e.mode = ModeInsert
		e.insertEntry = "A"
		e.insertBuf.Reset()
		e.insertCount = count
		return nil
	case "I":
		e.saveUndo()
		e.col = e.firstNonBlank(e.row)
		e.mode = ModeInsert
		e.insertEntry = "I"
		e.insertBuf.Reset()
		e.insertCount = count
		return nil
	case "O":
		e.saveUndo()
		e.insertLineBefore(e.row, "")
		e.col = 0
		e.mode = ModeInsert
		e.insertEntry = "O"
		e.insertBuf.Reset()
		e.insertCount = count
		return nil

	// Visual mode
	case "v":
		e.visual = visualChar
		e.visRow = e.row
		e.visCol = e.col
		return nil
	case "V":
		e.visual = visualLine
		e.visRow = e.row
		e.visCol = e.col
		return nil

	// Two-key prefixes
	case "Z":
		e.pendingKey = "Z"
		return nil
	case "z":
		e.pendingKey = "z"
		return nil
	case "[":
		e.count, e.countGiven = count, cg
		e.pendingKey = "["
		return nil
	case "]":
		e.count, e.countGiven = count, cg
		e.pendingKey = "]"
		return nil
	case "g":
		e.count, e.countGiven = count, cg
		e.pendingKey = "g"
		return nil

	// Mode entry
	case ":":
		e.mode = ModeCommand
		e.cmdBuf = ""
		return nil
	case "/":
		e.searchDir = 1
		e.mode = ModeSearch
		e.cmdBuf = ""
		return nil
	case "?":
		e.searchDir = -1
		e.mode = ModeSearch
		e.cmdBuf = ""
		return nil

	case "ctrl+c":
		e.pendingOp = 0
		e.pendingKey = ""
		e.count = 0
		e.countGiven = false
		e.hasReg = false
		e.pendingReg = 0
		if e.visual != visualNone {
			e.visual = visualNone
		}
	}

	e.clampCursor()
	e.ensureVisible()
	return nil
}

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

	case "f", "F", "t", "T":
		runes := []rune(key)
		if len(runes) == 1 {
			count := e.opCount
			if count < 1 {
				count = 1
			}
			if e.pendingOp != 0 {
				op := e.pendingOp
				e.pendingOp = 0
				startRow, startCol := e.row, e.col
				if e.findChar(runes[0], pending[0], count) {
					e.saveUndo()
					toC := e.col
					if pending == "f" || pending == "t" {
						toC++
					}
					e.row = startRow
					e.col = startCol
					fromC := e.col
					if fromC > toC {
						fromC, toC = toC, fromC
					}
					e.applyCharOp(op, e.row, fromC, e.row, toC, pending+key, count)
				}
			} else {
				e.findChar(runes[0], pending[0], count)
			}
		}
		e.opCount = 0
		e.opCountGiven = false
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
		if e.pendingOp != 0 && key != "g" {
			e.clearCommandState()
			return nil
		}
		switch key {
		case "g":
			count := 1
			cg := false
			if e.countGiven {
				count = e.count
				cg = true
			}
			e.count = 0
			e.countGiven = false

			if e.pendingOp != 0 {
				op := e.pendingOp
				if e.opCountGiven {
					count = countProduct(e.opCount, count)
					cg = true
				}
				e.pendingOp = 0
				e.opCount = 0
				e.opCountGiven = false
				e.executeOperatorMotion(op, "gg", count, cg)
				e.hasReg, e.pendingReg = false, 0
				return nil
			}

			e.addJump()
			if m, ok := e.motionDest("gg", count, cg); ok {
				e.row = m.row
				e.col = m.col
			}
			e.clampCursor()
			e.ensureVisible()
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

	case "'":
		if len(key) == 1 {
			ch := key[0]
			if p, ok := e.marks[ch]; ok {
				e.addJump()
				e.row = p.row
				e.col = e.firstNonBlank(p.row)
				e.clampCursor()
				e.ensureVisible()
			} else {
				e.statusMsg = fmt.Sprintf("Mark '%c' not set", ch)
			}
		}
		return nil

	case "`":
		if len(key) == 1 {
			ch := key[0]
			if p, ok := e.marks[ch]; ok {
				e.addJump()
				e.row = p.row
				e.col = p.col
				e.clampCursor()
				e.ensureVisible()
			} else {
				e.statusMsg = fmt.Sprintf("Mark '%c' not set", ch)
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
	msg := keyFromName(key)

	switch e.mode {
	case ModeNormal:
		e.handleNormal(msg)
	case ModeInsert:
		e.handleInsert(msg)
	case ModeReplace:
		e.handleReplace(msg)
	case ModeReplaceChar:
		e.handleReplaceChar(msg)
	case ModeCommand:
		e.handleCommand(msg)
	case ModeSearch:
		e.handleSearch(msg)
	}
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
