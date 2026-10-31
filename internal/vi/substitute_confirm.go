package vi

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

const confirmationInCompound = "Confirmation is not supported inside compound commands"

type substituteConfirm struct {
	pattern, replacement, display string
	row, to                       int
	next, start, end              int // byte positions within the current line
	total, lastRow                int
	global, ignoreCase            bool
}

func (c *substituteConfirm) prompt() string {
	return fmt.Sprintf("replace with %s (y/n/a/q/l/^E/^Y)?", c.display)
}

func (e *Editor) startSubstituteConfirm(from, to int, pattern, replacement string, global, ignoreCase bool) {
	cleanPattern := pattern
	if strings.HasPrefix(cleanPattern, `\`) {
		cleanPattern = strings.ReplaceAll(strings.ReplaceAll(cleanPattern, `\(`, "("), `\)`, ")")
	}
	cleanReplacement := strings.ReplaceAll(strings.ReplaceAll(replacement, `\n`, "\n"), `\t`, "\t")
	if cleanPattern == "" {
		e.statusMsg = "Empty patterns are not supported with confirmation"
		return
	}
	if strings.ContainsRune(cleanReplacement, '\n') {
		e.statusMsg = "Newline replacement is not supported with confirmation"
		return
	}
	if e.editDepth != 0 {
		e.statusMsg = confirmationInCompound
		return
	}
	c := &substituteConfirm{
		pattern: cleanPattern, replacement: cleanReplacement, display: replacement,
		row: from, to: to, lastRow: -1, global: global, ignoreCase: ignoreCase,
	}
	if !c.find(e.lines) {
		e.statusMsg = "Pattern not found: " + pattern
		return
	}
	e.beginEdit()
	e.confirm = c
	e.mode = ModeConfirm
	e.showSubstituteMatch()
}

func (c *substituteConfirm) find(lines []string) bool {
	for c.row <= c.to && c.row < len(lines) {
		line := lines[c.row]
		if c.next <= len(line) {
			start, end := strings.Index(line[c.next:], c.pattern), 0
			if c.ignoreCase {
				start, end = foldIndex(line[c.next:], c.pattern)
			} else if start >= 0 {
				end = start + len(c.pattern)
			}
			if start >= 0 {
				c.start, c.end = c.next+start, c.next+end
				return true
			}
		}
		c.row++
		c.next = 0
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
	c.next = c.end
	if replace {
		line := e.lines[c.row]
		newLine := line[:c.start] + c.replacement + line[c.end:]
		if newLine != line {
			e.noteEdit(c.row, utf8.RuneCountInString(line[:c.start]))
			e.lines[c.row] = newLine
			e.materializeEdit()
			e.modified = e.revision != e.savedRevision
		}
		c.total++
		c.lastRow = c.row
		c.next = c.start + len(c.replacement)
	}
	if !c.global {
		c.row++
		c.next = 0
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
