package termkeys

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

// Action is what a mouse event did.
type Action int

const (
	Press Action = iota
	Release
	Motion
)

const (
	mouseShift     = 4
	mouseAlt       = 8
	mouseCtrl      = 16
	mouseMotionBit = 32
	mouseWheel     = 64
)

var mouseButtonCodes = map[tea.MouseButton]int{
	tea.MouseLeft:       0,
	tea.MouseMiddle:     1,
	tea.MouseRight:      2,
	tea.MouseNone:       3,
	tea.MouseWheelUp:    64,
	tea.MouseWheelDown:  65,
	tea.MouseWheelLeft:  66,
	tea.MouseWheelRight: 67,
	tea.MouseBackward:   128,
	tea.MouseForward:    129,
}

// MouseReport encodes msg at cell (x, y) the way xterm reports it to a
// program, or returns nil for events the program's tracking mode excludes.
func MouseReport(msg tea.Mouse, action Action, x, y int, mode term.MouseMode) []byte {
	code, ok := mouseButtonCodes[msg.Button]
	if mode.Tracking == 0 || !ok || x < 0 || y < 0 {
		return nil
	}
	motion := action == Motion
	release := action == Release
	switch {
	case motion && mode.Tracking < 1002:
		return nil
	case motion && mode.Tracking == 1002 && msg.Button == tea.MouseNone:
		return nil
	case release && (mode.Tracking == 9 || code >= mouseWheel):
		return nil
	}
	if mode.Tracking != 9 {
		if msg.Mod.Contains(tea.ModShift) {
			code |= mouseShift
		}
		if msg.Mod.Contains(tea.ModAlt) {
			code |= mouseAlt
		}
		if msg.Mod.Contains(tea.ModCtrl) {
			code |= mouseCtrl
		}
	}
	if motion {
		code |= mouseMotionBit
	}
	if mode.SGR {
		final := 'M'
		if release {
			final = 'm'
		}
		return fmt.Appendf(nil, "\x1b[<%d;%d;%d%c", code, x+1, y+1, final)
	}
	if release {
		code = 3 | code&(mouseShift|mouseAlt|mouseCtrl)
	}
	if x > 222 || y > 222 {
		return nil
	}
	return []byte{0x1b, '[', 'M', byte(32 + code), byte(33 + x), byte(33 + y)}
}
