package term

// The reports a program that enabled DECSET 1004 gets: CSI I for focus in,
// CSI O for focus out.
const (
	focusReportIn  = "\x1b[I"
	focusReportOut = "\x1b[O"
)

// focusState is the host's focus, the program's mode and what it was told.
type focusState struct {
	known   bool // the host reported a focus state
	focused bool // that state
	mode    bool // the program enabled DECSET 1004
	told    bool // the program was told a state since it asked
	toldVal bool
}

// SetFocused records the host's focus and tells the program about a
// transition when it asked for focus reports.
func (t *Terminal) SetFocused(focused bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.focus.known = true
	t.focus.focused = focused
	t.reportFocus()
}

// FocusReporting reports whether the program enabled DECSET 1004.
func (t *Terminal) FocusReporting() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.focus.mode
}

// setFocusReporting applies DECSET/DECRST 1004: asking replays the cached
// state once, so a child that starts in a hidden pane pauses at once, while
// asking again changes nothing. The caller holds t.mu.
func (t *Terminal) setFocusReporting(on bool) {
	if t.focus.mode == on {
		return
	}
	t.focus.mode = on
	t.focus.told = false
	t.reportFocus()
}

// reportFocus answers a transition the program has not been told yet. A
// terminal without a program keeps owing one; active sessions use the existing
// bounded reply queue. The caller holds t.mu.
func (t *Terminal) reportFocus() {
	if t.ptm == nil {
		return
	}
	if s, ok := t.focus.next(); ok {
		t.respond(s)
	}
}

// next returns the report the program is owed, if any. Nothing is owed before
// the host reports a focus state: the terminal does not guess one.
func (f *focusState) next() (string, bool) {
	if !f.mode || !f.known || (f.told && f.toldVal == f.focused) {
		return "", false
	}
	f.told, f.toldVal = true, f.focused
	if f.focused {
		return focusReportIn, true
	}
	return focusReportOut, true
}

// reset is RIS: the program's mode and what it was told are gone, the host's
// focus state is not.
func (f *focusState) reset() {
	f.mode = false
	f.told = false
}
