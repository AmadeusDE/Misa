package imgview

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/imageframe"
	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
)

// LoopMode selects which loop count a sequence plays.
type LoopMode int

const (
	LoopFile LoopMode = iota
	LoopOnce
	LoopInfinite
)

func loopModeFromConfig(name string) LoopMode {
	switch name {
	case "once":
		return LoopOnce
	case "infinite":
		return LoopInfinite
	default:
		return LoopFile
	}
}

// PlaybackPolicy is the validated presentation policy handed to the controller.
// FPS caps the presentation rate; the sequence's own delays stay authoritative.
type PlaybackPolicy struct {
	Autoplay bool
	Loops    LoopMode
	FPS      int
}

// animationPlayback is the viewer's seam to playback.go's controller. The
// controller owns timing, loop accounting and pause/visibility state; the viewer
// owns the sequence source, bounded asynchronous frame decoding and the committed
// frame it presents. The controller is not concurrency-safe: the UI loop owns it.
type animationPlayback interface {
	Frames() int
	Frame() int
	Playing() bool
	Ended() bool
	Plays() uint64
	Loop() uint64
	Toggle()
	Step(int)
	SetVisible(bool)
	NextDelay() time.Duration
	Advance() bool
}

// playbackFactory builds the controller for one open sequence.
type playbackFactory func(*animdecode.Source, PlaybackPolicy, imageframe.Clock) animationPlayback

// newPlayback is playback.go's controller constructor, assigned by its init.
var newPlayback playbackFactory

var errPlaybackUnavailable = errors.New("playback unavailable")

// animation is the live sequence of the current item. Exactly one exists; the
// Playback controller owns timing, loops and pause/visibility state while the
// viewer owns the source, bounded decoding and the committed frame it presents.
type animation struct {
	item          int
	version       uint64
	src           *animdecode.Source
	ctrl          animationPlayback
	ctx           context.Context // sequence lifetime: cancelled when the sequence stops
	cancel        context.CancelFunc
	work          context.Context // outstanding decode/index commands
	stopWork      context.CancelFunc
	stop          chan struct{}
	gen           uint64
	armed         bool
	index         int // committed frame, -1 until the first decode lands
	frameInFlight bool
	frameGen      uint64
	closePending  bool
	visible       bool
	frame         *image.NRGBA
}

// newWork gives the next decode or index command its own child of the sequence
// context. The sequence stays live, so a hidden viewer can resume with the same
// source, controller, frame and loop counter.
func (a *animation) newWork() {
	if a.stopWork != nil {
		a.stopWork()
	}
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	a.work, a.stopWork = context.WithCancel(parent)
}

// invalidate drops outstanding ticks, decode and index commands. A codec call
// cannot be interrupted, so cancelling only stops the bounded reads and frame
// loop at their next checkpoint and the result is dropped by its generation; the
// source stays open because it is still the live sequence.
func (a *animation) invalidate() {
	if a.stop != nil {
		close(a.stop)
	}
	a.stop = make(chan struct{})
	a.gen++
	a.armed = false
	a.newWork()
}

func (a *animation) closeSource() {
	if a.src != nil {
		a.src.Close()
		a.src = nil
	}
}

func (a *animation) release() {
	if a.frameInFlight {
		a.closePending = true
		return
	}
	a.closeSource()
}

type viewerAnimation struct {
	anim *animation
	gen  uint64
	src  *animdecode.Source
	err  error
}

type viewerFrame struct {
	anim     *animation
	gen      uint64
	frame    int
	pixels   *image.NRGBA
	duration time.Duration
	err      error
}

type viewerTick struct {
	anim *animation
	gen  uint64
}

// SetVisible reports whether the host currently shows the viewer. Hidden owners
// stop timers, frame decoding and poster/thumbnail decoding; revealing resumes
// with the saved delay.
//
// A host reports true only for a screen that is on screen and owns the keyboard,
// so it also settles focus: terminal focus messages are routed to one pane at a
// time, and a viewer that was blurred and then revealed after the active pane
// changed would otherwise keep a stale unfocused flag and never resume. Hiding
// leaves focus alone; the standalone program has no host and relies on focus
// messages alone.
//
// The returned command is what restarts the work hiding cancelled: the pending
// boundary, the animation source and the poster or thumbnail decode. A host that
// only recorded the call would leave playback frozen and the image missing until
// some unrelated message reached the viewer, so the host must return it from its
// own Update.
func (m *Viewer) SetVisible(visible bool) tea.Cmd {
	m.visible = visible
	if visible {
		m.focused = true
	}
	m.syncVisibility()
	return tea.Batch(m.load(), m.ensureAnimation(), m.animCmd())
}

// screenVisible reports whether the screen may spend decode or playback work:
// on screen, keyboard-focused, not covered by help and not closing. A host
// reports visibility through SetVisible and focus through the focus messages, so
// both routes end here.
func (m *Viewer) screenVisible() bool {
	return m.visible && m.focused && !m.help && !m.quit
}

// animationVisible narrows screen visibility to the single-image view: the
// thumbnail grid always shows still posters.
func (m *Viewer) animationVisible() bool {
	return m.screenVisible() && !m.grid
}

// syncAnimation propagates viewer visibility to the controller. Grid, help and
// focus changes suspend playback without losing its position or loop counter.
func (m *Viewer) syncAnimation() {
	a := m.anim
	if a == nil {
		return
	}
	visible := m.animationVisible()
	if visible == a.visible {
		return
	}
	a.visible = visible
	if a.ctrl != nil {
		a.ctrl.SetVisible(visible)
	}
	if !visible {
		a.invalidate()
	}
}

func (m *Viewer) stopAnimation() {
	a := m.anim
	if a == nil {
		return
	}
	m.anim = nil
	a.invalidate()
	if a.ctrl != nil {
		a.ctrl.SetVisible(false)
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.release()
}

// openAnimation starts bounded indexing of the current item's sequence.
func (m *Viewer) openAnimation(index int) tea.Cmd {
	it := &m.items[index]
	ctx, cancel := context.WithCancel(context.Background())
	// a.index starts at -1: frame 0 is still a decode, and a poster (APNG's
	// separate still image) is not frame 0.
	a := &animation{item: index, version: it.version, ctx: ctx, cancel: cancel, visible: m.animationVisible(), index: -1}
	a.invalidate()
	m.anim = a
	path, limits, gen, work := it.path, m.limits, a.gen, a.work
	return func() tea.Msg {
		src, err := OpenAnimation(work, path, limits)
		return viewerAnimation{anim: a, gen: gen, src: src, err: err}
	}
}

func (m *Viewer) ensureAnimation() tea.Cmd {
	if m.anim != nil || len(m.items) == 0 || !m.animationVisible() {
		return nil
	}
	it := &m.items[m.index]
	if !it.animated || !it.loaded || it.err != nil || it.animErr != nil {
		return nil
	}
	return m.openAnimation(m.index)
}

func (m *Viewer) animationOpened(v viewerAnimation) {
	a := v.anim
	if a == nil {
		return
	}
	if m.anim != a || v.gen != a.gen {
		if v.src != nil {
			v.src.Close()
		}
		if m.anim == a {
			m.stopAnimation()
		}
		return
	}
	if v.err != nil {
		m.items[a.item].animErr = v.err
		m.note = "animation: " + safeText(v.err.Error())
		m.stopAnimation()
		return
	}
	a.src = v.src
	if m.newPlayback != nil {
		a.ctrl = m.newPlayback(a.src, m.policy, m.clock)
	}
	if a.ctrl == nil {
		m.items[a.item].animErr = errPlaybackUnavailable
		m.note = "animation: " + errPlaybackUnavailable.Error()
		m.stopAnimation()
		return
	}
	a.ctrl.SetVisible(a.visible)
}

func (m *Viewer) frameCmd(a *animation, frame int) tea.Cmd {
	src, ctx, gen, limits := a.src, a.work, a.gen, m.limits
	a.frameInFlight, a.frameGen = true, gen
	return func() tea.Msg {
		release, err := reserveAnimationWork(limits)
		if err != nil {
			return viewerFrame{anim: a, gen: gen, frame: frame, err: err}
		}
		defer release()
		decoded, err := src.DecodeFrame(ctx, frame)
		return viewerFrame{anim: a, gen: gen, frame: frame, pixels: decoded.Pixels, duration: decoded.Duration, err: err}
	}
}

func (m *Viewer) frameDecoded(v viewerFrame) {
	a := v.anim
	if a == nil {
		return
	}
	if v.gen == a.frameGen {
		a.frameInFlight = false
	}
	if a.closePending && !a.frameInFlight {
		a.closeSource()
	}
	if m.anim != a || a.ctrl == nil || v.gen != a.gen {
		return
	}
	if v.err != nil {
		if errors.Is(v.err, context.Canceled) {
			return
		}
		m.items[a.item].animErr = v.err
		m.note = "animation: " + safeText(v.err.Error())
		m.stopAnimation()
		return
	}
	if v.frame != a.ctrl.Frame() {
		return // superseded while decoding; animCmd requests the latest frame
	}
	a.frame, a.index = v.pixels, v.frame
}

func (m *Viewer) tick(v viewerTick) {
	a := v.anim
	if a == nil || m.anim != a || a.ctrl == nil {
		return
	}
	if v.gen != a.gen {
		return
	}
	a.armed = false
	if !m.animationVisible() || !a.ctrl.Playing() {
		return
	}
	a.ctrl.Advance()
}

// animCmd keeps at most one decode and one clock wait outstanding, so heavy
// frames coalesce to the latest needed one instead of queueing.
func (m *Viewer) animCmd() tea.Cmd {
	a := m.anim
	if a == nil || a.ctrl == nil || !m.animationVisible() {
		return nil
	}
	var cmds []tea.Cmd
	if !a.frameInFlight && a.src != nil && a.ctrl.Frames() > 0 && a.ctrl.Frame() != a.index {
		cmds = append(cmds, m.frameCmd(a, a.ctrl.Frame()))
	}
	// A boundary starts only once the frame it follows is on screen. Arming the
	// clock during a decode would let a slow codec skip the opening frame (or a
	// short sequence's last one) instead of merely delaying it.
	if a.ctrl.Playing() && !a.armed && a.index == a.ctrl.Frame() {
		a.armed = true
		cmds = append(cmds, m.waitCmd(a))
	}
	return tea.Batch(cmds...)
}

func (m *Viewer) waitCmd(a *animation) tea.Cmd {
	delay := a.ctrl.NextDelay()
	if delay < time.Millisecond {
		delay = time.Millisecond
	}
	stop, gen := a.stop, a.gen
	// The boundary starts when it is armed, not when the runtime schedules the
	// command, so the remaining delay stays accurate under load.
	clock := m.clock
	if clock == nil {
		clock = SystemClock{}
	}
	timer := clock.NewTimer(delay)
	return func() tea.Msg {
		select {
		case <-timer.C():
			return viewerTick{anim: a, gen: gen}
		case <-stop:
			timer.Stop()
			return nil
		}
	}
}

func (m *Viewer) toggleAnimation(prefix int) {
	if len(m.items) == 0 {
		return
	}
	if m.grid {
		m.note = "thumbnails stay still"
		return
	}
	if !m.items[m.index].animated {
		m.note = "not an animated image"
		return
	}
	if prefix != 0 {
		m.note = "numeric prefix ignored"
	} else {
		m.note = ""
	}
	a := m.anim
	if a == nil || a.ctrl == nil || a.item != m.index {
		if m.note == "" {
			m.note = "animation is loading"
		}
		return
	}
	a.ctrl.Toggle()
	a.invalidate()
}

func (m *Viewer) stepAnimation(delta int) {
	if len(m.items) == 0 || m.grid {
		return
	}
	if !m.items[m.index].animated {
		m.note = "not an animated image"
		return
	}
	a := m.anim
	if a == nil || a.ctrl == nil || a.item != m.index {
		m.note = "animation is loading"
		return
	}
	m.note = ""
	a.ctrl.Step(delta)
	a.invalidate()
}

// animationStatus reports committed frames, not merely requested decodes.
func (m *Viewer) animationStatus(it *viewerItem) string {
	if it.animErr != nil {
		return " · animation: " + safeText(it.animErr.Error())
	}
	a := m.anim
	if a == nil || a.ctrl == nil || a.item != m.index {
		return " · animated"
	}
	frames := max(1, a.ctrl.Frames())
	if a.index < 0 {
		// The poster is on screen and no frame has been committed yet, so no
		// frame number may be claimed here.
		return fmt.Sprintf(" · %d frames · loading", frames)
	}
	label := "paused"
	switch {
	case a.ctrl.Playing():
		label = "playing"
	case a.ctrl.Ended():
		label = "ended"
	}
	status := fmt.Sprintf(" · frame %d/%d · %s", a.index+1, frames, label)
	if plays := a.ctrl.Plays(); plays == 0 {
		status += " · loop ∞"
	} else if loop := a.ctrl.Loop(); loop > 0 {
		status += fmt.Sprintf(" · loop %d/%d", loop, plays)
	}
	return status
}
