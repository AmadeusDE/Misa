package android

// Font zoom from the extra-keys bar. Pinch uses the same event.
const (
	ZoomIn  = 1.15
	ZoomOut = 1 / 1.15
)

// Chord is one Android key of a multiplexer button.
type Chord struct {
	Code int
	Meta int
}

// MuxChords is the key sequence for an extra-keys multiplexer button.
// Names match the Java bar. The sequences are the bindings in internal/ui:
// ctrl+w is the prefix, then the command key; copy and paste are ctrl+y and
// ctrl+v; tags are alt+1 through alt+0. There is no separate horizontal or
// vertical split, and no pane-fullscreen binding: split is ctrl+w v, and
// zoom is the GUI font scale (zoom-in / zoom-out), not a chord.
func MuxChords(action string) []Chord {
	chords, ok := muxChords[action]
	if !ok {
		return nil
	}
	out := make([]Chord, len(chords))
	copy(out, chords)
	return out
}

func letter(c rune) int { return keyA + int(c-'a') }

var muxChords = map[string][]Chord{
	"prefix":   {{letter('w'), MetaCtrlOn}},
	"split":    {{letter('w'), MetaCtrlOn}, {letter('v'), 0}},
	"next":     {{letter('w'), MetaCtrlOn}, {letter('w'), 0}},
	"left":     {{letter('w'), MetaCtrlOn}, {letter('h'), 0}},
	"down":     {{letter('w'), MetaCtrlOn}, {letter('j'), 0}},
	"up":       {{letter('w'), MetaCtrlOn}, {letter('k'), 0}},
	"right":    {{letter('w'), MetaCtrlOn}, {letter('l'), 0}},
	"close":    {{letter('w'), MetaCtrlOn}, {letter('q'), 0}},
	"layout":   {{letter('w'), MetaCtrlOn}, {keySpace, 0}},
	"resize":   {{letter('w'), MetaCtrlOn}, {letter('r'), 0}},
	"agent":    {{letter('w'), MetaCtrlOn}, {letter('a'), 0}},
	"reattach": {{letter('w'), MetaCtrlOn}, {letter('a'), MetaShiftOn}},
	"copy":     {{letter('y'), MetaCtrlOn}},
	"paste":    {{letter('v'), MetaCtrlOn}},
	"tag1":     {{key0 + 1, MetaAltOn}},
	"tag2":     {{key0 + 2, MetaAltOn}},
	"tag3":     {{key0 + 3, MetaAltOn}},
	"tag4":     {{key0 + 4, MetaAltOn}},
	"tag5":     {{key0 + 5, MetaAltOn}},
	"tag6":     {{key0 + 6, MetaAltOn}},
	"tag7":     {{key0 + 7, MetaAltOn}},
	"tag8":     {{key0 + 8, MetaAltOn}},
	"tag9":     {{key0 + 9, MetaAltOn}},
	"tag0":     {{key0, MetaAltOn}},
}
