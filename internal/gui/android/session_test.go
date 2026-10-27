package android

import (
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/gui"
)

type gate struct {
	started  chan struct{}
	finished chan struct{}
	dir      string
	argv     []string
}

func newGate() *gate {
	return &gate{started: make(chan struct{}), finished: make(chan struct{})}
}

func (g *gate) run(be gui.Backend, dir string, argv []string) (int, error) {
	g.dir = dir
	g.argv = append([]string(nil), argv...)
	close(g.started)
	defer close(g.finished)
	for ev := range be.Events() {
		if ev.Kind == gui.EventClose {
			return 0, nil
		}
	}
	return 0, nil
}

func waitCh(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func restoreEnv(t *testing.T, key string) {
	t.Helper()
	old, ok := os.LookupEnv(key)
	t.Cleanup(func() {
		if ok {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestParseEnvAndClassName(t *testing.T) {
	pairs, err := ParseEnv([]string{"HOME=/data/home", "EMPTY=", "PATH=/bin"})
	if err != nil || len(pairs) != 3 || pairs[0] != [2]string{"HOME", "/data/home"} || pairs[1][1] != "" {
		t.Fatalf("%v %v", pairs, err)
	}
	if _, err := ParseEnv([]string{"novalue"}); err == nil {
		t.Fatal("accepted a pair with no equals")
	}
	if _, err := ParseEnv([]string{"=x"}); err == nil {
		t.Fatal("accepted an empty key")
	}
	if JNIClass != "app/misa/Native" || JNIClassName() != JNIClass {
		t.Fatalf("class %q", JNIClassName())
	}
	t.Setenv("MISA_JNI_CLASS", "app/example/Native")
	if JNIClassName() != "app/example/Native" {
		t.Fatalf("override %q", JNIClassName())
	}
}

func TestSessionsSurviveSurfaceLoss(t *testing.T) {
	restoreEnv(t, "HOME")
	g := newGate()
	m := NewManager(g.run)
	if m.NewSession() != 0 {
		t.Fatal("session without an executable")
	}
	if err := m.OnCreate([]string{"HOME=/data/user/0/app/home"}, "/data/app/lib/arm64/libmisa.so"); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("HOME") != "/data/user/0/app/home" {
		t.Fatalf("HOME %q", os.Getenv("HOME"))
	}
	id := m.NewSession()
	waitCh(t, g.started)
	if id == 0 || m.SessionCount() != 1 || m.ActiveSession() != id {
		t.Fatalf("id %d count %d active %d", id, m.SessionCount(), m.ActiveSession())
	}
	if g.argv[0] != "/data/app/lib/arm64/libmisa.so" || g.dir != "/data/user/0/app/home" {
		t.Fatalf("argv %q dir %q", g.argv, g.dir)
	}

	surf := newMemSurface(1080, 2400)
	m.attach(surf)
	m.OnSurfaceChanged(1080, 2400, 3, 0, 80, 0, 120)
	be := m.backend(id)
	if !be.CanPresent() {
		t.Fatal("active session cannot present")
	}
	ev := lastOf(be.eventsLogged(), gui.EventResize)
	if ev.Width != 360 || ev.Height != 733 || ev.Scale != 3 {
		t.Fatalf("resize %+v", ev)
	}

	m.OnSurfaceDestroyed()
	select {
	case <-g.finished:
		t.Fatal("surface loss ended the session")
	default:
	}
	if be.CanPresent() || m.SessionCount() != 1 || m.ActiveSession() != id {
		t.Fatalf("present %v count %d active %d", be.CanPresent(), m.SessionCount(), m.ActiveSession())
	}

	m.attach(newMemSurface(1080, 2000))
	m.OnSurfaceChanged(1080, 2000, 3, 0, 80, 0, 120)
	if !be.CanPresent() || m.SessionCount() != 1 {
		t.Fatal("reattach lost the session")
	}
	ev = lastOf(be.eventsLogged(), gui.EventResize)
	if ev.Height != 600 {
		t.Fatalf("rotation %+v", ev)
	}

	m.OnText("hi")
	m.OnKey(keyA, MetaAltOn, true)
	m.OnPinch(1.25)
	m.OnPinch(0)
	m.OnMotion(ActionDown, 30, 86, 0)
	m.OnMotion(ActionUp, 30, 86, 0)
	logged := be.eventsLogged()
	if lastOf(logged, gui.EventText).Text != "hi" {
		t.Fatal("text")
	}
	key := lastOf(logged, gui.EventKey)
	if key.Key.Code != 'a' || key.Key.Mod&tea.ModAlt == 0 {
		t.Fatalf("key %+v", key.Key)
	}
	if lastOf(logged, gui.EventFont).Scale != 1.25 {
		t.Fatal("pinch")
	}
	down := lastOf(logged, gui.EventPointer)
	if down.X != 10 || down.Y != 2 || down.Action != gui.PointerUp {
		t.Fatalf("pointer %+v", down)
	}

	before := be.bufferID()
	m.OnLowMemory()
	if be.bufferID() == before {
		t.Fatal("low memory kept the framebuffer id")
	}

	m.CloseSession(id)
	waitCh(t, g.finished)
	if m.SessionCount() != 0 || m.ActiveSession() != 0 {
		t.Fatalf("count %d active %d", m.SessionCount(), m.ActiveSession())
	}
}

func TestSessionSwitchRoutesInput(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	m := NewManager(func(be gui.Backend, dir string, argv []string) (int, error) {
		<-release
		return 0, nil
	})
	if err := m.OnCreate(nil, "/bin/misa"); err != nil {
		t.Fatal(err)
	}
	first := m.NewSession()
	second := m.NewSession()
	if m.ActiveSession() != second || m.SessionCount() != 2 || first == second {
		t.Fatalf("active %d first %d second %d", m.ActiveSession(), first, second)
	}
	m.OnKey(keyA, 0, true)
	if hasKind(m.backend(first).eventsLogged(), gui.EventKey) {
		t.Fatal("hidden session received the key")
	}
	if lastOf(m.backend(second).eventsLogged(), gui.EventKey).Key.Code != 'a' {
		t.Fatal("active session missed the key")
	}
	m.SwitchSession(first)
	m.OnKey(keyEnter, 0, true)
	if lastOf(m.backend(first).eventsLogged(), gui.EventKey).Key.Code != tea.KeyEnter {
		t.Fatal("switch did not move input")
	}
	m.CloseSession(second)
	if m.ActiveSession() != first || m.SessionCount() != 1 {
		t.Fatalf("after close active %d count %d", m.ActiveSession(), m.SessionCount())
	}
	m.CloseSession(first)
	waitCount(t, m, 0)
}

func TestSessionEndsWhenProgramExits(t *testing.T) {
	release := make(chan struct{})
	m := NewManager(func(be gui.Backend, dir string, argv []string) (int, error) {
		<-release
		return 7, nil
	})
	if err := m.OnCreate(nil, "/bin/true"); err != nil {
		t.Fatal(err)
	}
	id := m.NewSession()
	close(release)
	waitCount(t, m, 0)
	if m.ActiveSession() != 0 || m.backend(id) != nil {
		t.Fatal("exited session stayed active")
	}
}

func waitCount(t *testing.T, m *Manager, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.SessionCount() == n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("count %d, want %d", m.SessionCount(), n)
}

func lastOf(evs []gui.Event, kind gui.EventKind) gui.Event {
	var found gui.Event
	for _, ev := range evs {
		if ev.Kind == kind {
			found = ev
		}
	}
	return found
}

func hasKind(evs []gui.Event, kind gui.EventKind) bool {
	for _, ev := range evs {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}
