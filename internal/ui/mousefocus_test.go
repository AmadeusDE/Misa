package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
)

// twoPaneRoot returns a root with two side-by-side panes, the second focused,
// and a screen point inside the first.
func twoPaneRoot(t *testing.T) (m *Model, first *muxPane, x, y int) {
	t.Helper()
	m = newRoot(t, 100, 30)
	m.addMuxPane()
	first = m.activePane()
	m.addMuxPane()
	leaf := m.leafOf(first)
	if leaf == nil {
		t.Fatal("first pane has no leaf")
	}
	dx, dy := m.muxContentOffset()
	return m, first, dx + leaf.x + 1, dy + leaf.y + 1
}

func hoverAt(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseNone}
}

func TestHoverFocusesPaneUnderPointer(t *testing.T) {
	m, first, x, y := twoPaneRoot(t)
	m.Update(hoverAt(x, y))
	if m.activePane() != first {
		t.Error("hover did not focus the pane under the pointer")
	}
}

func TestHoverFocusCanBeTurnedOff(t *testing.T) {
	m, first, x, y := twoPaneRoot(t)
	m.muxFocusFollowsMouse = false
	m.Update(hoverAt(x, y))
	if m.activePane() == first {
		t.Error("hover moved focus with focus_follows_mouse off")
	}
}

func TestHoverGoesToPaneUnderPointer(t *testing.T) {
	m, first, x, y := twoPaneRoot(t)
	m.muxFocusFollowsMouse = false
	dx, dy := m.muxContentOffset()
	if got := m.mouseLeaf(fromTea(hoverAt(x, y)), x-dx, y-dy); got == nil || got.pane != first {
		t.Error("hover was not routed to the pane under the pointer")
	}
}

func TestDragDoesNotMoveFocus(t *testing.T) {
	m, first, x, y := twoPaneRoot(t)
	m.Update(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
	if m.activePane() == first {
		t.Error("a drag across panes moved focus")
	}
}

func TestMuxConfigSavedInPaneReachesRoot(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	p := m.activePane()
	cfg := config.MuxConfig{BarPosition: "top", FocusFollowsMouse: config.BoolPtr(false)}
	m.Update(paneMsg{owner: p.misa, msg: muxConfigChanged{cfg}})
	if m.muxBarPosition != "top" || m.muxFocusFollowsMouse {
		t.Errorf("root bar=%q follow=%v, want top/false", m.muxBarPosition, m.muxFocusFollowsMouse)
	}
}

func (m *Model) leafOf(p *muxPane) *muxNode {
	for _, n := range allLeaves(m.mux.tree) {
		if n.pane == p {
			return n
		}
	}
	return nil
}

func TestHoverFocusChangeRedraws(t *testing.T) {
	m, _, x, y := twoPaneRoot(t)
	m.frame.view = "cached"
	m.Update(hoverAt(x, y))
	if m.frame.render(func() string { return "fresh" }) != "fresh" {
		t.Error("focus moved by hover but the frame was reused")
	}
}

func TestInertHoverReusesFrame(t *testing.T) {
	m, _, x, y := twoPaneRoot(t)
	m.Update(hoverAt(x, y))
	m.View()
	builds := 0
	m.frame.view = "cached"
	m.Update(hoverAt(x+1, y))
	if got := m.frame.render(func() string { builds++; return "fresh" }); got != "cached" || builds != 0 {
		t.Errorf("inert hover rebuilt the frame (%q, %d builds)", got, builds)
	}
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if got := m.frame.render(func() string { return "fresh" }); got != "fresh" {
		t.Error("frame reused after a key press")
	}
}
