package android

import "github.com/AmadeusDE/Misa/internal/gui"

// OnVisibility reports Activity onStart/onStop independently of keyboard focus.
func (m *Manager) OnVisibility(visible bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.visible = visible
	m.syncPresentationLocked(false)
}

// OnFocus reports actual Android window focus; foregrounding alone cannot grant it.
func (m *Manager) OnFocus(focused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.focused = focused
	m.syncPresentationLocked(false)
}

func (m *Manager) focusLocked(s *session, focused bool) {
	if s.focusKnown && s.focused == focused {
		return
	}
	s.focusKnown, s.focused = true, focused
	s.be.emit(gui.Event{Kind: gui.EventFocus, Focused: focused})
}

func (m *Manager) hideLocked(s *session) {
	s.be.setPresenting(false)
	s.be.setSurface(nil)
	s.shown = false
	m.focusLocked(s, false)
}

func (m *Manager) syncPresentationLocked(invalidate bool) {
	// Blur every old session before focusing the newly current session.
	for _, s := range m.sessions {
		if s.id != m.active || !m.visible || m.surf == nil {
			m.hideLocked(s)
		}
	}
	if s := m.sessions[m.active]; s != nil && m.visible && m.surf != nil {
		redraw := !s.shown || invalidate
		s.be.setSurface(m.surf)
		s.be.setPresenting(true)
		s.shown = true
		// Reconstruct before the focus change: a session whose caches were
		// emptied while hidden would otherwise draw a whole frame on the
		// focus event and a second, identical one on this invalidate.
		if redraw {
			s.be.emit(gui.Event{Kind: gui.EventInvalidate})
			s.be.pushGeom(m.geom)
		}
		m.focusLocked(s, m.focused)
	}
}

func (m *Manager) attach(surf Surface) {
	m.mu.Lock()
	old := m.surf
	m.surf = surf
	m.syncPresentationLocked(true)
	m.mu.Unlock()
	if old != nil && old != surf {
		old.Release()
	}
}

// OnSurfaceDestroyed releases the window, not the live terminal or its image store.
func (m *Manager) OnSurfaceDestroyed() {
	m.mu.Lock()
	surf := m.surf
	m.surf = nil
	m.syncPresentationLocked(false)
	m.mu.Unlock()
	if surf != nil {
		surf.Release()
	}
}

// OnLowMemory evicts reconstructible caches on each session's GUI loop. Hidden
// sessions defer reconstruction until they have a visible surface again.
func (m *Manager) OnLowMemory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		s.be.emit(gui.Event{Kind: gui.EventInvalidate})
	}
}
