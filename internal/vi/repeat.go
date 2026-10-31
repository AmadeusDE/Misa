package vi

import "slices"

// semanticChange stores an editing operation rather than a closure over editor state.
// A zero register denotes the current unnamed register.
type semanticChange struct {
	command, motion string
	count           int
	countGiven      bool
	register        byte
	insertKeys      []string
	findChar        rune
	findCmd         byte
	searchPat       string
	searchDir       int
}

func (e *Editor) repeatRegister() byte {
	if e.hasReg {
		return e.pendingReg
	}
	return 0
}

func (e *Editor) recordOperatorChange(c semanticChange, before []string) {
	if e.replayingChange || c.command == "y" {
		return
	}
	if e.mode == ModeInsert {
		e.repeatPending = c
	} else if !sameLines(before, e.lines) {
		e.repeatChange = c
	}
}

// finishRepeatInsert is called before the insert transaction is committed.
func (e *Editor) finishRepeatInsert(keys []string) {
	if e.replayingChange {
		return
	}
	if e.editDepth != 0 && !sameLines(e.editBefore.lines, e.lines) {
		c := e.repeatPending
		if c.command != "" {
			c.insertKeys = slices.Clone(keys)
			e.repeatChange = c
		}
	}
	e.repeatPending = semanticChange{}
}

func (e *Editor) repeatLastChange(count int, given bool) {
	c := e.repeatChange
	if c.command == "" {
		return
	}
	if given {
		c.count, c.countGiven = count, true
	}
	e.beginEdit()
	e.replayingChange = true
	oldChar, oldCmd := e.lastFChar, e.lastFCmd
	oldPat, oldDir := e.searchPat, e.searchDir
	if c.motion == ";" || c.motion == "," {
		e.lastFChar, e.lastFCmd = c.findChar, c.findCmd
	}
	if c.motion == "n" || c.motion == "N" {
		e.searchPat, e.searchDir = c.searchPat, c.searchDir
	}
	if c.register != 0 {
		e.hasReg, e.pendingReg = true, c.register
	}
	if c.motion == "line" {
		e.executeLineOp(c.command[0], c.count)
	} else if c.motion != "" {
		e.executeOperatorMotion(c.command[0], c.motion, c.count, c.countGiven)
	} else {
		e.count, e.countGiven = c.count, c.countGiven
		e.handleNormal(keyFromName(c.command))
	}
	if e.mode == ModeInsert || e.mode == ModeReplace || e.mode == ModeReplaceChar {
		e.replayInsertInput(c.insertKeys)
		if e.mode == ModeInsert || e.mode == ModeReplace {
			e.processKey("esc")
		}
	}
	if c.motion == ";" || c.motion == "," {
		e.lastFChar, e.lastFCmd = oldChar, oldCmd
	}
	if c.motion == "n" || c.motion == "N" {
		e.searchPat, e.searchDir = oldPat, oldDir
	}
	e.clearCommandState()
	e.replayingChange = false
	changed := !sameLines(e.editBefore.lines, e.lines)
	e.commitEdit()
	if changed && given {
		e.repeatChange = c
	}
	e.clampCursor()
	e.ensureVisible()
}
