package imgview

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imageframe"
	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
	"github.com/charmbracelet/x/ansi"
)

// fakeClock records requested delays and fires immediately so viewer tests can
// drive playback without real time passing.
type fakeClock struct{ delays []time.Duration }

func (c *fakeClock) Now() time.Time { return time.Unix(0, 0) }
func (c *fakeClock) NewTimer(d time.Duration) imageframe.Timer {
	c.delays = append(c.delays, d)
	return immediateTimer{}
}

type immediateTimer struct{}

func (immediateTimer) C() <-chan time.Time {
	ch := make(chan time.Time)
	close(ch)
	return ch
}
func (immediateTimer) Stop() bool { return false }

// fakePlayback is a scriptable animationPlayback: it isolates viewer
// integration from the real controller's timing and loop semantics.
type fakePlayback struct {
	frames, frame     int
	playing, ended    bool
	plays, loop       uint64
	visible           bool
	delay             time.Duration
	limit             int
	advances, toggles int
	steps             []int
	visibility        []bool
	policy            PlaybackPolicy
	clock             imageframe.Clock
}

func (f *fakePlayback) Frames() int              { return f.frames }
func (f *fakePlayback) Frame() int               { return f.frame }
func (f *fakePlayback) Playing() bool            { return f.playing }
func (f *fakePlayback) Ended() bool              { return f.ended }
func (f *fakePlayback) Plays() uint64            { return f.plays }
func (f *fakePlayback) Loop() uint64             { return f.loop }
func (f *fakePlayback) NextDelay() time.Duration { return f.delay }
func (f *fakePlayback) SetVisible(v bool) {
	f.visible = v
	f.visibility = append(f.visibility, v)
}
func (f *fakePlayback) Toggle() {
	f.toggles++
	if f.ended {
		f.ended, f.playing, f.frame = false, true, 0
		return
	}
	f.playing = !f.playing
}
func (f *fakePlayback) Step(delta int) {
	f.steps = append(f.steps, delta)
	f.playing = false
	f.frame = max(0, min(f.frames-1, f.frame+delta))
}
func (f *fakePlayback) Advance() bool {
	if !f.playing || f.frames == 0 {
		return false
	}
	f.advances++
	f.frame++
	if f.frame >= f.frames {
		f.frame, f.loop = 0, f.loop+1
	}
	if f.limit > 0 && f.advances >= f.limit {
		f.playing, f.ended = false, true
	}
	return true
}

func ctrlKey(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl} }

// writeAnimatedGIF writes a frames-long GIF. loop is the GIF loop count: -1
// plays once, 0 is infinite and n means n+1 plays.
func writeAnimatedGIF(t *testing.T, frames, loop int) string {
	t.Helper()
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{LoopCount: loop, Config: image.Config{ColorModel: pal, Width: 2, Height: 2}}
	for i := 0; i < frames; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
		for j := range frame.Pix {
			frame.Pix[j] = uint8(i % 2)
		}
		g.Image = append(g.Image, frame)
		g.Delay = append(g.Delay, 2)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "anim.gif")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// animatedViewer opens a real sequence with a fake controller and autoplay off.
func animatedViewer(t *testing.T, frames, loop int, fake *fakePlayback) (*Viewer, *fakeClock) {
	t.Helper()
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = false
	v, err := NewConfigured([]string{writeAnimatedGIF(t, frames, loop)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := &fakeClock{}
	v.clock = clock
	v.newPlayback = func(src *animdecode.Source, p PlaybackPolicy, c imageframe.Clock) animationPlayback {
		meta := src.Metadata()
		fake.frames, fake.plays = meta.Frames, meta.Plays
		fake.policy, fake.clock = p, c
		fake.playing = p.Autoplay
		if fake.delay == 0 {
			fake.delay = 20 * time.Millisecond
		}
		return fake
	}
	settle(t, v, v.Init(), 64)
	if v.anim == nil || v.anim.ctrl == nil {
		t.Fatalf("animation not attached: item %+v", v.items[0])
	}
	return v, clock
}

// settle runs commands to completion, including batched ones.
func settle(t *testing.T, v *Viewer, cmd tea.Cmd, max int) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; i < max && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg == nil {
			continue
		}
		if next := v.Update(msg); next != nil {
			queue = append(queue, next)
		}
	}
}

func TestAnimationPolicyFromConfig(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = false
	cfg.AnimationLoops = "once"
	cfg.HalfblockAnimationFPS = 7
	v, err := NewConfigured([]string{"one"}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if v.policy.Autoplay || v.policy.Loops != LoopOnce || v.policy.FPS != 7 {
		t.Fatalf("policy %+v", v.policy)
	}
	if loopModeFromConfig("infinite") != LoopInfinite || loopModeFromConfig("file") != LoopFile {
		t.Fatal("loop mode mapping")
	}
	if v.newPlayback == nil {
		t.Fatal("no playback controller is wired")
	}
}

func TestAnimationKeysToggleAndStep(t *testing.T) {
	fake := &fakePlayback{}
	v, clock := animatedViewer(t, 4, 0, fake)

	armed := len(clock.delays)
	v.Update(ctrlKey('a'))
	if fake.toggles != 1 || !fake.playing {
		t.Fatalf("ctrl+a did not play: %+v", fake)
	}
	if len(clock.delays) == armed {
		t.Fatal("playing did not arm a clock timer")
	}
	v.Update(ctrlKey('a'))
	if fake.toggles != 2 || fake.playing {
		t.Fatal("second ctrl+a did not pause")
	}

	v.Update(key("3"))
	v.Update(ctrlKey('n'))
	if !reflect.DeepEqual(fake.steps, []int{3}) {
		t.Fatalf("ctrl+n steps %v", fake.steps)
	}
	if v.prefix != 0 {
		t.Fatal("prefix not consumed by ctrl+n")
	}
	v.Update(key("2"))
	v.Update(ctrlKey('p'))
	if !reflect.DeepEqual(fake.steps, []int{3, -2}) {
		t.Fatalf("ctrl+p steps %v", fake.steps)
	}

	// A numeric prefix is discarded, with feedback, by ctrl+a.
	v.Update(key("9"))
	toggles := fake.toggles
	v.Update(ctrlKey('a'))
	if fake.toggles != toggles+1 || !strings.Contains(v.note, "prefix") {
		t.Fatalf("prefix not discarded: %q", v.note)
	}
	if v.prefix != 0 {
		t.Fatal("prefix survived ctrl+a")
	}

	// Thumbnails stay still: animation keys do nothing in the grid.
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !v.grid {
		t.Fatal("did not enter grid")
	}
	toggles, steps := fake.toggles, len(fake.steps)
	v.Update(ctrlKey('a'))
	v.Update(ctrlKey('n'))
	if fake.toggles != toggles || len(fake.steps) != steps {
		t.Fatal("grid changed playback state")
	}
}

func TestStaticImageIgnoresAnimationKeys(t *testing.T) {
	v, _ := New([]string{"one"}, false, "auto")
	v.Resize(30, 8)
	v.Update(ctrlKey('a'))
	if !strings.Contains(v.note, "not an animated image") {
		t.Fatalf("note %q", v.note)
	}
	v.Update(ctrlKey('n'))
	if v.anim != nil {
		t.Fatal("static image opened playback")
	}
}

func TestAnimationStatusLine(t *testing.T) {
	v, _ := New([]string{"a"}, false, "auto")
	v.Resize(80, 6)
	v.items[0].loaded, v.items[0].animated = true, true
	fake := &fakePlayback{frames: 24, plays: 5, loop: 2, playing: true, delay: time.Millisecond}
	v.anim = &animation{item: 0, ctrl: fake, index: 2, visible: true}

	status := ansi.Strip(v.View())
	for _, want := range []string{"frame 3/24", "playing", "loop 2/5"} {
		if !strings.Contains(status, want) {
			t.Errorf("%q missing from %q", want, status)
		}
	}
	if strings.Contains(status, "first frame only") {
		t.Fatal("pending-support note still shown")
	}

	fake.plays = 0
	if s := ansi.Strip(v.View()); !strings.Contains(s, "loop ∞") {
		t.Errorf("infinite loop missing: %q", s)
	}
	fake.playing, fake.ended = false, true
	if s := ansi.Strip(v.View()); !strings.Contains(s, "ended") {
		t.Errorf("ended state missing: %q", s)
	}
}

func TestAnimationVisibilityAndGrid(t *testing.T) {
	fake := &fakePlayback{}
	v, clock := animatedViewer(t, 3, 0, fake)
	v.Update(ctrlKey('a'))
	if !fake.playing || !fake.visible {
		t.Fatalf("playback not visible after play: %+v", fake)
	}

	armed := len(clock.delays)
	if cmd := v.SetVisible(false); cmd != nil {
		t.Fatal("hidden viewer returned work")
	}
	if fake.visible {
		t.Fatal("hidden viewer kept playback visible")
	}
	if len(clock.delays) != armed {
		t.Fatal("hidden viewer armed a clock timer")
	}
	if cmd := v.SetVisible(true); cmd != nil {
		drainCmd(v, cmd)
	} else {
		t.Fatal("reveal returned no command")
	}
	if !fake.visible {
		t.Fatal("revealed viewer did not resume")
	}
	if len(clock.delays) == armed {
		t.Fatal("resume did not arm a clock timer")
	}

	v.Update(tea.BlurMsg{})
	if fake.visible {
		t.Fatal("unfocused viewer kept playback visible")
	}
	v.Update(tea.FocusMsg{})
	if !fake.visible {
		t.Fatal("focused viewer did not resume")
	}

	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !v.grid || fake.visible {
		t.Fatal("grid did not suspend playback")
	}
	armed = len(clock.delays)
	v.Update(nil)
	if len(clock.delays) != armed {
		t.Fatal("grid armed a clock timer")
	}
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if v.grid || !fake.visible {
		t.Fatal("leaving the grid did not resume playback")
	}
}

func TestAnimationStaleTickAndFrame(t *testing.T) {
	fake := &fakePlayback{}
	v, _ := animatedViewer(t, 3, 0, fake)
	v.Update(ctrlKey('a'))
	a := v.anim
	stale := a.gen - 1

	v.Update(viewerTick{anim: a, gen: stale})
	if fake.advances != 0 {
		t.Fatal("stale tick advanced playback")
	}

	a.frameInFlight, a.frameGen = true, a.gen
	pixels := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	committed := a.frame
	v.Update(viewerFrame{anim: a, gen: stale, frame: 1, pixels: pixels})
	if a.frame != committed {
		t.Fatal("stale frame was published")
	}

	fake.frame = 1
	v.Update(viewerFrame{anim: a, gen: a.frameGen, frame: 1, pixels: pixels})
	if a.frame != pixels || a.index != 1 {
		t.Fatalf("committed frame %d", a.index)
	}
	if a.frameInFlight {
		t.Fatal("committed frame left a decode in flight")
	}
}

func TestAnimationCoalescesFrames(t *testing.T) {
	fake := &fakePlayback{}
	v, _ := animatedViewer(t, 4, 0, fake)
	v.Update(ctrlKey('a'))

	cmd := v.Update(viewerTick{anim: v.anim, gen: v.anim.gen})
	if !v.anim.frameInFlight {
		t.Fatal("tick did not start a frame decode")
	}
	if cmd == nil {
		t.Fatal("tick returned no follow-up commands")
	}
	// Another boundary while decoding must not queue a second decode.
	v.Update(viewerTick{anim: v.anim, gen: v.anim.gen})
	if !v.anim.frameInFlight || fake.frame != 2 {
		t.Fatalf("controller frame %d", fake.frame)
	}
	// The slow frame is superseded, so it is dropped and the latest requested.
	committed := v.anim.frame
	v.Update(viewerFrame{anim: v.anim, gen: v.anim.frameGen, frame: 1, pixels: image.NewNRGBA(image.Rect(0, 0, 2, 2))})
	if v.anim.frame != committed {
		t.Fatal("superseded frame was published")
	}
	if !v.anim.frameInFlight {
		t.Fatal("latest frame was not requested")
	}
}

func TestAnimationStopsOnReload(t *testing.T) {
	fake := &fakePlayback{}
	v, _ := animatedViewer(t, 2, 0, fake)
	src := v.anim.src
	reload := v.Update(key("r"))
	if v.anim != nil {
		t.Fatal("reload kept the old sequence")
	}
	if src.Format() != "" {
		t.Fatal("reload did not close the old source")
	}
	settle(t, v, reload, 64)
	if v.anim == nil || v.anim.src == src {
		t.Fatal("reload did not reopen the sequence")
	}
}

// settleUntilFrame runs commands until the first frame decode would be delivered
// and returns that message instead of handling it, so the viewer stays mid-load
// the way it is while a codec call is still in flight.
func settleUntilFrame(t *testing.T, v *Viewer, cmd tea.Cmd, max int) tea.Msg {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; i < max && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg == nil {
			continue
		}
		if _, ok := msg.(viewerFrame); ok {
			return msg
		}
		if next := v.Update(msg); next != nil {
			queue = append(queue, next)
		}
	}
	return nil
}

// drainCmd runs one command and hands the messages it produces to the viewer,
// without following the commands the viewer answers with (an armed playback
// boundary would fire forever against the fake clock).
func drainCmd(v *Viewer, cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for i := 0; i < 8 && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg == nil {
			continue
		}
		v.Update(msg)
	}
}

// TestAnimationFirstFrameWithoutAutoplay covers the poster trap: an image whose
// poster is not frame 0 (APNG) must still present frame 0 when playback starts
// paused, so the viewer decodes frame 0 on open instead of trusting its index.
func TestAnimationFirstFrameWithoutAutoplay(t *testing.T) {
	fake := &fakePlayback{}
	v, clock := animatedViewer(t, 4, 0, fake)
	if fake.playing {
		t.Fatal("autoplay is off but the controller is playing")
	}
	if len(clock.delays) != 0 {
		t.Fatalf("paused animation armed %d timers", len(clock.delays))
	}
	a := v.anim
	if a.frame == nil || a.index != 0 {
		t.Fatalf("frame 0 not presented while paused: frame %v index %d", a.frame != nil, a.index)
	}
	if px := v.pixels(v.index); px != a.frame {
		t.Fatal("the presented pixels are not the decoded frame")
	}
	status := ansi.Strip(v.View())
	if !strings.Contains(status, "frame 1/4") || !strings.Contains(status, "paused") {
		t.Fatalf("status %q does not report the committed frame", status)
	}

	// Stepping from the paused first frame must not re-decode it.
	if cmd := v.Update(ctrlKey('n')); cmd != nil {
		drainCmd(v, cmd)
	}
	if a.index != 1 || fake.steps[len(fake.steps)-1] != 1 {
		t.Fatalf("ctrl+n from the first frame: index %d steps %v", a.index, fake.steps)
	}
}

// TestAnimationStatusWhileLoading keeps the status honest before a frame is
// committed: the poster is on screen, so no frame number may be claimed.
func TestAnimationStatusWhileLoading(t *testing.T) {
	fake := &fakePlayback{}
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = false
	v, err := NewConfigured([]string{writeAnimatedGIF(t, 4, 0)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	v.clock = &fakeClock{}
	v.newPlayback = func(src *animdecode.Source, p PlaybackPolicy, c imageframe.Clock) animationPlayback {
		fake.frames, fake.plays = src.Metadata().Frames, src.Metadata().Plays
		return fake
	}
	frame := settleUntilFrame(t, v, v.Init(), 64)
	if frame == nil {
		t.Fatal("no frame decode was requested")
	}
	if v.anim == nil || v.anim.ctrl == nil || v.anim.frame != nil || v.anim.index >= 0 {
		t.Fatalf("viewer is not mid-load: anim %+v", v.anim)
	}
	status := ansi.Strip(v.View())
	if strings.Contains(status, "frame 1/") {
		t.Fatalf("status claims a committed frame while loading: %q", status)
	}
	if !strings.Contains(status, "loading") {
		t.Fatalf("status %q does not say loading", status)
	}
	if !strings.Contains(status, "4 frames") {
		t.Fatalf("status %q does not report the frame count", status)
	}

	v.Update(frame)
	if s := ansi.Strip(v.View()); !strings.Contains(s, "frame 1/4") || strings.Contains(s, "loading") {
		t.Fatalf("status %q after the first frame", s)
	}
}

// TestAnimationRevealResumesClock proves the reveal path needs no other message:
// hiding cancels the pending boundary, so SetVisible(true) has to hand back the
// command that arms the next one.
func TestAnimationRevealResumesClock(t *testing.T) {
	fake := &fakePlayback{}
	v, clock := animatedViewer(t, 3, 0, fake)
	v.Update(ctrlKey('a'))
	if !fake.playing {
		t.Fatal("ctrl+a did not start playback")
	}

	if cmd := v.SetVisible(false); cmd != nil {
		t.Fatal("hiding the viewer returned work")
	}
	armed := len(clock.delays)
	cmd := v.SetVisible(true)
	if cmd == nil {
		t.Fatal("reveal returned no command: nothing would re-arm the clock")
	}
	drainCmd(v, cmd)
	if len(clock.delays) == armed {
		t.Fatal("reveal did not arm a clock timer")
	}
	if fake.advances == 0 {
		t.Fatal("reveal did not resume playback")
	}

	// A reveal while still hidden or unfocused must not schedule anything.
	v.Update(tea.BlurMsg{})
	armed = len(clock.delays)
	if cmd := v.SetVisible(true); cmd != nil {
		t.Fatalf("unfocused reveal returned %v", cmd())
	}
	if len(clock.delays) != armed {
		t.Fatal("unfocused reveal armed a clock timer")
	}
}

// openUntilFrame runs commands until the first frame decode is requested and
// returns it undelivered. It fails if a clock boundary is armed before any frame
// has been committed, which would let a slow decode skip the initial frame.
func openUntilFrame(t *testing.T, v *Viewer, clock *ManualClock, cmd tea.Cmd, max int) tea.Msg {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; i < max && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg == nil {
			continue
		}
		if _, ok := msg.(viewerFrame); ok {
			return msg
		}
		if next := v.Update(msg); next != nil {
			queue = append(queue, next)
		}
		if n := clock.Pending(); n != 0 {
			t.Fatalf("%d clock boundaries armed before the initial frame committed", n)
		}
	}
	return nil
}

// drivePlayback runs the viewer's commands one at a time with a full day passing
// before each, so every decode is far slower than its frame delay. It returns the
// committed frame after each delivered decode.
func drivePlayback(t *testing.T, v *Viewer, clock *ManualClock, cmd tea.Cmd, steps int) []int {
	t.Helper()
	var committed []int
	for i := 0; i < steps; i++ {
		if cmd == nil {
			return committed
		}
		clock.Advance(24 * time.Hour)
		switch msg := cmd().(type) {
		case nil:
			return committed
		case tea.BatchMsg:
			t.Fatalf("step %d ran %d commands at once: a decode overlapped a boundary", i, len(msg))
		case viewerFrame:
			if msg.err != nil {
				t.Fatalf("decode: %v", msg.err)
			}
			cmd = v.Update(msg)
			committed = append(committed, v.anim.index)
		default:
			cmd = v.Update(msg)
		}
	}
	return committed
}

// TestAnimationInitialFrameBeforeClock covers the poster trap's timing half: the
// clock may not start before the first frame is on screen, or a slow decode lets
// the sequence skip its own opening frame.
func TestAnimationInitialFrameBeforeClock(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = true
	cfg.HalfblockAnimationFPS = 120 // no fps sampling: only decode latency is under test
	v, err := NewConfigured([]string{writeAnimatedGIF(t, 3, 0)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := NewManualClock()
	v.clock = clock

	frame := openUntilFrame(t, v, clock, v.Init(), 64)
	if frame == nil {
		t.Fatal("no initial frame decode was requested")
	}
	if n := clock.Pending(); n != 0 {
		t.Fatalf("%d clock boundaries armed before the initial frame", n)
	}
	// Time passes before the decode lands: nothing may advance meanwhile.
	clock.Advance(time.Hour)
	v.Update(frame)
	a := v.anim
	if a.index != 0 || a.ctrl.Frame() != 0 || a.ctrl.Plays() != 0 {
		t.Fatalf("slow decode skipped the initial frame: index %d controller %d plays %d", a.index, a.ctrl.Frame(), a.ctrl.Plays())
	}
	if n := clock.Pending(); n != 1 {
		t.Fatalf("committed frame armed %d boundaries, want 1", n)
	}
}

// TestAnimationSlowDecodeNeverSkipsFrames keeps every frame of the sequence on
// screen even when each decode takes far longer than its own delay.
func TestAnimationSlowDecodeNeverSkipsFrames(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = true
	cfg.HalfblockAnimationFPS = 120 // no fps sampling: only decode latency is under test
	v, err := NewConfigured([]string{writeAnimatedGIF(t, 3, 0)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := NewManualClock()
	v.clock = clock

	frame := openUntilFrame(t, v, clock, v.Init(), 64)
	if frame == nil {
		t.Fatal("no initial frame decode was requested")
	}
	// Every step is either a committed decode or one boundary, so 16 steps must
	// present 9 frames in order: no frame may be dropped on the way.
	committed := append([]int{0}, drivePlayback(t, v, clock, v.Update(frame), 16)...)
	if len(committed) != 9 {
		t.Fatalf("%d frames presented: %v", len(committed), committed)
	}
	for i, idx := range committed {
		if idx != i%3 {
			t.Fatalf("frame presented out of order at %d: %v", i, committed)
		}
	}
}

// TestAnimationVisiblePushClearsStaleFocus covers a host that routed the blur to
// another pane: the viewer's own focus flag is stale, so a visible push has to
// settle it or the animation stays paused forever.
func TestAnimationVisiblePushClearsStaleFocus(t *testing.T) {
	fake := &fakePlayback{}
	v, _ := animatedViewer(t, 3, 0, fake)
	v.Update(ctrlKey('a'))
	v.Update(tea.BlurMsg{})
	if fake.visible {
		t.Fatal("blur did not pause playback")
	}
	if v.focused {
		t.Fatal("blur left the viewer focused")
	}

	cmd := v.SetVisible(true)
	if !v.focused {
		t.Fatal("visible push left a stale unfocused flag")
	}
	if cmd == nil {
		t.Fatal("visible push returned no command")
	}
	drainCmd(v, cmd)
	if !fake.visible || fake.advances == 0 {
		t.Fatalf("stale focus kept playback paused: visible %v advances %d", fake.visible, fake.advances)
	}

	// Hiding is not a focus change: only visibility moves.
	v.SetVisible(false)
	if !v.focused {
		t.Fatal("hiding changed the focus state")
	}
}

// pump drives commands to completion, firing any armed playback boundary with
// the manual clock before each step.
func pump(t *testing.T, v *Viewer, clock *ManualClock, cmd tea.Cmd, max int) (int, []tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	steps := 0
	for len(queue) > 0 && steps < max {
		steps++
		clock.Advance(24 * time.Hour)
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg == nil {
			continue
		}
		if follow := v.Update(msg); follow != nil {
			queue = append(queue, follow)
		}
	}
	return steps, queue
}

// TestAnimationRealControllerPlaysAndSteps runs the viewer against playback.go's
// controller with an injected manual clock: finite loops, stepping and committed
// frames all reach the half-block presentation.
func TestAnimationRealControllerPlaysAndSteps(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = false
	v, err := NewConfigured([]string{writeAnimatedGIF(t, 3, 0)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := NewManualClock()
	v.clock = clock
	_, _ = pump(t, v, clock, v.Init(), 64)
	if v.anim == nil || v.anim.ctrl == nil {
		t.Fatal("animation not attached")
	}

	steps, pending := pump(t, v, clock, v.Update(ctrlKey('a')), 40)
	if steps < 4 {
		t.Fatalf("playback stopped after %d steps", steps)
	}
	a := v.anim
	if a.frame == nil || a.index < 0 || a.index > 2 {
		t.Fatalf("committed frame %d (controller %d)", a.index, a.ctrl.Frame())
	}
	if v.pixels(0) != a.frame {
		t.Fatal("viewer does not present the committed frame")
	}
	status := ansi.Strip(v.View())
	if !strings.Contains(status, "frame ") || !strings.Contains(status, "playing") {
		t.Fatalf("status %q", status)
	}

	// Pausing drains the outstanding decode, then stepping clamps and commits
	// the requested frame.
	v.Update(ctrlKey('a'))
	if next := v.Update(nil); next != nil {
		pending = append(pending, next)
	}
	pump(t, v, clock, tea.Batch(pending...), 32)
	if a.frameInFlight || a.index != a.ctrl.Frame() {
		t.Fatalf("paused drain left frame %d, index %d, in flight %v", a.ctrl.Frame(), a.index, a.frameInFlight)
	}
	v.Update(key("5"))
	step := v.Update(ctrlKey('n'))
	pump(t, v, clock, step, 16)
	if a.ctrl.Playing() {
		t.Fatal("stepping kept playback running")
	}
	if got := a.ctrl.Frame(); got != a.index || got != 2 {
		t.Fatalf("step committed %d, controller %d", a.index, got)
	}
	if s := ansi.Strip(v.View()); !strings.Contains(s, "frame 3/3") || !strings.Contains(s, "paused") {
		t.Fatalf("stepped status %q", s)
	}
	v.Update(ctrlKey('a'))
	if !a.ctrl.Playing() {
		t.Fatal("ctrl+a did not resume")
	}
}

// TestAnimationRealControllerFiniteEnds proves the ended state and restart reach
// the status line and keys.
func TestAnimationRealControllerFiniteEnds(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = true
	v, err := NewConfigured([]string{writeAnimatedGIF(t, 2, -1)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := NewManualClock()
	v.clock = clock
	_, _ = pump(t, v, clock, v.Init(), 128)
	if v.anim == nil || !v.anim.ctrl.Ended() {
		t.Fatal("finite sequence did not end")
	}
	if s := ansi.Strip(v.View()); !strings.Contains(s, "ended") {
		t.Fatalf("ended status missing: %q", s)
	}
	v.Update(ctrlKey('a'))
	if !v.anim.ctrl.Playing() {
		t.Fatal("ctrl+a did not restart the ended sequence")
	}
	_, _ = pump(t, v, clock, v.Update(nil), 32)
}
