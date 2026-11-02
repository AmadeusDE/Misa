package android

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/gui"
)

func visibilityManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(func(be gui.Backend, _ string, _ []string) (int, error) {
		for ev := range be.Events() {
			if ev.Kind == gui.EventClose {
				return 0, nil
			}
		}
		return 0, nil
	})
	if err := m.OnCreate(nil, "/unused/misa"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		m.mu.Lock()
		ids := append([]int(nil), m.order...)
		m.mu.Unlock()
		for _, id := range ids {
			m.CloseSession(id)
		}
		waitCount(t, m, 0)
	})
	return m
}

func focusEvents(be *Backend) []bool {
	var states []bool
	for _, ev := range be.eventsLogged() {
		if ev.Kind == gui.EventFocus {
			states = append(states, ev.Focused)
		}
	}
	return states
}

func requireFocus(t *testing.T, be *Backend, want ...bool) {
	t.Helper()
	got := focusEvents(be)
	if len(got) != len(want) {
		t.Fatalf("focus events %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("focus events %v, want %v", got, want)
		}
	}
}

func TestVisibilityNeedsActivityFocusAndSurface(t *testing.T) {
	m := visibilityManager(t)
	id := m.NewSession()
	be := m.backend(id)
	requireFocus(t, be, false)
	m.OnSurfaceChanged(120, 80, 1, 0, 0, 0, 0)
	m.attach(newMemSurface(120, 80))
	if be.CanPresent() {
		t.Fatal("background Activity can present")
	}
	m.OnFocus(true)
	requireFocus(t, be, false)
	m.OnVisibility(true)
	if !be.CanPresent() {
		t.Fatal("foreground session with surface cannot present")
	}
	requireFocus(t, be, false, true)
	m.OnVisibility(true)
	m.OnFocus(true)
	requireFocus(t, be, false, true)
	m.OnFocus(false)
	requireFocus(t, be, false, true, false)
	m.OnFocus(true)
	m.OnVisibility(false)
	if be.CanPresent() {
		t.Fatal("background session still presenting")
	}
	requireFocus(t, be, false, true, false, true, false)
	m.OnFocus(false)
	m.OnVisibility(true)
	requireFocus(t, be, false, true, false, true, false)
	m.OnFocus(true)
	requireFocus(t, be, false, true, false, true, false, true)
	m.OnSurfaceDestroyed()
	if be.CanPresent() || m.SessionCount() != 1 {
		t.Fatal("surface loss did not hide and preserve session")
	}
	requireFocus(t, be, false, true, false, true, false, true, false)
	m.attach(newMemSurface(120, 80))
	requireFocus(t, be, false, true, false, true, false, true, false, true)
}

func TestVisibilitySwitchNeverFocusesHiddenSession(t *testing.T) {
	m := visibilityManager(t)
	m.OnVisibility(true)
	m.OnFocus(true)
	m.attach(newMemSurface(120, 80))
	first := m.NewSession()
	one := m.backend(first)
	requireFocus(t, one, true)
	second := m.NewSession()
	two := m.backend(second)
	requireFocus(t, one, true, false)
	requireFocus(t, two, true)
	m.SwitchSession(first)
	requireFocus(t, one, true, false, true)
	requireFocus(t, two, true, false)
	m.OnVisibility(false)
	m.SwitchSession(second)
	if one.CanPresent() || two.CanPresent() {
		t.Fatal("a hidden tab can present")
	}
	requireFocus(t, one, true, false, true, false)
	requireFocus(t, two, true, false)
	m.OnFocus(false)
	m.OnVisibility(true)
	requireFocus(t, two, true, false)
	m.OnFocus(true)
	requireFocus(t, two, true, false, true)
	m.CloseSession(second)
	requireFocus(t, two, true, false, true, false)
	requireFocus(t, one, true, false, true, false, true)
}

func TestVisibilitySurfaceAndLowMemoryInvalidateAllCaches(t *testing.T) {
	m := visibilityManager(t)
	m.OnVisibility(true)
	m.OnFocus(true)
	first := m.NewSession()
	second := m.NewSession()
	one, two := m.backend(first), m.backend(second)
	m.attach(newMemSurface(120, 80))
	if !hasKind(two.eventsLogged(), gui.EventInvalidate) {
		t.Fatal("new surface did not request reconstruction")
	}
	m.OnLowMemory()
	for _, be := range []*Backend{one, two} {
		if !hasKind(be.eventsLogged(), gui.EventInvalidate) {
			t.Fatal("low memory did not invalidate session caches")
		}
	}
	if one.CanPresent() || !two.CanPresent() {
		t.Fatal("low memory changed session visibility")
	}
}

func TestSessionIDsEnumerationDoesNotRevealHiddenTabs(t *testing.T) {
	m := visibilityManager(t)
	m.OnVisibility(true)
	m.OnFocus(true)
	m.attach(newMemSurface(120, 80))
	first, second := m.NewSession(), m.NewSession()
	one, two := m.backend(first), m.backend(second)
	beforeOne, beforeTwo := focusEvents(one), focusEvents(two)
	ids := m.SessionIDs()
	if len(ids) != 2 || ids[0] != first || ids[1] != second {
		t.Fatalf("ordered session ids %v", ids)
	}
	ids[0] = 999 // callers cannot mutate the manager's session order
	if m.SessionIDs()[0] != first || m.ActiveSession() != second {
		t.Fatal("enumeration changed the active session or order")
	}
	requireFocus(t, one, beforeOne...)
	requireFocus(t, two, beforeTwo...)
	if one.CanPresent() || !two.CanPresent() {
		t.Fatal("enumeration changed visibility")
	}
}
