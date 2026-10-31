package vi

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// --- Operator execution ---

// operatorRange converts motion endpoints into ordered half-open character ranges,
// or inclusive row ranges for linewise operations.
func (e *Editor) operatorRange(m motionResult) (fromR, fromC, toR, toC int, linewise bool) {
	fromR, fromC, toR, toC = e.row, e.col, m.row, m.col
	if fromR > toR || (fromR == toR && fromC > toC) {
		fromR, fromC, toR, toC = toR, toC, fromR, fromC
	}
	if m.kind == rangeLinewise {
		return fromR, 0, toR, 0, true
	}
	if m.kind == rangeExclusive && m.row > e.row && m.col == 0 {
		toR--
		if fromC <= e.firstNonBlank(fromR) {
			return fromR, 0, toR, 0, true
		}
		toC = len(e.lineRunes(toR))
	} else if m.kind == rangeInclusive {
		toC = min(toC+1, len(e.lineRunes(toR)))
	}
	return fromR, fromC, toR, toC, false
}

// wordOperatorDest permits an endpoint just past EOF, unlike cursor motion.
func (e *Editor) wordOperatorDest(count int) motionResult {
	row, col := e.row, e.col
	for n := 0; n < count; n++ {
		startRow := row
		runes := e.lineRunes(row)
		hadWord := col < len(runes) && !unicode.IsSpace(runes[col])
		if col < len(runes) {
			if isWordChar(runes[col]) {
				for col < len(runes) && isWordChar(runes[col]) {
					col++
				}
			} else if isPunct(runes[col]) {
				for col < len(runes) && isPunct(runes[col]) {
					col++
				}
			}
		}
		endWord := col
		for {
			runes = e.lineRunes(row)
			for col < len(runes) && unicode.IsSpace(runes[col]) {
				col++
			}
			if col < len(runes) {
				break
			}
			if row == len(e.lines)-1 {
				return motionResult{row, len(runes), rangeExclusive}
			}
			if n == count-1 && hadWord && row == startRow {
				return motionResult{row, endWord, rangeExclusive}
			}
			row++
			col = 0
			if len(e.lineRunes(row)) == 0 {
				break
			}
		}
	}
	return motionResult{row, col, rangeExclusive}
}

func (e *Editor) executeOperatorMotion(op byte, key string, count int, countGiven bool) {
	before := slices.Clone(e.lines)
	change := semanticChange{command: string(op), motion: key, count: count, countGiven: countGiven, register: e.repeatRegister()}
	defer func() { e.recordOperatorChange(change, before) }()
	change.findChar, change.findCmd = e.lastFChar, e.lastFCmd
	change.searchPat, change.searchDir = e.searchPat, e.searchDir
	m, ok := e.resolveMotion(key, count, countGiven)
	if !ok {
		return
	}
	if key == "w" {
		runes := e.lineRunes(e.row)
		if op == 'c' && e.col < len(runes) && !unicode.IsSpace(runes[e.col]) {
			atEnd := e.col+1 == len(runes) || (isWordChar(runes[e.col]) && !isWordChar(runes[e.col+1])) || (isPunct(runes[e.col]) && !isPunct(runes[e.col+1]))
			if atEnd && count == 1 {
				m = motionResult{e.row, e.col, rangeInclusive}
			} else {
				endCount := count
				if atEnd {
					endCount--
				}
				m, _ = e.motionDest("e", endCount, countGiven)
			}
		} else {
			m = e.wordOperatorDest(e.motionLimit(count))
		}
	}
	if key == "l" || key == "right" {
		m.col = min(e.col+count, len(e.lineRunes(e.row)))
	}
	e.executeOperatorResult(op, key, count, m)
}

func (e *Editor) executeOperatorResult(op byte, key string, count int, m motionResult) {
	if key == "n" && e.pendingOp != 0 {
		before := slices.Clone(e.lines)
		change := semanticChange{command: string(op), motion: key, count: count, countGiven: true, register: e.repeatRegister(), searchPat: e.searchPat, searchDir: e.searchDir}
		defer func() { e.recordOperatorChange(change, before) }()
	}
	fromR, fromC, toR, toC, linewise := e.operatorRange(m)
	if !linewise && fromR == toR && fromC == toC {
		if op == 'c' && (key == "$" || key == "end") {
			e.saveUndo()
			e.mode = ModeInsert
			e.insertEntry = "c-" + key
			e.insertBuf.Reset()
			e.insertCount = 0
		}
		return
	}
	if op != 'y' {
		e.saveUndo()
	}
	if linewise {
		e.applyLineOp(op, fromR, toR, key, count)
	} else {
		e.applyCharOp(op, fromR, fromC, toR, toC, key, count)
	}
	if op == 'y' && (m.row < e.row || (m.row == e.row && m.col < e.col)) {
		e.row, e.col = fromR, fromC
		if linewise {
			e.col = e.firstNonBlank(fromR)
		}
	}
	e.goalValid, e.goalEnd = false, false
	e.clampCursor()
	e.ensureVisible()
}

func (e *Editor) applyLineOp(op byte, fromR, toR int, motionKey string, count int) {
	deleted := make([]string, 0, toR-fromR+1)
	for i := fromR; i <= toR && i < len(e.lines); i++ {
		deleted = append(deleted, e.lines[i])
	}

	switch op {
	case 'd':
		e.noteEdit(fromR, 0)
		e.modified = true
		e.deleteToReg(deleted, true)
		if toR >= len(e.lines) {
			toR = len(e.lines) - 1
		}
		e.lines = append(e.lines[:fromR], e.lines[toR+1:]...)
		if len(e.lines) == 0 {
			e.lines = []string{""}
		}
		e.row = fromR
		if e.row >= len(e.lines) {
			e.row = len(e.lines) - 1
		}
		e.col = e.firstNonBlank(e.row)
	case 'c':
		e.noteEdit(fromR, 0)
		e.modified = true
		e.deleteToReg(deleted, true)
		if toR >= len(e.lines) {
			toR = len(e.lines) - 1
		}
		e.lines = append(e.lines[:fromR], e.lines[toR+1:]...)
		nl := make([]string, 0, len(e.lines)+1)
		nl = append(nl, e.lines[:fromR]...)
		nl = append(nl, "")
		nl = append(nl, e.lines[fromR:]...)
		e.lines = nl
		e.row = fromR
		e.col = 0
		e.mode = ModeInsert
		e.insertEntry = "c-" + motionKey
		e.insertBuf.Reset()
		e.insertCount = 0
	case 'y':
		e.yankToReg(deleted, true)
		e.statusMsg = fmt.Sprintf("%d lines yanked", len(deleted))
	case '>':
		for i := fromR; i <= toR && i < len(e.lines); i++ {
			e.lines[i] = "\t" + e.lines[i]
		}
		e.modified = true
		e.row = fromR
		e.col = e.firstNonBlank(fromR)
	case '<':
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
		e.row = fromR
		e.col = e.firstNonBlank(fromR)
	}
}

func (e *Editor) applyCharOp(op byte, fromR, fromC, toR, toC int, motionKey string, count int) {
	text := e.extractText(fromR, fromC, toR, toC)

	switch op {
	case 'd':
		e.deleteToReg([]string{text}, false)
		e.deleteRange(fromR, fromC, toR, toC)
		e.row = fromR
		e.col = fromC
	case 'c':
		e.deleteToReg([]string{text}, false)
		e.deleteRange(fromR, fromC, toR, toC)
		e.row = fromR
		e.col = fromC
		e.mode = ModeInsert
		e.insertEntry = "c-" + motionKey
		e.insertBuf.Reset()
		e.insertCount = 0
	case 'y':
		e.yankToReg([]string{text}, false)
	case '>':
		for i := fromR; i <= toR && i < len(e.lines); i++ {
			e.lines[i] = "\t" + e.lines[i]
		}
		e.modified = true
		e.row = fromR
		e.col = e.firstNonBlank(fromR)
	case '<':
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
		e.row = fromR
		e.col = e.firstNonBlank(fromR)
	}
}

func (e *Editor) executeLineOp(op byte, count int) {
	before := slices.Clone(e.lines)
	change := semanticChange{command: string(op), motion: "line", count: count, countGiven: true, register: e.repeatRegister()}
	defer func() { e.recordOperatorChange(change, before) }()
	to := min(e.row+count-1, len(e.lines)-1)
	if op != 'y' {
		e.saveUndo()
	}
	e.applyLineOp(op, e.row, to, "_", count)
	if op == 'c' {
		e.insertEntry = "S"
	}
	e.goalValid, e.goalEnd = false, false
	e.clampCursor()
	e.ensureVisible()
}
