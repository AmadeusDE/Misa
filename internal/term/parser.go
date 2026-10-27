package term

func (t *Terminal) processRune(r rune) {
	switch {
	case r == '\x1b':
		t.state = stEscape
	case r == '\n', r == '\v', r == '\f':
		t.linefeed()
	case r == '\r':
		t.curCol = 0
		t.wrapNext = false
	case r == '\t':
		t.tabForward(1)
	case r == '\b':
		if t.curCol > 0 {
			t.curCol--
		}
		t.wrapNext = false
	case r == '\x0e':
		t.charsets.gl = 1
	case r == '\x0f':
		t.charsets.gl = 0
	case r >= 0x20 && r != 0x7f && (r < 0x80 || r >= 0xa0):
		t.putChar(t.charsets.translate(r))
	}
}

func (t *Terminal) startCSI() {
	t.state = stCSI
	t.params = append(t.params[:0], 0)
	t.subParams = append(t.subParams[:0], nil)
	t.csiPrivate = 0
	t.csiSub = false
	t.csiInter = 0
}

func (t *Terminal) startOSC() {
	t.state = stOSC
	t.osc = t.osc[:0]
}

func (t *Terminal) processByte(b byte) {
	switch t.state {
	case stNormal:
		t.processRune(rune(b))

	case stEscape:
		t.state = stNormal
		switch b {
		case '[':
			t.startCSI()
		case ']':
			t.startOSC()
		case 'P', '_', '^', 'X':
			t.state = stString
		case '7':
			t.saveCursor()
		case '8':
			t.restoreCursor()
		case 'D':
			t.linefeed()
		case 'M':
			t.reverseLinefeed()
		case 'E':
			t.curCol = 0
			t.linefeed()
		case 'c':
			t.reset()
		case '(', ')', '*', '+':
			t.charsetTarget = b
			t.state = stCharset
		case '#', '%', ' ':
			t.state = stEscInter
		}

	case stCSI:
		t.csiByte(b)

	case stOSC:
		switch {
		case b == '\a':
			t.dispatchOSC("\a")
			t.state = stNormal
		case b == '\x1b':
			t.dispatchOSC("\x1b\\")
			t.state = stEscape
		case len(t.osc) < maxOSC:
			t.osc = append(t.osc, b)
		}

	case stCharset:
		t.charsets.designate(t.charsetTarget, b)
		t.state = stNormal

	case stEscInter:
		t.state = stNormal

	case stString:
		if b == '\x1b' {
			t.state = stEscape
		}
	}
}

func (t *Terminal) csiByte(b byte) {
	last := len(t.params) - 1
	switch {
	case b == 0x1b:
		t.state = stEscape
	case b < 0x20:
		t.processRune(rune(b))
	case b >= '0' && b <= '9':
		d := int(b - '0')
		if t.csiSub {
			subs := t.subParams[last]
			subs[len(subs)-1] = addDigit(max(subs[len(subs)-1], 0), d)
		} else {
			t.params[last] = addDigit(t.params[last], d)
		}
	case b == ':':
		t.csiSub = true
		if len(t.subParams[last]) < maxCSISubParams {
			t.subParams[last] = append(t.subParams[last], -1)
		}
	case b == ';':
		t.csiSub = false
		if len(t.params) < maxCSIParams {
			t.params = append(t.params, 0)
			t.subParams = append(t.subParams, nil)
		}
	case b >= '<' && b <= '?':
		t.csiPrivate = b
	case b >= 0x20 && b <= 0x2F:
		t.csiInter = b
	case b >= 0x40:
		t.state = stNormal
		t.dispatchCSI(b)
	}
}

// CSI parameters saturate and the lists stop growing past these, so hostile
// output can neither overflow a count nor grow the parser without bound.
const (
	maxCSIParam     = 65535
	maxCSIParams    = 32
	maxCSISubParams = 16
)

func addDigit(v, d int) int { return min(v*10+d, maxCSIParam) }

func (t *Terminal) dispatchCSI(final byte) {
	switch {
	case t.csiInter == '$' && final == 'p' && t.csiPrivate == '?':
		t.reportMode()
	case t.csiInter != 0:
	case t.csiPrivate == '?':
		t.handlePrivateCSI(final)
	case t.csiPrivate == '>' && final == 'c':
		if t.p(0, 0) == 0 {
			t.respond("\x1b[>1;10;0c")
		}
	case t.csiPrivate == 0:
		t.handleCSI(final)
	}
}

func (t *Terminal) p(i, def int) int {
	if i >= len(t.params) || t.params[i] == 0 {
		return def
	}
	return t.params[i]
}

func (t *Terminal) handlePrivateCSI(final byte) {
	switch final {
	case 'h':
		t.privateMode(true)
	case 'l':
		t.privateMode(false)
	case 'n':
		if t.p(0, 0) == 6 {
			t.respond(fmtCursorReport("\x1b[?", t.curRow, t.curCol))
		}
	}
}

func (t *Terminal) handleCSI(final byte) {
	switch final {
	case 'A':
		top := 0
		if t.curRow >= t.scrollTop {
			top = t.scrollTop
		}
		t.curRow = max(t.curRow-t.p(0, 1), top)
		t.wrapNext = false
	case 'B', 'e':
		bot := t.Rows - 1
		if t.curRow <= t.scrollBot {
			bot = t.scrollBot
		}
		t.curRow = min(t.curRow+t.p(0, 1), bot)
		t.wrapNext = false
	case 'C', 'a':
		t.curCol = min(t.curCol+t.p(0, 1), t.Cols-1)
		t.wrapNext = false
	case 'D':
		t.curCol = max(t.curCol-t.p(0, 1), 0)
		t.wrapNext = false
	case 'E':
		t.handleCSI('B')
		t.curCol = 0
	case 'F':
		t.handleCSI('A')
		t.curCol = 0
	case 'G', '`':
		t.curCol = t.p(0, 1) - 1
		t.clamp()
		t.wrapNext = false
	case 'H', 'f':
		t.curRow = t.p(0, 1) - 1
		t.curCol = t.p(1, 1) - 1
		t.clamp()
		t.wrapNext = false
	case 'I':
		t.tabForward(t.p(0, 1))
	case 'Z':
		t.tabBackward(t.p(0, 1))
	case 'J':
		t.eraseDisplay(t.p(0, 0))
	case 'K':
		t.eraseLine(t.p(0, 0))
	case 'L':
		t.insertLines(t.p(0, 1))
	case 'M':
		t.deleteLines(t.p(0, 1))
	case 'P':
		t.deleteChars(t.p(0, 1))
	case '@':
		t.insertChars(t.p(0, 1))
	case 'S':
		t.scrollUpN(t.p(0, 1))
	case 'T':
		t.scrollDownN(t.p(0, 1))
	case 'X':
		n := min(t.p(0, 1), t.Cols-t.curCol)
		t.fill(t.curRow, t.curCol, t.curCol+n)
	case 'b':
		t.repeatLast(t.p(0, 1))
	case 'd':
		t.curRow = t.p(0, 1) - 1
		t.clamp()
		t.wrapNext = false
	case 'm':
		t.handleSGR()
	case 'r':
		top := max(t.p(0, 1)-1, 0)
		bot := min(t.p(1, t.Rows)-1, t.Rows-1)
		if top < bot {
			t.scrollTop = top
			t.scrollBot = bot
		}
		t.curRow = 0
		t.curCol = 0
		t.wrapNext = false
	case 'n':
		switch t.p(0, 0) {
		case 5:
			t.respond("\x1b[0n")
		case 6:
			t.respond(fmtCursorReport("\x1b[", t.curRow, t.curCol))
		}
	case 'c':
		if t.p(0, 0) == 0 {
			t.respond("\x1b[?1;2c")
		}
	case 't':
		if t.p(0, 0) == 18 {
			t.respond(fmtSizeReport(t.Rows, t.Cols))
		}
	case 's':
		t.saveCursor()
	case 'u':
		t.restoreCursor()
	}
}

func (t *Terminal) privateMode(set bool) {
	for _, p := range t.params {
		switch p {
		case 1:
			t.appCursorKeys = set
		case 7:
			t.noAutowrap = !set
			if !set {
				t.wrapNext = false
			}
		case 25:
			t.cursorHidden = !set
		case 1049:
			if set {
				t.saveCursor()
				t.enterAlt()
			} else {
				t.exitAlt()
				t.restoreCursor()
			}
		case 47, 1047:
			if set {
				t.enterAlt()
			} else {
				t.exitAlt()
			}
		case 1048:
			if set {
				t.saveCursor()
			} else {
				t.restoreCursor()
			}
		case 2004:
			t.bracketedPaste = set
		case 9, 1000, 1002, 1003, 1006:
			t.setMouseMode(p, set)
		}
	}
}

func (t *Terminal) saveCursor() {
	t.saved = savedCursor{row: t.curRow, col: t.curCol, attr: t.attr, charsets: t.charsets}
}

func (t *Terminal) restoreCursor() {
	t.curRow, t.curCol = t.clampPos(t.saved.row, t.saved.col)
	t.attr = t.saved.attr
	t.charsets = t.saved.charsets
	t.wrapNext = false
}

func (t *Terminal) enterAlt() {
	if t.altScreen {
		return
	}
	t.altScreen = true
	t.cells, t.altCells = t.altCells, t.cells
	t.clearRange(0, 0, t.Rows-1, t.Cols-1)
}

func (t *Terminal) exitAlt() {
	if !t.altScreen {
		return
	}
	t.altScreen = false
	t.cells, t.altCells = t.altCells, t.cells
}

// mainCells is the normal-screen grid whether or not the alt screen is shown.
func (t *Terminal) mainCells() [][]Cell {
	if t.altScreen {
		return t.altCells
	}
	return t.cells
}
