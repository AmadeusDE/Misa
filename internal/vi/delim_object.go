package vi

// Dispatch of the delimiter objects (pairs and quotes) for operators and
// Visual mode. The scanners live in pair_object.go and quote_object.go.
//
// A failed object changes nothing: the cursor stays, no register is touched
// and the pending operator is dropped. Misa has no audible bell, so the
// failure is silent.

// delimObject describes the delimiters of an object. For a quote open and
// close are the same character.
type delimObject struct {
	open, close rune
	quote       bool
}

// delimObjectFor resolves a key such as "i(" or "aB". It reports false for
// every other key, including the word objects.
func delimObjectFor(key string) (delimObject, bool) {
	if len(key) != 2 || (key[0] != 'i' && key[0] != 'a') {
		return delimObject{}, false
	}
	switch key[1] {
	case '(', ')', 'b':
		return delimObject{open: '(', close: ')'}, true
	case '[', ']':
		return delimObject{open: '[', close: ']'}, true
	case '{', '}', 'B':
		return delimObject{open: '{', close: '}'}, true
	case '<', '>':
		return delimObject{open: '<', close: '>'}, true
	case '"', '\'', '`':
		q := rune(key[1])
		return delimObject{open: q, close: q, quote: true}, true
	}
	return delimObject{}, false
}

// isTextObjectKey reports whether key names any text object.
func isTextObjectKey(key string) bool {
	if _, ok := delimObjectFor(key); ok {
		return true
	}
	return key == "iw" || key == "aw" || key == "iW" || key == "aW"
}

// delimSpan computes the operator span of a delimiter object at the cursor.
func (e *Editor) delimSpan(d delimObject, around bool, at textPos, count int) (textSpan, bool) {
	if d.quote {
		mode := quoteInner
		switch {
		case around:
			mode = quoteAround
		case count >= 2:
			mode = quoteQuotes
		}
		from, to, ok := quoteObject(e.lineRunes(at.row), at.col, d.open, mode)
		return textSpan{from: textPos{at.row, from}, to: textPos{at.row, to}}, ok
	}
	p, ok := e.pairFor(at, d, count)
	if !ok {
		return textSpan{}, false
	}
	return e.pairObject(p, around).span, true
}

// applyDelimSpan runs an operator over the span of a delimiter object.
func (e *Editor) applyDelimSpan(op byte, key string, count int, span textSpan) {
	from, to := span.from, span.to
	if op == '!' {
		last := to.row
		if !span.whole && to.col == 0 && last > from.row {
			last--
		}
		e.openShellFilter(from.row, last)
		return
	}
	switch {
	case span.whole:
		if op != 'y' {
			e.saveUndo()
		}
		e.applyLineOp(op, from.row, to.row, key, count)
		if op == 'y' {
			e.row, e.col = from.row, 0
		}
	case from == to:
		// An empty object deletes and yanks nothing but still moves the
		// cursor, and a change opens an insert between the delimiters.
		e.row, e.col = from.row, from.col
		if op == 'c' {
			e.saveUndo()
			e.mode = ModeInsert
			e.insertEntry = "c-" + key
			e.insertBuf.Reset()
			e.insertCount = 0
		}
	default:
		if op != 'y' {
			e.saveUndo()
		}
		e.applyCharOpText(op, from.row, from.col, to.row, to.col, key, count, e.extractText(from.row, from.col, to.row, to.col))
		if op == 'y' {
			e.row, e.col = from.row, from.col
		}
	}
	e.goalValid, e.goalEnd = false, false
	e.clampCursor()
	e.ensureVisible()
}

// --- Visual ----------------------------------------------------------------

// visualDelimObject applies a delimiter object to the Visual selection. With
// a single character selected it selects the object around the cursor. With a
// larger selection it selects the closest object that is strictly larger than
// the selection, so repeating the key grows the selection outwards.
func (e *Editor) visualDelimObject(key string, count int) {
	d, _ := delimObjectFor(key)
	around := key[0] == 'a'
	cursor := textPos{e.row, e.col}
	from, to := cursor, e.after(cursor)
	trivial := e.visual == visualLine
	if !trivial {
		fromR, fromC, toR, toC, _ := e.visualHalfOpen()
		from, to = textPos{fromR, fromC}, textPos{toR, toC}
		trivial = from == to || e.after(from) == to
		if trivial {
			from, to = cursor, e.after(cursor)
		}
	}
	var selFrom, selTo textPos
	var ok bool
	if d.quote {
		selFrom, selTo, ok = e.visualQuote(d, around, count, from, to, trivial)
	} else {
		selFrom, selTo, ok = e.visualPair(d, around, count, from, to, trivial)
	}
	if !ok {
		return
	}
	e.visObj, e.visBack, e.visObjValid = textSpan{from: selFrom, to: selTo}, false, true
	if e.visual == visualLine {
		e.visual = visualChar
	}
	e.visRow, e.visCol = selFrom.row, selFrom.col
	end := e.lastIncluded(selFrom, selTo)
	e.row, e.col = end.row, end.col
	e.clampCursor()
	e.ensureVisible()
}

// visualPair finds the pair object for a Visual selection [from, to).
func (e *Editor) visualPair(d delimObject, around bool, count int, from, to textPos, trivial bool) (textPos, textPos, bool) {
	if trivial {
		p, ok := e.pairFor(from, d, count)
		if !ok {
			return from, to, false
		}
		obj := e.pairObject(p, around)
		return obj.span.from, obj.visualTo, obj.span.from != obj.visualTo
	}
	found := 0
	for n := 1; ; n++ {
		p, ok := e.nthPair(from, d, n)
		if !ok {
			return from, to, false
		}
		obj := e.pairObject(p, around)
		if !strictlyContains(obj.span.from, obj.visualTo, from, to) {
			continue
		}
		if found++; found == count {
			return obj.span.from, obj.visualTo, true
		}
	}
}

// visualQuote finds the quote object for a Visual selection [from, to).
func (e *Editor) visualQuote(d delimObject, around bool, count int, from, to textPos, trivial bool) (textPos, textPos, bool) {
	runes := e.lineRunes(from.row)
	start, end, ok := quoteBounds(runes, from.col, d.open)
	if !ok {
		return from, to, false
	}
	modes := []quoteMode{quoteInner, quoteQuotes}
	switch {
	case around:
		modes = []quoteMode{quoteAround}
	case trivial && count >= 2:
		modes = []quoteMode{quoteQuotes}
	}
	for _, mode := range modes {
		objFrom, objTo := quoteRange(runes, start, end, mode)
		a, b := textPos{from.row, objFrom}, textPos{from.row, objTo}
		if (trivial && a != b) || strictlyContains(a, b, from, to) {
			return a, b, true
		}
		if trivial {
			break
		}
	}
	return from, to, false
}

// strictlyContains reports whether [aFrom, aTo) holds [bFrom, bTo) and is larger.
func strictlyContains(aFrom, aTo, bFrom, bTo textPos) bool {
	return !posLess(bFrom, aFrom) && !posLess(aTo, bTo) && (aFrom != bFrom || aTo != bTo)
}
