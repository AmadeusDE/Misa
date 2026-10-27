package ui

import tea "charm.land/bubbletea/v2"

// muxFrame keeps the last root frame. Bubble Tea renders after every event,
// and with hover tracking most events are pointer moves that change nothing
// on screen, so rebuilding every pane for them would burn CPU.
type muxFrame struct {
	view  string
	reuse bool
}

func (f *muxFrame) render(build func() string) string {
	if !f.reuse || f.view == "" {
		f.view = build()
	}
	return f.view
}

func (m *Model) muxSelectionActive() bool {
	if m.sel.active {
		return true
	}
	for _, p := range m.mux.panes {
		if p.misa != nil && p.misa.sel.active {
			return true
		}
	}
	return false
}

// isInertHover reports a pointer move without buttons. Without an active
// selection, it leaves the screen unchanged unless it moves focus.
func isInertHover(msg tea.Msg) bool {
	mouse, ok := msg.(tea.MouseMsg)
	return ok && isHover(fromTea(mouse))
}
