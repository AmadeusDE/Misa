package android

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/AmadeusDE/Misa/internal/gui"
)

// Runner drives one session to completion. The production runner is
// gui.RunSession; tests substitute a fake that never opens a PTY.
type Runner func(be gui.Backend, dir string, argv []string) (int, error)

// Manager owns the terminal sessions and the one Surface they share.
// Input reaches the active session. Destroying the surface releases the
// window and leaves every child running; the next surface gets a full redraw.
type Manager struct {
	mu       sync.Mutex
	runner   Runner
	sessions map[int]*session
	order    []int
	active   int
	nextID   int
	exe      string
	dir      string
	geom     Geometry
	surf     Surface
	pointer  Pointer
	clip     *memClip
}

type session struct {
	id   int
	be   *Backend
	code int
	err  error
}

// Default is the process-wide manager JNI calls.
var Default = NewManager(nil)

// NewManager uses run for each session. A nil run is gui.RunSession.
func NewManager(run Runner) *Manager {
	if run == nil {
		run = gui.RunSession
	}
	return &Manager{
		runner:   run,
		sessions: map[int]*session{},
		nextID:   1,
		clip:     newMemClip(),
	}
}

// ParseEnv checks KEY=VALUE pairs. A missing '=' or an empty key is an error.
// Values may be empty. Later duplicates win when applied in order.
func ParseEnv(vars []string) ([][2]string, error) {
	out := make([][2]string, 0, len(vars))
	for _, kv := range vars {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("misa: bad env %q", kv)
		}
		out = append(out, [2]string{k, v})
	}
	return out, nil
}

// OnCreate stores the child executable and applies env to this process, so
// the PTY inherits HOME, TMPDIR and the rest of the APK environment (D10).
func (m *Manager) OnCreate(env []string, exe string) error {
	pairs, err := ParseEnv(env)
	if err != nil {
		return err
	}
	for _, kv := range pairs {
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exe = exe
	if home := lastEnv(pairs, "HOME"); home != "" {
		m.dir = home
	} else if m.dir == "" {
		if home := os.Getenv("HOME"); home != "" {
			m.dir = home
		} else {
			m.dir = "."
		}
	}
	return nil
}

func lastEnv(pairs [][2]string, key string) string {
	val := ""
	found := false
	for _, kv := range pairs {
		if kv[0] == key {
			val = kv[1]
			found = true
		}
	}
	if !found {
		return ""
	}
	return val
}

// OnSurfaceCreated takes ownership of an ANativeWindow (already acquired).
// A nil pointer drops the current surface.
func (m *Manager) OnSurfaceCreated(win unsafe.Pointer) {
	if win == nil {
		m.OnSurfaceDestroyed()
		return
	}
	if nativeSurface == nil {
		return
	}
	surf := nativeSurface(win)
	if surf == nil {
		return
	}
	m.attach(surf)
}

// OnSurfaceChanged records the new size, density and insets and resizes the
// active session. A content box that comes out empty (the IME covering the
// screen) keeps the previous grid.
func (m *Manager) OnSurfaceChanged(w, h int, density float64, left, top, right, bottom int) {
	g := Geometry{
		Width: w, Height: h, Density: density,
		Insets: Insets{Left: left, Top: top, Right: right, Bottom: bottom},
	}.clamped()
	m.mu.Lock()
	m.geom = g
	var be *Backend
	if s := m.sessions[m.active]; s != nil {
		be = s.be
	}
	m.mu.Unlock()
	if be != nil {
		be.pushGeom(g)
	}
}

// OnSurfaceDestroyed releases the window. Sessions keep running.
func (m *Manager) OnSurfaceDestroyed() {
	m.mu.Lock()
	surf := m.surf
	m.surf = nil
	backs := make([]*Backend, 0, len(m.sessions))
	for _, s := range m.sessions {
		backs = append(backs, s.be)
	}
	m.mu.Unlock()
	for _, b := range backs {
		b.setPresenting(false)
		b.setSurface(nil)
	}
	if surf != nil {
		surf.Release()
	}
}

// OnText delivers an IME commit to the active session. meta is the latched
// Ctrl/Alt state from the extra-keys bar; zero sends the text unchanged.
func (m *Manager) OnText(s string, meta int) {
	for _, ev := range EventsForText(s, meta) {
		m.emitActive(ev)
	}
}

// MuxAction runs one extra-keys multiplexer button. Unknown names do nothing.
func (m *Manager) MuxAction(name string) {
	switch name {
	case "zoom-in":
		m.OnPinch(ZoomIn)
	case "zoom-out":
		m.OnPinch(ZoomOut)
	default:
		for _, c := range MuxChords(name) {
			m.OnKey(c.Code, c.Meta, true)
		}
	}
}

// OnKey delivers one Android key. Releases and bare modifiers are ignored.
func (m *Manager) OnKey(code, meta int, down bool) {
	ev, ok := TranslateKey(code, meta, down)
	if !ok {
		return
	}
	m.emitActive(ev)
}

// OnMotion delivers one motion event. x and y are surface pixels, except
// ACTION_SCROLL, whose x and y are wheel steps.
func (m *Manager) OnMotion(action int, x, y float64, buttons int) {
	m.mu.Lock()
	lx, ly := x, y
	if action != ActionScroll {
		lx, ly = m.geom.LogicalPoint(x, y)
	}
	evs := m.pointer.Feed(action, lx, ly, buttons, time.Now())
	var be *Backend
	if s := m.sessions[m.active]; s != nil {
		be = s.be
	}
	m.mu.Unlock()
	if be == nil {
		return
	}
	for _, ev := range evs {
		be.emit(ev)
	}
}

// OnPinch multiplies the active session's point size by scale.
func (m *Manager) OnPinch(scale float64) {
	if scale <= 0 {
		return
	}
	m.emitActive(gui.Event{Kind: gui.EventFont, Scale: scale})
}

// OnLowMemory drops spare framebuffers and asks the active session to
// repaint, so the next frame is whole.
func (m *Manager) OnLowMemory() {
	m.mu.Lock()
	var active *Backend
	for _, s := range m.sessions {
		s.be.DropBuffers()
		if s.id == m.active {
			active = s.be
		}
	}
	g := m.geom
	m.mu.Unlock()
	if active != nil {
		active.pushGeom(g)
	}
}

// NewSession starts another child on exe and makes it active. The id is
// never reused. Zero means OnCreate has not stored an executable.
func (m *Manager) NewSession() int {
	m.mu.Lock()
	if m.exe == "" || m.runner == nil {
		m.mu.Unlock()
		return 0
	}
	id := m.nextID
	m.nextID++
	be := newBackend(m.clip)
	s := &session{id: id, be: be}
	m.sessions[id] = s
	m.order = append(m.order, id)
	if old := m.sessions[m.active]; old != nil {
		old.be.setPresenting(false)
		old.be.setSurface(nil)
	}
	m.active = id
	surf := m.surf
	g := m.geom
	dir, exe := m.dir, m.exe
	if dir == "" {
		dir = "."
	}
	run := m.runner
	m.mu.Unlock()
	be.setSurface(surf)
	be.setPresenting(surf != nil)
	be.pushGeom(g)
	go func() {
		code, err := run(be, dir, []string{exe})
		m.finish(s, code, err)
	}()
	return id
}

// CloseSession ends id. The most recently created remaining session becomes
// active. An unknown id is ignored.
func (m *Manager) CloseSession(id int) {
	m.mu.Lock()
	s := m.sessions[id]
	if s == nil {
		m.mu.Unlock()
		return
	}
	delete(m.sessions, id)
	m.order = removeID(m.order, id)
	var next *session
	if m.active == id {
		m.active = 0
		if n := len(m.order); n > 0 {
			next = m.sessions[m.order[n-1]]
			m.active = next.id
		}
	}
	surf := m.surf
	g := m.geom
	m.mu.Unlock()
	s.be.emit(gui.Event{Kind: gui.EventClose})
	if next != nil {
		next.be.setSurface(surf)
		next.be.setPresenting(surf != nil)
		next.be.pushGeom(g)
	}
}

// SwitchSession shows id. The terminal is resized to the current surface
// so a rotation that happened while it was hidden is applied.
func (m *Manager) SwitchSession(id int) {
	m.mu.Lock()
	s := m.sessions[id]
	if s == nil || m.active == id {
		m.mu.Unlock()
		return
	}
	if old := m.sessions[m.active]; old != nil {
		old.be.setPresenting(false)
		old.be.setSurface(nil)
	}
	m.active = id
	surf := m.surf
	g := m.geom
	m.mu.Unlock()
	s.be.setSurface(surf)
	s.be.setPresenting(surf != nil)
	s.be.pushGeom(g)
}

// ActiveSession is the visible session, or 0 when there is none.
func (m *Manager) ActiveSession() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// SessionCount is the number of live sessions.
func (m *Manager) SessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

func (m *Manager) attach(surf Surface) {
	m.mu.Lock()
	old := m.surf
	m.surf = surf
	var active *Backend
	for _, s := range m.sessions {
		if s.id == m.active {
			active = s.be
			continue
		}
		s.be.setPresenting(false)
		s.be.setSurface(nil)
	}
	g := m.geom
	m.mu.Unlock()
	if old != nil && old != surf {
		old.Release()
	}
	if active != nil {
		active.setSurface(surf)
		active.setPresenting(true)
		active.pushGeom(g)
	}
}

func (m *Manager) finish(s *session, code int, err error) {
	m.mu.Lock()
	s.code, s.err = code, err
	if m.sessions[s.id] != s {
		m.mu.Unlock()
		s.be.Close()
		return
	}
	delete(m.sessions, s.id)
	m.order = removeID(m.order, s.id)
	var next *Backend
	g := m.geom
	surf := m.surf
	if m.active == s.id {
		m.active = 0
		if n := len(m.order); n > 0 {
			nxt := m.sessions[m.order[n-1]]
			m.active = nxt.id
			next = nxt.be
		}
	}
	m.mu.Unlock()
	if next != nil {
		next.setSurface(surf)
		next.setPresenting(surf != nil)
		next.pushGeom(g)
	}
	s.be.Close()
}

func (m *Manager) emitActive(evs ...gui.Event) {
	m.mu.Lock()
	var be *Backend
	if s := m.sessions[m.active]; s != nil {
		be = s.be
	}
	m.mu.Unlock()
	if be == nil {
		return
	}
	for _, ev := range evs {
		be.emit(ev)
	}
}

func (m *Manager) backend(id int) *Backend {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sessions[id]; s != nil {
		return s.be
	}
	return nil
}

func removeID(ids []int, id int) []int {
	out := ids[:0]
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
