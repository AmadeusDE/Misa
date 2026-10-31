package vi

import (
	"fmt"
	"slices"
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

	// Multi-key motions retain the operator, count, and register until resolved.
	if isMotionPrefix(key) {
		e.count, e.countGiven = count, cg
		e.pendingKey = key
		return nil
	}
	if key == "/" || key == "?" {
		e.count, e.countGiven = count, cg
		e.startSearch(key)
		return nil
	}
	if e.pendingOp != 0 {
		count, cg = e.commandMotionCount(count, cg)
		e.executeOperatorMotion(e.pendingOp, key, count, cg)
		e.clearCommandState()
		return nil
	}

	// Any completed command consumes an explicit register. Prefixes retain it.
	defer func() {
		if e.pendingKey == "" && e.pendingOp == 0 {
			e.hasReg, e.pendingReg = false, 0
		}
	}()

	// --- Standalone motions ---
	if m, ok := e.resolveMotion(key, count, cg); ok {
		if motionAddsJump(key) {
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

	change := semanticChange{command: key, count: count, countGiven: cg, register: e.repeatRegister()}
	var before []string
	switch key {
	case "x", "X", "D", "p", "P", "J", "~":
		before = slices.Clone(e.lines)
	}
	defer func() {
		switch key {
		case "x", "X", "D", "p", "P", "J", "~":
			if !e.replayingChange && !sameLines(before, e.lines) {
				e.repeatChange = change
			}
		case "i", "a", "I", "A", "o", "O", "s", "C", "S", "R", "r":
			if !e.replayingChange {
				e.repeatPending = change
			}
		}
	}()
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
			e.noteEdit(e.row, e.col)
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
		e.insertCount = count
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
		}
	case ".":
		e.repeatLastChange(count, cg)
		return nil

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

	// Mode entry
	case ":":
		e.mode = ModeCommand
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
