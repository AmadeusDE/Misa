package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// A split session that ends while its tag isn't shown closes its pane there,
// as it would on the visible tag.
func TestSplitSessionEndingOnAHiddenTagClosesItsPane(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	a, open := splitSession(t, m)
	_, quit := m.Update(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	m.switchTag(1)
	m.addMuxPane()
	shown := m.activePane()
	pumpRoot(t, m, tea.Batch(open, quit), "pane close on the hidden tag", func() bool {
		return len(m.mux.tags[0].panes) == 1
	})
	if !a.isClosed() || len(m.detached) != 0 {
		t.Errorf("closed=%v detached=%d, want the ended session closed", a.isClosed(), len(m.detached))
	}
	if m.mux.activeTag != 1 || m.activePane() != shown || len(m.mux.panes) != 1 {
		t.Errorf("visible tag changed: tag=%d panes=%d", m.mux.activeTag, len(m.mux.panes))
	}
}
