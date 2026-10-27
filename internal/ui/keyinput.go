package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
	"github.com/AmadeusDE/Misa/internal/termkeys"
)

func keyToBytes(msg tea.KeyPressMsg) []byte { return termkeys.KeyPress(msg, false) }

func keyToBytesMode(msg tea.KeyPressMsg, appCursor bool) []byte {
	return termkeys.KeyPress(msg, appCursor)
}

func sendKey(t *term.Terminal, msg tea.KeyPressMsg) { termkeys.SendKey(t, msg) }

func pasteInput(text string, t *term.Terminal) []byte { return termkeys.PasteBytes(text, t) }

func sendPaste(t *term.Terminal, text string) { termkeys.SendPaste(t, text) }

func mouseReport(msg mouseMsg, x, y int, mode term.MouseMode) []byte {
	action := termkeys.Press
	switch msg.Action {
	case mouseRelease:
		action = termkeys.Release
	case mouseMotion:
		action = termkeys.Motion
	}
	return termkeys.MouseReport(msg.Mouse, action, x, y, mode)
}
