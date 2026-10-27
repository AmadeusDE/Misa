package vi

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

func (e *Editor) visualBounds() (fromRow, fromCol, toRow, toCol int) {
	fromRow, fromCol = e.visRow, e.visCol
	toRow, toCol = e.row, e.col
	if fromRow > toRow || (fromRow == toRow && fromCol > toCol) {
		fromRow, fromCol, toRow, toCol = toRow, toCol, fromRow, fromCol
	}
	return
}

func (e *Editor) updateVisual(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil, nil
	}

	if e.recording {
		key := typedKey(keyMsg)
		if key == "q" && e.pendingKey == "" {
			e.recording = false
			if e.regs[e.recordReg] == nil {
				e.regs[e.recordReg] = &regContent{}
			}
			e.regs[e.recordReg].keys = e.recordBuf
			e.statusMsg = fmt.Sprintf("Recorded @%c", e.recordReg)
			return nil, nil
		}
		e.recordBuf = append(e.recordBuf, key)
	}

	key := typedKey(keyMsg)

	// Count prefix in visual mode
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		e.count = e.count*10 + int(key[0]-'0')
		e.countGiven = true
		return nil, nil
	}
	if len(key) == 1 && key[0] == '0' && e.countGiven {
		e.count = e.count * 10
		return nil, nil
	}

	count := e.count
	if count < 1 {
		count = 1
	}
	cg := e.countGiven
	e.count = 0
	e.countGiven = false

	switch key {
	case "esc", "ctrl+c", "ctrl+[":
		e.visual = visualNone
		return nil, nil

	case "v":
		if e.visual == visualChar {
			e.visual = visualNone
		} else {
			e.visual = visualChar
			e.visRow = e.row
			e.visCol = e.col
		}
		return nil, nil

	case "V":
		if e.visual == visualLine {
			e.visual = visualNone
		} else {
			e.visual = visualLine
			e.visRow = e.row
			e.visCol = e.col
		}
		return nil, nil

	case "o":
		e.visRow, e.row = e.row, e.visRow
		e.visCol, e.col = e.col, e.visCol
		e.clampCursor()
		e.ensureVisible()
		return nil, nil

	case "d", "x":
		e.saveUndo()
		if e.visual == visualLine {
			fromR, _, toR, _ := e.visualBounds()
			deleted := make([]string, 0, toR-fromR+1)
			for i := fromR; i <= toR && i < len(e.lines); i++ {
				deleted = append(deleted, e.lines[i])
			}
			e.deleteToReg(deleted, true)
			e.lines = append(e.lines[:fromR], e.lines[toR+1:]...)
			if len(e.lines) == 0 {
				e.lines = []string{""}
			}
			e.row = fromR
			if e.row >= len(e.lines) {
				e.row = len(e.lines) - 1
			}
			e.col = e.firstNonBlank(e.row)
		} else {
			fromR, fromC, toR, toC := e.visualBounds()
			text := e.extractText(fromR, fromC, toR, toC+1)
			e.deleteToReg([]string{text}, false)
			e.deleteRange(fromR, fromC, toR, toC+1)
			e.row = fromR
			e.col = fromC
		}
		e.visual = visualNone
		e.clampCursor()
		e.ensureVisible()
		return nil, nil

	case "c", "s":
		e.saveUndo()
		if e.visual == visualLine {
			fromR, _, toR, _ := e.visualBounds()
			deleted := make([]string, 0, toR-fromR+1)
			for i := fromR; i <= toR && i < len(e.lines); i++ {
				deleted = append(deleted, e.lines[i])
			}
			e.deleteToReg(deleted, true)
			e.lines = append(e.lines[:fromR], e.lines[toR+1:]...)
			nl := make([]string, 0, len(e.lines)+1)
			nl = append(nl, e.lines[:fromR]...)
			nl = append(nl, "")
			nl = append(nl, e.lines[fromR:]...)
			e.lines = nl
			e.row = fromR
			e.col = 0
		} else {
			fromR, fromC, toR, toC := e.visualBounds()
			text := e.extractText(fromR, fromC, toR, toC+1)
			e.deleteToReg([]string{text}, false)
			e.deleteRange(fromR, fromC, toR, toC+1)
			e.row = fromR
			e.col = fromC
		}
		e.visual = visualNone
		e.mode = ModeInsert
		e.insertEntry = "c-v"
		e.insertBuf.Reset()
		e.insertCount = 0
		return nil, nil

	case "y":
		if e.visual == visualLine {
			fromR, _, toR, _ := e.visualBounds()
			yanked := make([]string, 0, toR-fromR+1)
			for i := fromR; i <= toR && i < len(e.lines); i++ {
				yanked = append(yanked, e.lines[i])
			}
			e.yankToReg(yanked, true)
			e.statusMsg = fmt.Sprintf("%d lines yanked", len(yanked))
		} else {
			fromR, fromC, toR, toC := e.visualBounds()
			text := e.extractText(fromR, fromC, toR, toC+1)
			e.yankToReg([]string{text}, false)
		}
		e.row = e.visRow
		e.col = e.visCol
		if e.row > e.row {
			e.row, e.col = e.visRow, e.visCol
		}
		e.visual = visualNone
		e.clampCursor()
		return nil, nil

	case ">":
		fromR, _, toR, _ := e.visualBounds()
		e.saveUndo()
		for i := fromR; i <= toR && i < len(e.lines); i++ {
			e.lines[i] = "\t" + e.lines[i]
		}
		e.modified = true
		e.visual = visualNone
		e.row = fromR
		e.col = e.firstNonBlank(fromR)
		return nil, nil

	case "<":
		fromR, _, toR, _ := e.visualBounds()
		e.saveUndo()
		for i := fromR; i <= toR && i < len(e.lines); i++ {
			line := e.lines[i]
			if len(line) > 0 && line[0] == '\t' {
				e.lines[i] = line[1:]
			} else {
				trimmed := strings.TrimLeft(line, " ")
				removed := len(line) - len(trimmed)
				if removed > 8 {
					removed = 8
				}
				e.lines[i] = line[removed:]
			}
		}
		e.modified = true
		e.visual = visualNone
		e.row = fromR
		e.col = e.firstNonBlank(fromR)
		return nil, nil

	case "~":
		fromR, fromC, toR, toC := e.visualBounds()
		e.saveUndo()
		if e.visual == visualLine {
			for i := fromR; i <= toR && i < len(e.lines); i++ {
				runes := []rune(e.lines[i])
				for j := range runes {
					if unicode.IsLower(runes[j]) {
						runes[j] = unicode.ToUpper(runes[j])
					} else if unicode.IsUpper(runes[j]) {
						runes[j] = unicode.ToLower(runes[j])
					}
				}
				e.lines[i] = string(runes)
			}
		} else {
			for i := fromR; i <= toR; i++ {
				runes := []rune(e.lines[i])
				startC := 0
				endC := len(runes)
				if i == fromR {
					startC = fromC
				}
				if i == toR {
					endC = toC + 1
				}
				for j := startC; j < endC && j < len(runes); j++ {
					if unicode.IsLower(runes[j]) {
						runes[j] = unicode.ToUpper(runes[j])
					} else if unicode.IsUpper(runes[j]) {
						runes[j] = unicode.ToLower(runes[j])
					}
				}
				e.lines[i] = string(runes)
			}
		}
		e.modified = true
		e.visual = visualNone
		e.clampCursor()
		return nil, nil

	case "J":
		fromR, _, toR, _ := e.visualBounds()
		if fromR < toR {
			e.saveUndo()
			e.row = fromR
			for i := fromR; i < toR; i++ {
				e.doJoinLine(e.row)
			}
		}
		e.visual = visualNone
		e.clampCursor()
		return nil, nil

	case "u":
		fromR, fromC, toR, toC := e.visualBounds()
		e.saveUndo()
		if e.visual == visualLine {
			for i := fromR; i <= toR && i < len(e.lines); i++ {
				e.lines[i] = strings.ToLower(e.lines[i])
			}
		} else {
			for i := fromR; i <= toR; i++ {
				runes := []rune(e.lines[i])
				startC := 0
				endC := len(runes)
				if i == fromR {
					startC = fromC
				}
				if i == toR {
					endC = toC + 1
				}
				for j := startC; j < endC && j < len(runes); j++ {
					runes[j] = unicode.ToLower(runes[j])
				}
				e.lines[i] = string(runes)
			}
		}
		e.modified = true
		e.visual = visualNone
		return nil, nil

	case "U":
		fromR, fromC, toR, toC := e.visualBounds()
		e.saveUndo()
		if e.visual == visualLine {
			for i := fromR; i <= toR && i < len(e.lines); i++ {
				e.lines[i] = strings.ToUpper(e.lines[i])
			}
		} else {
			for i := fromR; i <= toR; i++ {
				runes := []rune(e.lines[i])
				startC := 0
				endC := len(runes)
				if i == fromR {
					startC = fromC
				}
				if i == toR {
					endC = toC + 1
				}
				for j := startC; j < endC && j < len(runes); j++ {
					runes[j] = unicode.ToUpper(runes[j])
				}
				e.lines[i] = string(runes)
			}
		}
		e.modified = true
		e.visual = visualNone
		return nil, nil

	case ":":
		fromR, _, toR, _ := e.visualBounds()
		e.visual = visualNone
		e.mode = ModeCommand
		e.cmdBuf = fmt.Sprintf("%d,%d", fromR+1, toR+1)
		return nil, nil

	default:
		if m, ok := e.motionDest(key, count, cg); ok {
			e.row = m.row
			e.col = m.col
			e.clampCursor()
			e.ensureVisible()
			return nil, nil
		}

		switch key {
		case "ctrl+f":
			e.row += e.height
			if e.row >= len(e.lines) {
				e.row = len(e.lines) - 1
			}
			e.topLine += e.height
			e.clampTop()
		case "ctrl+b":
			e.row -= e.height
			if e.row < 0 {
				e.row = 0
			}
			e.topLine -= e.height
			if e.topLine < 0 {
				e.topLine = 0
			}
		case "ctrl+d":
			half := e.height / 2
			e.row += half
			if e.row >= len(e.lines) {
				e.row = len(e.lines) - 1
			}
			e.topLine += half
			e.clampTop()
		case "ctrl+u":
			half := e.height / 2
			e.row -= half
			if e.row < 0 {
				e.row = 0
			}
			e.topLine -= half
			if e.topLine < 0 {
				e.topLine = 0
			}
		case "g":
			e.pendingKey = "g"
		case "/":
			e.searchDir = 1
			e.mode = ModeSearch
			e.cmdBuf = ""
		case "?":
			e.searchDir = -1
			e.mode = ModeSearch
			e.cmdBuf = ""
		case "n":
			if e.searchPat != "" {
				e.searchNext()
			}
		case "N":
			if e.searchPat != "" {
				e.searchPrev()
			}
		}

		e.clampCursor()
		e.ensureVisible()
	}

	return nil, nil
}
