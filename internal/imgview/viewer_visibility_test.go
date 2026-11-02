package imgview

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imageframe"
)

// recordingClock is the deterministic manual clock plus the delays the viewer
// asked for, so a test can prove which boundary was armed and when.
type recordingClock struct {
	*ManualClock
	delays []time.Duration
}

func (c *recordingClock) NewTimer(d time.Duration) imageframe.Timer {
	c.delays = append(c.delays, d)
	return c.ManualClock.NewTimer(d)
}

func (c *recordingClock) last() time.Duration {
	if len(c.delays) == 0 {
		return 0
	}
	return c.delays[len(c.delays)-1]
}

func writeStillPNG(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "still.png")
	if err := os.WriteFile(p, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestViewerBlurCancelsTheOutstandingDecode proves a blurred screen stops paying
// for work it cannot show: the outstanding request is cancelled, its result is
// not published as an error, and no new decode starts until the screen is
// visible again.
func TestViewerBlurCancelsTheOutstandingDecode(t *testing.T) {
	v, err := New([]string{writeStillPNG(t), writeStillPNG(t)}, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	pending := v.Init()
	if pending == nil || !v.inFlight {
		t.Fatalf("visible viewer did not start a poster decode: cmd %v in flight %v", pending != nil, v.inFlight)
	}

	if cmd := v.Update(tea.BlurMsg{}); cmd != nil {
		t.Fatal("blur returned work")
	}
	if v.cancel != nil {
		t.Fatal("blur left the decode request uncancelled")
	}
	if cmd := v.Update(nil); cmd != nil {
		t.Fatal("a hidden viewer scheduled work")
	}

	// Revealing before the interrupted codec returns must not start a second
	// concurrent decode: the cancelled request frees the slot when it reports.
	if cmd := v.Update(tea.FocusMsg{}); cmd != nil {
		t.Fatal("a reveal overlapped the cancelled codec call")
	}

	// The cancelled request still reports back, because a codec call cannot be
	// interrupted. It must not turn the item into an error, and the reveal that is
	// already in effect restarts the decode in the same pass.
	restart := v.Update(pending())
	if it := v.items[0]; it.loaded || it.err != nil {
		t.Fatalf("cancelled decode published: loaded %v err %v", it.loaded, it.err)
	}
	if restart == nil || !v.inFlight {
		t.Fatalf("the freed slot did not restart the decode: cmd %v in flight %v", restart != nil, v.inFlight)
	}
	settle(t, v, restart, 8)
	if v.pixels(0) == nil {
		t.Fatal("the restarted decode published nothing")
	}
}

// TestViewerHiddenGridStartsNoThumbnailDecode covers the same gate for a
// thumbnail page: a hidden grid must not keep filling its visible range.
func TestViewerHiddenGridStartsNoThumbnailDecode(t *testing.T) {
	v, err := New([]string{writeStillPNG(t), writeStillPNG(t), writeStillPNG(t)}, true, "auto")
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(60, 20)
	pending := v.Init()
	if pending == nil || !v.inFlight {
		t.Fatalf("visible grid did not start a thumbnail decode: cmd %v in flight %v", pending != nil, v.inFlight)
	}

	v.Update(tea.BlurMsg{})
	v.Update(pending())
	if cmd := v.Update(nil); cmd != nil {
		t.Fatal("a hidden grid scheduled another thumbnail")
	}
	if v.inFlight {
		t.Fatal("a hidden grid started a thumbnail decode")
	}

	cmd := v.Update(tea.FocusMsg{})
	if cmd == nil || !v.inFlight {
		t.Fatalf("focus did not resume the page fill: cmd %v in flight %v", cmd != nil, v.inFlight)
	}
	settle(t, v, cmd, 64)
	if v.pixels(0) == nil {
		t.Fatal("the resumed page fill published nothing")
	}
}

// TestViewerHelpStopsDecodeWork covers the overlay half of the visibility
// contract: the help screen covers the image, so it must not decode behind it.
func TestViewerHelpStopsDecodeWork(t *testing.T) {
	v, err := New([]string{writeStillPNG(t)}, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	pending := v.Init()
	if pending == nil {
		t.Fatal("visible viewer did not start a poster decode")
	}

	v.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if !v.help {
		t.Fatal("F1 did not open help")
	}
	if v.cancel != nil {
		t.Fatal("help left the decode request uncancelled")
	}
	v.Update(pending())
	if it := v.items[0]; it.loaded || it.err != nil {
		t.Fatalf("a covered decode published: loaded %v err %v", it.loaded, it.err)
	}
	if cmd := v.Update(nil); cmd != nil {
		t.Fatal("help scheduled decode work")
	}

	cmd := v.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if v.help {
		t.Fatal("F1 did not close help")
	}
	if cmd == nil || !v.inFlight {
		t.Fatalf("closing help did not resume the decode: cmd %v in flight %v", cmd != nil, v.inFlight)
	}
	settle(t, v, cmd, 8)
	if v.pixels(0) == nil {
		t.Fatal("closing help did not publish the poster")
	}
}

// playingViewer opens an animated viewer with the real playback controller on a
// manual clock and commits its first frame, returning the armed boundary.
func playingViewer(t *testing.T, frames, loop int) (*Viewer, *recordingClock, tea.Cmd) {
	t.Helper()
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = true
	cfg.HalfblockAnimationFPS = 120 // no fps sampling: only visibility is under test
	v, err := NewConfigured([]string{writeAnimatedGIF(t, frames, loop)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := &recordingClock{ManualClock: NewManualClock()}
	v.clock = clock
	frame := openUntilFrame(t, v, clock.ManualClock, v.Init(), 64)
	if frame == nil {
		t.Fatal("no initial frame decode was requested")
	}
	wait := v.Update(frame)
	if v.anim == nil || v.anim.index != 0 {
		t.Fatalf("initial frame not committed: %+v", v.anim)
	}
	return v, clock, wait
}

// TestAnimationBlurStopsTheClockAndFocusResumesIt covers the timer half of the
// acceptance: a blur stops the armed boundary and freezes the remaining delay,
// hidden time accumulates no catch-up work, and focus resumes from the saved
// remainder with the frame and loop counter untouched.
func TestAnimationBlurStopsTheClockAndFocusResumesIt(t *testing.T) {
	v, clock, wait := playingViewer(t, 3, 0)
	if wait == nil {
		t.Fatal("the committed frame armed no boundary")
	}
	if n := clock.Pending(); n != 1 {
		t.Fatalf("%d boundaries armed, want 1", n)
	}
	if got := clock.last(); got != 20*time.Millisecond {
		t.Fatalf("first boundary %v", got)
	}

	clock.Advance(8 * time.Millisecond)
	if cmd := v.Update(tea.BlurMsg{}); cmd != nil {
		t.Fatal("blur returned work")
	}
	if msg := wait(); msg != nil {
		t.Fatalf("a cancelled boundary produced %T", msg)
	}
	if n := clock.Pending(); n != 0 {
		t.Fatalf("%d timers survived the blur", n)
	}
	armed := len(clock.delays)
	clock.Advance(time.Hour) // hidden time must not queue catch-up boundaries
	if v.anim.frameInFlight {
		t.Fatal("a hidden viewer kept a frame decode in flight")
	}
	if cmd := v.Update(nil); cmd != nil {
		t.Fatal("a hidden viewer scheduled playback work")
	}
	if len(clock.delays) != armed {
		t.Fatal("a hidden viewer armed a boundary")
	}
	if v.anim.index != 0 || v.anim.ctrl.Frame() != 0 || v.anim.ctrl.Plays() != 0 || v.anim.ctrl.Loop() != 1 {
		t.Fatalf("hidden viewer moved: index %d frame %d plays %d loop %d",
			v.anim.index, v.anim.ctrl.Frame(), v.anim.ctrl.Plays(), v.anim.ctrl.Loop())
	}

	// Focus resumes from the saved remainder, not from a fresh delay.
	cmd := v.Update(tea.FocusMsg{})
	if cmd == nil {
		t.Fatal("focus armed no boundary")
	}
	if len(clock.delays) != armed+1 {
		t.Fatalf("focus armed %d boundaries, want 1", len(clock.delays)-armed)
	}
	if got := clock.last(); got != 12*time.Millisecond {
		t.Fatalf("resumed boundary %v, want the saved 12ms", got)
	}
	if v.anim.ctrl.Frame() != 0 || v.anim.ctrl.Plays() != 0 || v.anim.ctrl.Loop() != 1 {
		t.Fatalf("focus restarted the sequence: frame %d plays %d loop %d",
			v.anim.ctrl.Frame(), v.anim.ctrl.Plays(), v.anim.ctrl.Loop())
	}
	clock.Advance(12 * time.Millisecond)
	tick, ok := cmd().(viewerTick)
	if !ok {
		t.Fatalf("the resumed boundary produced %T", tick)
	}
	next := v.Update(tick)
	if got := v.anim.ctrl.Frame(); got != 1 {
		t.Fatalf("controller frame %d after the resumed boundary", got)
	}
	if next == nil || !v.anim.frameInFlight {
		t.Fatal("resumed playback requested no frame decode")
	}
	decoded, ok := next().(viewerFrame)
	if !ok || decoded.err != nil {
		t.Fatalf("frame decode: %#v", decoded)
	}
	v.Update(decoded)
	if v.anim.index != 1 {
		t.Fatalf("committed frame %d after the resume", v.anim.index)
	}
}

// TestAnimationHiddenMidDecodeResumes proves a decode that was in flight when the
// screen was hidden cannot wedge playback: its result is dropped, the request
// flag is cleared, and the reveal asks for the frame again.
func TestAnimationHiddenMidDecodeResumes(t *testing.T) {
	v, clock, wait := playingViewer(t, 3, 0)
	clock.Advance(20 * time.Millisecond)
	tick, ok := wait().(viewerTick)
	if !ok {
		t.Fatalf("boundary produced %T", tick)
	}
	pending := v.Update(tick)
	if pending == nil || !v.anim.frameInFlight {
		t.Fatal("the advanced boundary requested no frame decode")
	}

	if cmd := v.Update(tea.BlurMsg{}); cmd != nil {
		t.Fatal("blur returned work")
	}
	// A codec call cannot be interrupted, so the result still arrives.
	stale, ok := pending().(viewerFrame)
	if !ok {
		t.Fatalf("frame decode produced %T", stale)
	}
	v.Update(stale)
	if v.anim.frameInFlight {
		t.Fatal("the hidden cancellation left a decode in flight")
	}
	if v.anim.index != 0 {
		t.Fatalf("a hidden decode was published as frame %d", v.anim.index)
	}

	cmd := v.Update(tea.FocusMsg{})
	if cmd == nil || !v.anim.frameInFlight {
		t.Fatalf("the reveal did not request the missing frame: cmd %v in flight %v", cmd != nil, v.anim.frameInFlight)
	}
	fresh, ok := cmd().(viewerFrame)
	if !ok || fresh.err != nil {
		t.Fatalf("frame decode after the reveal: %#v", fresh)
	}
	v.Update(fresh)
	if v.anim.index != 1 {
		t.Fatalf("committed frame %d after the reveal", v.anim.index)
	}
}

// TestAnimationUserPauseSurvivesBlurAndReveal covers the distinction between a
// user pause and a visibility pause: revealing must not undo the user's choice.
func TestAnimationUserPauseSurvivesBlurAndReveal(t *testing.T) {
	cfg := config.DefaultImageViewer()
	cfg.AnimationAutoplay = false
	v, err := NewConfigured([]string{writeAnimatedGIF(t, 3, 0)}, false, "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	clock := &recordingClock{ManualClock: NewManualClock()}
	v.clock = clock
	settle(t, v, v.Init(), 64)
	if v.anim == nil || v.anim.ctrl == nil {
		t.Fatal("animation not attached")
	}

	v.Update(ctrlKey('a'))
	if !v.anim.ctrl.Playing() {
		t.Fatal("ctrl+a did not start playback")
	}
	v.Update(ctrlKey('a'))
	if v.anim.ctrl.Playing() {
		t.Fatal("the second ctrl+a did not pause")
	}
	armed := len(clock.delays)

	v.Update(tea.BlurMsg{})
	v.Update(tea.FocusMsg{})
	if v.anim.ctrl.Playing() {
		t.Fatal("the reveal undid the user's pause")
	}
	if len(clock.delays) != armed {
		t.Fatal("the reveal armed a boundary for a paused sequence")
	}
	clock.Advance(time.Hour)
	if cmd := v.Update(nil); cmd != nil {
		t.Fatal("a user-paused sequence scheduled work")
	}
	if v.anim.ctrl.Frame() != 0 || v.anim.index != 0 {
		t.Fatalf("a paused sequence moved: frame %d index %d", v.anim.ctrl.Frame(), v.anim.index)
	}
}
