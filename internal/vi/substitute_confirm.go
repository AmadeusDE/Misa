package vi

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/vi/vipat"
)

const confirmationInCompound = "Confirmation is not supported inside compound commands"

type substituteConfirm struct {
	re               *vipat.Regexp
	replacement      string
	display          string
	row, to          int
	next, start, end int // byte positions within the current line
	prevEnd          int // end of the last handled match in the current line, or -1
	total, lastRow   int
	global           bool
	err              error
}

func (c *substituteConfirm) prompt() string {
	return fmt.Sprintf("replace with %s (y/n/a/q/l/^E/^Y)?", c.display)
}

func (e *Editor) startSubstituteConfirm(from, to int, re *vipat.Regexp, pattern, replacement string, global bool) {
	cleanReplacement := replacementText(replacement)
	if strings.ContainsRune(cleanReplacement, '\n') {
		e.statusMsg = "Newline replacement is not supported with confirmation"
		return
	}
	if e.editDepth != 0 {
		e.statusMsg = confirmationInCompound
		return
	}
	c := &substituteConfirm{
		re: re, replacement: cleanReplacement, display: replacement,
		row: from, to: to, lastRow: -1, prevEnd: -1, global: global,
	}
	if !c.find(e.lines) {
		e.statusMsg = "Pattern not found: " + pattern
		if c.err != nil {
			e.statusMsg = c.err.Error()
		}
		return
	}
	e.beginEdit()
	e.confirm = c
	e.mode = ModeConfirm
	e.showSubstituteMatch()
}

func (c *substituteConfirm) find(lines []string) bool {
	for c.row <= c.to && c.row < len(lines) {
		start, end, ok, err := nextSubstitution(c.re, lines[c.row], c.next, c.prevEnd)
		if err != nil {
			c.err = err
			return false
		}
		if ok {
			c.start, c.end = start, end
			return true
		}
		c.row++
		c.next, c.prevEnd = 0, -1
	}
	return false
}

func (e *Editor) showSubstituteMatch() {
	c := e.confirm
	e.row, e.col = c.row, utf8.RuneCountInString(e.lines[c.row][:c.start])
	e.clampCursor()
	e.ensureVisible()
}

func (e *Editor) answerSubstitute(replace bool) bool {
	c := e.confirm
	line := e.lines[c.row]
	c.prevEnd = c.end
	if replace {
		newLine := line[:c.start] + c.replacement + line[c.end:]
		if newLine != line {
			e.noteEdit(c.row, utf8.RuneCountInString(line[:c.start]))
			e.lines[c.row] = newLine
			e.materializeEdit()
			e.modified = e.revision != e.savedRevision
		}
		c.total++
		c.lastRow = c.row
		c.prevEnd = c.start + len(c.replacement)
		line = newLine
	}
	c.next = c.prevEnd
	if c.start == c.end && c.next < len(line) {
		_, w := utf8.DecodeRuneInString(line[c.next:])
		c.next += w
	}
	if !c.global {
		c.row++
		c.next, c.prevEnd = 0, -1
	}
	return c.find(e.lines)
}

func (e *Editor) finishSubstituteConfirm() {
	c := e.confirm
	e.confirm = nil
	e.mode = ModeNormal
	e.commitEdit()
	if c.lastRow >= 0 {
		e.row = c.lastRow
		e.col = e.firstNonBlank(e.row)
	}
	e.clampCursor()
	e.ensureVisible()
	e.statusMsg = fmt.Sprintf("%d substitution(s)", c.total)
	if c.err != nil {
		e.statusMsg = c.err.Error()
	}
}

func (e *Editor) handleSubstituteConfirm(msg tea.KeyPressMsg) {
	switch typedKey(msg) {
	case "y", "n":
		if e.answerSubstitute(typedKey(msg) == "y") {
			e.showSubstituteMatch()
			return
		}
	case "a":
		for e.answerSubstitute(true) {
		}
	case "l":
		e.answerSubstitute(true)
	case "q", "esc", "ctrl+[", "ctrl+c":
	case "ctrl+e":
		e.topLine++
		e.clampTop()
		return
	case "ctrl+y":
		e.topLine--
		e.clampTop()
		return
	default:
		return
	}
	e.finishSubstituteConfirm()
}
