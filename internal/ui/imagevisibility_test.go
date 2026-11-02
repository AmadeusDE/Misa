package ui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/procmon"
)

var (
	_ procmon.Viewer = (*stubViewer)(nil)
	_ procmon.Viewer = (*trackingViewer)(nil)
)

// stubViewer is an in-process screen without the pause seam.
type stubViewer struct{ quit bool }

func (s *stubViewer) Init() tea.Cmd          { return nil }
func (s *stubViewer) Update(tea.Msg) tea.Cmd { return nil }
func (s *stubViewer) View() string           { return "" }
func (s *stubViewer) Resize(int, int)        {}
func (s *stubViewer) IsQuit() bool           { return s.quit }

// trackingViewer is a pause-only viewer that arms a clock on any Update pass.
type trackingViewer struct {
	stubViewer
	states  []bool
	updates int
}

func (t *trackingViewer) SetVisible(visible bool) { t.states = append(t.states, visible) }

func (t *trackingViewer) Update(tea.Msg) tea.Cmd {
	t.updates++
	return func() tea.Msg { return resumeMsg{} }
}

// resumeMsg is what a viewer returns from SetVisible(true) to restart its clock.
type resumeMsg struct{}

// viewerTickMsg is the tick a resumed viewer schedules.
type viewerTickMsg struct{}

// resumingViewer implements the command-returning seam.
type resumingViewer struct {
	stubViewer
	states  []bool
	updates int
}

func (r *resumingViewer) SetVisible(visible bool) tea.Cmd {
	r.states = append(r.states, visible)
	if !visible {
		return nil
	}
	return func() tea.Msg { return resumeMsg{} }
}

func (r *resumingViewer) Update(msg tea.Msg) tea.Cmd {
	r.updates++
	if _, ok := msg.(resumeMsg); ok {
		return func() tea.Msg { return viewerTickMsg{} }
	}
	return nil
}

// focusViewer models the real viewer's contract: focus messages set its own flag,
// a visible push from the host settles it again, and it runs only while the host
// shows it and it believes it is focused.
type focusViewer struct {
	stubViewer
	focused bool
	visible bool
}

func (f *focusViewer) SetVisible(visible bool) tea.Cmd {
	f.visible = visible
	if visible {
		f.focused = true
	}
	return nil
}

func (f *focusViewer) running() bool { return f.visible && f.focused }

func (f *focusViewer) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tea.FocusMsg:
		f.focused = true
	case tea.BlurMsg:
		f.focused = false
	}
	return nil
}

func run(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command")
	}
	return cmd()
}

func wantVisibility(t *testing.T, v *trackingViewer, want ...bool) {
	t.Helper()
	if !reflect.DeepEqual(v.states, want) {
		t.Fatalf("visibility pushes = %v, want %v", v.states, want)
	}
}

func TestViewerVisibilityPushedOnceOnEnter(t *testing.T) {
	m := newRoot(t, 80, 24)
	v := &trackingViewer{}
	m.enterProcMon(v)
	wantVisibility(t, v, true)

	m.Update(tickMsg{})
	m.Update(tickMsg{})
	wantVisibility(t, v, true)
}

func TestViewerPausesWhileAnotherScreenCovers(t *testing.T) {
	m := newRoot(t, 80, 24)
	v := &trackingViewer{}
	m.enterProcMon(v)

	m.screen = screenHelp
	m.Update(tickMsg{})
	wantVisibility(t, v, true, false)

	m.screen = screenProcMon
	m.Update(tickMsg{})
	wantVisibility(t, v, true, false, true)
}

func TestRootViewerHiddenByMuxPanes(t *testing.T) {
	m := newRoot(t, 100, 30)
	v := &trackingViewer{}
	m.enterProcMon(v)
	wantVisibility(t, v, true)

	m.addMuxPane()
	m.Update(tickMsg{})
	wantVisibility(t, v, true, false)
}

func TestViewerPausesWhenTerminalUnfocused(t *testing.T) {
	m := newRoot(t, 80, 24)
	v := &trackingViewer{}
	m.enterProcMon(v)

	m.Update(tea.BlurMsg{})
	wantVisibility(t, v, true, false)

	m.Update(tea.FocusMsg{})
	wantVisibility(t, v, true, false, true)
}

func TestNestedViewerFollowsPaneFocusAndTag(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	p := m.activePane()
	v := &trackingViewer{}
	p.misa.enterProcMon(v)
	m.Update(tickMsg{})
	wantVisibility(t, v, true)

	m.addMuxPane()
	m.Update(tickMsg{})
	wantVisibility(t, v, true, false)

	m.mux.activeIdx = 0
	m.Update(tickMsg{})
	wantVisibility(t, v, true, false, true)

	m.switchTag(1)
	m.Update(tickMsg{})
	wantVisibility(t, v, true, false, true, false)

	m.switchTag(0)
	m.Update(tickMsg{})
	wantVisibility(t, v, true, false, true, false, true)
}

func TestNestedViewerPausesWhileTerminalUnfocused(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	v := &trackingViewer{}
	m.activePane().misa.enterProcMon(v)
	m.Update(tickMsg{})
	wantVisibility(t, v, true)

	m.Update(tea.BlurMsg{})
	wantVisibility(t, v, true, false)

	m.Update(tea.FocusMsg{})
	wantVisibility(t, v, true, false, true)
}

func TestViewerEnteredWhileUnfocusedStartsPaused(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	first := m.activePane()
	m.addMuxPane()
	m.Update(tickMsg{})

	nested := &trackingViewer{}
	first.misa.enterProcMon(nested)
	wantVisibility(t, nested, false)

	root := &trackingViewer{}
	m.enterProcMon(root)
	wantVisibility(t, root, false)

	m.mux.activeIdx = 0
	m.Update(tickMsg{})
	wantVisibility(t, nested, false, true)
	wantVisibility(t, root, false)
}

func TestNestedViewerReentryPushesVisibleAgain(t *testing.T) {
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	nested := m.activePane().misa
	first := &trackingViewer{}
	nested.enterProcMon(first)
	first.quit = true
	m.Update(tea.KeyPressMsg{Code: 'q'})
	if nested.procViewer != nil || nested.screen == screenProcMon {
		t.Fatal("quit viewer should leave the screen")
	}

	second := &trackingViewer{}
	nested.enterProcMon(second)
	wantVisibility(t, second, true)
}

func TestPauseOnlyRevealGivesViewerAnUpdatePass(t *testing.T) {
	m := newRoot(t, 80, 24)
	v := &trackingViewer{}
	m.enterProcMon(v)
	m.Update(tea.BlurMsg{})
	before := v.updates

	_, cmd := m.Update(tea.FocusMsg{})
	if msg := run(t, cmd); msg != (resumeMsg{}) {
		t.Fatalf("reveal command produced %T, want the viewer's own command", msg)
	}
	if v.updates != before+1 {
		t.Fatalf("reveal update passes = %d, want %d", v.updates, before+1)
	}
	wantVisibility(t, v, true, false, true)
}

func TestRevealRestartsViewerClock(t *testing.T) {
	m := newRoot(t, 80, 24)
	v := &resumingViewer{}
	m.enterProcMon(v)
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(tea.FocusMsg{})
	if msg := run(t, cmd); msg != (resumeMsg{}) {
		t.Fatalf("reveal command produced %T, want the resume message", msg)
	}
	if !reflect.DeepEqual(v.states, []bool{true, false, true}) {
		t.Fatalf("visibility pushes = %v", v.states)
	}
	if v.updates != 0 {
		t.Fatalf("a viewer returning its command needs no update pass, got %d", v.updates)
	}
}

func TestNestedRevealRoutesResumeCommandToPane(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	p := m.activePane()
	v := &resumingViewer{}
	p.misa.enterProcMon(v)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	m.addMuxPane()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !reflect.DeepEqual(v.states, []bool{true, false}) {
		t.Fatalf("pane viewer should pause when the pane loses focus: %v", v.states)
	}

	m.mux.activeIdx = 0
	_, cmd := m.Update(tea.FocusMsg{})
	msg := run(t, cmd)
	pane, ok := msg.(paneMsg)
	if !ok || pane.owner != p.misa || pane.msg != (resumeMsg{}) {
		t.Fatalf("reveal command produced %#v, want a resume routed to the pane", msg)
	}

	_, next := m.Update(pane)
	tick, ok := run(t, next).(paneMsg)
	if !ok || tick.owner != p.misa || tick.msg != (viewerTickMsg{}) {
		t.Fatalf("resumed tick produced %#v, want a tick routed to the pane", tick)
	}
}

func TestPauseOnlyViewerStillGetsVisibility(t *testing.T) {
	m := newRoot(t, 80, 24)
	v := &trackingViewer{}
	m.enterProcMon(v)
	if _, cmd := m.Update(tea.BlurMsg{}); cmd != nil {
		t.Fatal("a pause-only viewer must not produce a command")
	}
	wantVisibility(t, v, true, false)
}

func TestViewerWithoutVisibilitySeamIsLeftAlone(t *testing.T) {
	m := newRoot(t, 80, 24)
	m.enterProcMon(&stubViewer{})
	m.Update(tickMsg{})
	m.Update(tea.BlurMsg{})
	if m.procViewer == nil || m.screen != screenProcMon {
		t.Fatal("screen without the pause seam must stay up")
	}
}

// TestRealImageViewerUsesTheResumingSeam guards the resume path: a viewer that
// only paused would leave playback frozen after a reveal until some unrelated
// message happened to reach it.
func TestRealImageViewerUsesTheResumingSeam(t *testing.T) {
	forceImageViewerFallback(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newRoot(t, 80, 24)
	if ok, _ := m.handleBuiltinCmd("imgview a.png"); !ok {
		t.Fatal("did not enter image viewer")
	}
	if _, ok := m.procViewer.(visibilityViewer); !ok {
		t.Fatalf("image viewer only pauses: %T returns no resume command", m.procViewer)
	}
}

// TestPaneRegainingFocusAfterBlurIsRevealed covers a viewer whose own focus flag
// went stale: the blur reached only the then-active pane, so the pane revealed
// later has to be told it may run again.
func TestPaneRegainingFocusAfterBlurIsRevealed(t *testing.T) {
	m := newRoot(t, 100, 30)
	m.addMuxPane()
	first := m.activePane()
	v1 := &focusViewer{focused: true}
	first.misa.enterProcMon(v1)
	m.Update(tickMsg{})
	if !v1.running() {
		t.Fatal("active pane viewer is not running")
	}

	// The window blurs while the first pane owns the keyboard: it is the only
	// pane that sees the blur.
	m.Update(tea.BlurMsg{})
	if v1.focused || v1.running() {
		t.Fatal("blurred pane viewer kept running")
	}

	// The active pane changes while the window is still blurred, so the new pane
	// is the one that receives focus when the window comes back.
	m.addMuxPane()
	v2 := &focusViewer{focused: true}
	m.activePane().misa.enterProcMon(v2)
	m.Update(tickMsg{})
	if v2.running() {
		t.Fatal("pane viewer ran while the window was blurred")
	}
	m.Update(tea.FocusMsg{})
	if !v2.running() {
		t.Fatal("focused pane viewer did not resume")
	}

	// Back to the first pane: its own focus flag is stale, so only the reveal
	// can settle it.
	m.mux.activeIdx = 0
	m.Update(tickMsg{})
	if !v1.running() {
		t.Fatalf("pane revealed after a stale blur is stuck paused: %+v", v1)
	}
	if v2.visible {
		t.Fatal("hidden pane viewer kept running")
	}
}

func TestImageViewerVisibilityWithRealViewer(t *testing.T) {
	forceImageViewerFallback(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := newRoot(t, 80, 24)
	if ok, _ := m.handleBuiltinCmd("imgview a.png"); !ok {
		t.Fatal("did not enter image viewer")
	}
	m.Update(tea.BlurMsg{})
	m.Update(tea.FocusMsg{})
	m.screen = screenHelp
	m.Update(tickMsg{})
	if m.procViewer == nil {
		t.Fatal("real viewer left the screen")
	}
}
