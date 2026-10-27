package ui

import (
	"bytes"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

// forwardMouse hands msg, at terminal cell (x, y), to a program that tracks
// the mouse. Shift keeps Misa's own selection reachable, as in most
// terminals. It reports whether the event belongs to the program.
func forwardMouse(t *term.Terminal, msg mouseMsg, x, y int) bool {
	mode := t.Mouse()
	if mode.Tracking == 0 || msg.shift() || y < 0 || y >= t.Rows {
		return false
	}
	if b := mouseReport(msg, x, y, mode); b != nil {
		t.SendBytes(b)
	}
	return true
}

// scrollAltScreen turns the wheel into arrow keys for full-screen programs
// that don't track the mouse (less, man), as xterm's alternate scroll does.
func scrollAltScreen(t *term.Terminal, msg mouseMsg) bool {
	if !t.InAltScreen() || t.Mouse().Tracking != 0 {
		return false
	}
	var key rune
	switch msg.Button {
	case tea.MouseWheelUp:
		key = tea.KeyUp
	case tea.MouseWheelDown:
		key = tea.KeyDown
	default:
		return false
	}
	arrow := keyToBytesMode(tea.KeyPressMsg{Code: key}, t.AppCursorKeys())
	t.SendBytes(bytes.Repeat(arrow, 3))
	return true
}
