package vi

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// inputEvent is the editor's normalized input stream. Text is literal data
// (used by paste and its semantic replay), never an editor command.
type inputKind uint8

const (
	inputKey inputKind = iota
	inputText
	inputPaste
)

type inputEvent struct {
	kind inputKind
	key  tea.KeyPressMsg
	text string
}

// dispatchInput is the single mode dispatcher for live input and replay.
func (e *Editor) dispatchInput(in inputEvent) tea.Cmd {
	if in.kind == inputKey {
		// Terminals encode Ctrl-I as Tab; explicit Ctrl-I events are equivalent.
		if in.key.Code == 'i' && in.key.Mod == tea.ModCtrl {
			in.key = keyFromName("tab")
		}
		if in.key.Mod&(tea.ModCtrl|tea.ModAlt) == 0 && utf8.RuneCountInString(in.key.Text) > 1 {
			var cmds []tea.Cmd
			for _, r := range in.key.Text {
				if cmd := e.dispatchInput(inputEvent{kind: inputKey, key: keyFromName(string(r))}); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			return tea.Batch(cmds...)
		}
		e.statusMsg = ""
	}
	if in.kind == inputPaste {
		in.text = strings.ReplaceAll(strings.ReplaceAll(in.text, "\r\n", "\n"), "\r", "\n")
		if in.text == "" {
			return nil
		}
	}
	switch e.mode {
	case ModeNormal:
		if e.visual != visualNone {
			if in.kind == inputKey {
				e.updateVisual(in.key)
			}
			return nil
		}
		switch in.kind {
		case inputKey:
			return e.handleNormal(in.key)
		case inputText:
			for _, r := range in.text {
				e.insertLiteral(r, false)
			}
		case inputPaste:
			e.clearCommandState()
			e.saveUndo()
			e.dispatchInput(inputEvent{kind: inputText, text: in.text})
			if e.col > 0 {
				e.col--
			}
			e.clampCursor()
		}
	case ModeInsert, ModeReplace:
		switch in.kind {
		case inputKey:
			if e.mode == ModeInsert {
				return e.handleInsert(in.key)
			}
			return e.handleReplace(in.key)
		case inputPaste:
			e.dispatchInput(inputEvent{kind: inputText, text: in.text})
		case inputText:
			for _, r := range in.text {
				e.insertLiteral(r, true)
			}
		}
	case ModeReplaceChar:
		if in.kind == inputKey {
			return e.handleReplaceChar(in.key)
		}
	case ModeCommand, ModeSearch:
		if in.kind == inputKey {
			if e.mode == ModeCommand {
				return e.handleCommand(in.key)
			}
			return e.handleSearch(in.key)
		}
		if in.kind == inputPaste {
			var b strings.Builder
			for _, r := range in.text {
				if r == '\n' {
					b.WriteByte(' ')
				} else if !unicode.IsControl(r) {
					b.WriteRune(r)
				}
			}
			e.cmdBuf += b.String()
			if e.mode == ModeSearch && e.incSearch {
				e.previewSearch()
			}
		}
	case ModeConfirm:
		if in.kind == inputKey {
			e.handleSubstituteConfirm(in.key)
		}
	}
	return nil
}

// insertLiteral records a tagged rune so repeat never interprets pasted text
// as editing keys, including backspace and escape.
func (e *Editor) insertLiteral(r rune, record bool) {
	if record {
		e.insertBuf.WriteRune(0)
		e.insertBuf.WriteRune(r)
	}
	if r == '\n' {
		if e.mode == ModeReplace {
			e.replaceSteps = append(e.replaceSteps, replaceStep{row: e.row, col: e.col, newline: true})
		}
		e.splitLine(e.row, e.col)
		e.row++
		e.col = 0
		return
	}
	if e.mode == ModeReplace {
		lr := e.lineRunes(e.row)
		step := replaceStep{row: e.row, col: e.col}
		if e.col < len(lr) {
			step.old, step.existed = lr[e.col], true
			lr[e.col] = r
			e.setLine(e.row, string(lr))
		} else {
			e.insertCharAt(e.row, e.col, r)
		}
		e.replaceSteps = append(e.replaceSteps, step)
	} else {
		e.insertCharAt(e.row, e.col, r)
	}
	e.col++
}
