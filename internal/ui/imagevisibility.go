package ui

import tea "charm.land/bubbletea/v2"

// visibilityViewer is the pause seam of an in-process screen, implemented by
// imgview.Viewer. false means hidden or unfocused: stop ticking and prefetching
// and cancel outstanding work. true means on screen and keyboard-focused: resume
// from the saved delay without restarting loop counters. A user's own pause is
// separate viewer state and survives these calls.
//
// The returned command is what restarts the clock on reveal: a hidden owner
// cancelled its pending tick, so without this command nothing would schedule the
// next one until some unrelated message reached the viewer.
type visibilityViewer interface {
	SetVisible(visible bool) tea.Cmd
}

// pauseOnlyViewer is the pause-only form of the seam. Its Update is the only
// place that arms the next clock wait, so a reveal gives it one pass and forwards
// the command it returns.
type pauseOnlyViewer interface {
	SetVisible(visible bool)
}

// viewerResumed is that pass: viewers ignore unknown messages but still return
// their pending clock and decode commands.
type viewerResumed struct{}

// viewerVisible reports whether the in-process screen is on screen and owns the
// keyboard. In the root the mux replaces the app's own screen as soon as a pane
// exists, so a pane hides it entirely. A nested pane is on screen while its tag
// is active, but pauses conservatively unless it owns the keyboard: a fair
// root-wide frame budget for several animating panes does not exist yet.
func (m *Model) viewerVisible() bool {
	if m.screen != screenProcMon || m.procViewer == nil {
		return false
	}
	if m.nestedMode {
		return m.paneShown && m.paneFocused
	}
	return m.windowFocused && m.isAppActive()
}

// syncViewerVisibility pushes a visibility change to the screen, once.
func (m *Model) syncViewerVisibility() tea.Cmd {
	visible := m.viewerVisible()
	if visible == m.procVisible {
		return nil
	}
	return m.pushViewerVisibility(visible)
}

// pushViewerVisibility reports a state even when it matches the cache: a freshly
// entered viewer assumes it is visible until told otherwise.
func (m *Model) pushViewerVisibility(visible bool) tea.Cmd {
	m.procVisible = visible
	switch v := m.procViewer.(type) {
	case visibilityViewer:
		return v.SetVisible(visible)
	case pauseOnlyViewer:
		v.SetVisible(visible)
		if visible {
			return m.procViewer.Update(viewerResumed{})
		}
	}
	return nil
}

// syncPaneVisibility hands every nested pane its place in the active tag, which
// is all a pane needs to decide whether its own screens may run. Commands a pane
// returns are routed back to that pane, never to the focused one.
func (m *Model) syncPaneVisibility() tea.Cmd {
	if m.nestedMode || !m.anyTagHasPanes() {
		return nil
	}
	active := m.activePane()
	var cmds []tea.Cmd
	for _, p := range m.allMuxPanes() {
		if p.kind != paneMisa || p.misa == nil {
			continue
		}
		p.misa.paneShown = m.tagShowsPane(p)
		p.misa.paneFocused = p == active && m.windowFocused
		if cmd := p.misa.syncViewerVisibility(); cmd != nil {
			cmds = append(cmds, routeTo(p.misa, cmd))
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) tagShowsPane(p *muxPane) bool {
	for _, q := range m.mux.panes {
		if q == p {
			return true
		}
	}
	return false
}
