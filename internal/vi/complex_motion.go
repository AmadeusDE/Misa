package vi

// complexMotion resolves the multi-key motions without changing the cursor or jumps.
func (e *Editor) complexMotion(key string, count int, countGiven bool) (motionResult, bool) {
	if count < 1 {
		count = 1
	}
	switch key {
	case "*", "#", "g*", "g#":
		return e.wordSearchMotion(key, count)
	case "n", "N":
		e.searchHighlightSuppressed = false
		return e.searchMotion(count, key == "N")
	case "%":
		if countGiven {
			if count > 100 || len(e.lines) == 0 {
				e.statusMsg = "Invalid percentage"
				return motionResult{}, false
			}
			// ceil(count * lines / 100), without overflowing.
			n := len(e.lines)
			row := (n/100)*count + ((n%100)*count+99)/100 - 1
			return motionResult{row, e.firstNonBlank(row), rangeLinewise}, true
		}
		return e.bracketMotion()
	case ";", ",":
		if e.lastFCmd == 0 {
			e.statusMsg = "No previous character search"
			return motionResult{}, false
		}
		cmd := e.lastFCmd
		if key == "," {
			cmd = reverseFind(cmd)
		}
		return e.findMotion(e.lastFChar, cmd, count, true)
	}
	rs := []rune(key)
	if len(rs) != 2 {
		return motionResult{}, false
	}
	switch rs[0] {
	case 'f', 'F', 't', 'T':
		e.lastFChar, e.lastFCmd = rs[1], byte(rs[0])
		return e.findMotion(rs[1], byte(rs[0]), count, false)
	case '\'', '`':
		// Uppercase marks retain the editor's existing same-buffer behavior.
		if (rs[1] < 'a' || rs[1] > 'z') && (rs[1] < 'A' || rs[1] > 'Z') {
			e.statusMsg = "Invalid mark"
			return motionResult{}, false
		}
		p, ok := e.marks[byte(rs[1])]
		if !ok || p.row < 0 || p.row >= len(e.lines) {
			e.statusMsg = "Mark not set"
			return motionResult{}, false
		}
		if rs[0] == '\'' {
			return motionResult{p.row, e.firstNonBlank(p.row), rangeLinewise}, true
		}
		col := min(max(p.col, 0), max(0, len(e.lineRunes(p.row))-1))
		return motionResult{p.row, col, rangeExclusive}, true
	}
	return motionResult{}, false
}

func reverseFind(cmd byte) byte {
	switch cmd {
	case 'f':
		return 'F'
	case 'F':
		return 'f'
	case 't':
		return 'T'
	case 'T':
		return 't'
	}
	return cmd
}

func (e *Editor) findMotion(ch rune, cmd byte, count int, repeat bool) (motionResult, bool) {
	line := e.lineRunes(e.row)
	if count > len(line) {
		e.statusMsg = "Character not found"
		return motionResult{}, false
	}
	dir := 1
	if cmd == 'F' || cmd == 'T' {
		dir = -1
	}
	till := cmd == 't' || cmd == 'T'
	col := e.col
	// An uncounted till repeat must move; counted repeats include the adjacent target.
	if repeat && till && count == 1 && col+dir >= 0 && col+dir < len(line) && line[col+dir] == ch {
		col += dir
	}
	for i := 0; i < count; i++ {
		col += dir
		for col >= 0 && col < len(line) && line[col] != ch {
			col += dir
		}
		if col < 0 || col >= len(line) {
			e.statusMsg = "Character not found"
			return motionResult{}, false
		}
	}
	if till {
		col -= dir
	}
	kind := rangeInclusive
	if dir < 0 {
		kind = rangeExclusive
	}
	return motionResult{e.row, col, kind}, true
}

func (e *Editor) bracketMotion() (motionResult, bool) {
	line := e.lineRunes(e.row)
	col := max(e.col, 0)
	for col < len(line) && !isBracket(line[col]) {
		col++
	}
	if col >= len(line) {
		e.statusMsg = "No matching bracket"
		return motionResult{}, false
	}
	ch := line[col]
	var target rune
	dir := 1
	switch ch {
	case '(':
		target = ')'
	case '[':
		target = ']'
	case '{':
		target = '}'
	case ')':
		target = '('
		dir = -1
	case ']':
		target = '['
		dir = -1
	case '}':
		target = '{'
		dir = -1
	}
	depth := 1
	row := e.row
	for {
		col += dir
		for col < 0 || col >= len(e.lineRunes(row)) {
			row += dir
			if row < 0 || row >= len(e.lines) {
				e.statusMsg = "No matching bracket"
				return motionResult{}, false
			}
			if dir > 0 {
				col = 0
			} else {
				col = len(e.lineRunes(row)) - 1
			}
		}
		c := e.lineRunes(row)[col]
		if c == ch {
			depth++
		} else if c == target {
			depth--
			if depth == 0 {
				return motionResult{row, col, rangeInclusive}, true
			}
		}
	}
}
func isBracket(r rune) bool {
	return r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}'
}

func (e *Editor) findChar(ch rune, cmd byte, count int) bool {
	m, ok := e.complexMotion(string(cmd)+string(ch), count, true)
	if ok {
		e.col = m.col
	}
	return ok
}
func (e *Editor) repeatFind(count int, reverse bool) bool {
	key := ";"
	if reverse {
		key = ","
	}
	m, ok := e.complexMotion(key, count, true)
	if ok {
		e.col = m.col
	}
	return ok
}
func (e *Editor) matchBracket() bool {
	m, ok := e.bracketMotion()
	if ok {
		e.addJump()
		e.row, e.col = m.row, m.col
		e.ensureVisible()
	}
	return ok
}
