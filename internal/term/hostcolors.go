package term

import (
	"fmt"
	"image/color"
	"sync/atomic"
)

// hostFg and hostBg are the real terminal's default colors as OSC 10/11
// replies ("rgb:rrrr/gggg/bbbb"), so programs in panes can pick a light or dark
// theme. Unset means unknown, and those queries go unanswered. They arrive as
// replies while sessions may already be running, hence atomic.
var hostFg, hostBg atomic.Pointer[string]

// SetHostForeground records the host terminal's default foreground; nil
// forgets it.
func SetHostForeground(c color.Color) { hostFg.Store(oscColor(c)) }

// SetHostBackground records the host terminal's default background; nil
// forgets it.
func SetHostBackground(c color.Color) { hostBg.Store(oscColor(c)) }

func oscColor(c color.Color) *string {
	if c == nil {
		return nil
	}
	r, g, b, _ := c.RGBA()
	s := fmt.Sprintf("rgb:%04x/%04x/%04x", r, g, b)
	return &s
}

// dispatchOSC answers default-color queries; other OSCs (titles, links,
// clipboard) have no effect on the screen and go to the hook, if any.
func (t *Terminal) dispatchOSC(terminator string) {
	var reply *string
	switch string(t.osc) {
	case "10;?":
		reply = hostFg.Load()
	case "11;?":
		reply = hostBg.Load()
	default:
		if t.oscHook != nil {
			t.oscHook(string(t.osc))
		}
	}
	if reply != nil {
		t.respond("\x1b]" + string(t.osc[:2]) + ";" + *reply + terminator)
	}
}
