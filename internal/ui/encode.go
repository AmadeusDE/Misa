package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

// KeyBytes encodes a key press the way xterm sends it, for frontends that
// drive a Misa session through a PTY.
func KeyBytes(k tea.Key, appCursor bool) []byte {
	return keyToBytesMode(tea.KeyPressMsg(k), appCursor)
}

// MouseBytes reports msg at cell (x, y) to a program in tracking mode, or
// returns nil for events that mode excludes.
func MouseBytes(msg tea.MouseMsg, x, y int, mode term.MouseMode) []byte {
	return mouseReport(fromTea(msg), x, y, mode)
}

// PasteBytes encodes pasted text for t, bracketed when its program asked.
func PasteBytes(text string, t *term.Terminal) []byte { return pasteInput(text, t) }
