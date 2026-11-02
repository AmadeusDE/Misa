package imgview

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
)

// updateUntilIndexing drives the viewer until it creates the sequence and
// returns the command that starts indexing it, undelivered.
func updateUntilIndexing(t *testing.T, v *Viewer, cmd tea.Cmd, max int) tea.Cmd {
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
		next := v.Update(msg)
		if v.anim != nil {
			return next
		}
		if next != nil {
			queue = append(queue, next)
		}
	}
	t.Fatal("the viewer never started indexing its sequence")
	return nil
}

// animationResult runs a command tree until it produces the sequence index
// result and returns it undelivered.
func animationResult(t *testing.T, cmd tea.Cmd, max int) viewerAnimation {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; i < max && len(queue) > 0; i++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case viewerAnimation:
			return msg
		}
	}
	t.Fatal("indexing produced no sequence")
	return viewerAnimation{}
}

// TestHiddenViewerStartsNoDecodeForANewSelection covers the gate rather than the
// cancellation: a viewer that is hidden while its selection moves on must not
// start a decode for the item it cannot show.
func TestHiddenViewerStartsNoDecodeForANewSelection(t *testing.T) {
	v, err := New([]string{writeStillPNG(t), writeStillPNG(t)}, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	pending := v.Init()
	if pending == nil || !v.inFlight {
		t.Fatalf("visible viewer started no decode: cmd %v in flight %v", pending != nil, v.inFlight)
	}
	v.Update(tea.BlurMsg{})
	v.Update(pending()) // the cancelled codec still reports
	if v.inFlight {
		t.Fatal("the cancelled decode kept its slot")
	}

	v.Update(tea.KeyPressMsg{Code: 'n'})
	if v.index != 1 {
		t.Fatalf("selection %d, want the next item", v.index)
	}
	if cmd := v.Update(nil); cmd != nil {
		t.Fatal("a hidden viewer started a decode for the new selection")
	}
	if v.inFlight || v.items[1].loading {
		t.Fatal("a hidden viewer marked the new selection loading")
	}

	cmd := v.Update(tea.FocusMsg{})
	if cmd == nil || !v.inFlight {
		t.Fatalf("the reveal did not decode the selection: cmd %v in flight %v", cmd != nil, v.inFlight)
	}
	settle(t, v, cmd, 8)
	if v.pixels(1) == nil {
		t.Fatal("the revealed selection published nothing")
	}
}

// TestHiddenViewerInvalidatesOnce covers the request bookkeeping: hiding twice
// must not invalidate a later request, and a hidden pass with nothing outstanding
// must not keep bumping the generation.
func TestHiddenViewerInvalidatesOnce(t *testing.T) {
	v, err := New([]string{writeStillPNG(t)}, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	v.Resize(40, 12)
	pending := v.Init()
	v.Update(tea.BlurMsg{})
	gen := v.loadGen
	for range 3 {
		v.Update(nil)
	}
	if v.loadGen != gen {
		t.Fatalf("generation moved from %d to %d with nothing outstanding", gen, v.loadGen)
	}
	v.Update(pending())
	gen = v.loadGen
	v.Update(nil)
	if v.loadGen != gen {
		t.Fatalf("a hidden pass invalidated the next request: %d -> %d", gen, v.loadGen)
	}
}

// TestHiddenViewerDropsTheSequenceIndex covers the other decode command: the
// bounded index of an animation. Its result may not attach a source to a viewer
// that was hidden meanwhile, and the reveal must index again.
func TestHiddenViewerDropsTheSequenceIndex(t *testing.T) {
	t.Run("result arrives after the hide", func(t *testing.T) {
		v, _, _ := playingViewer(t, 3, 0)
		first := v.anim
		if first == nil || first.src == nil {
			t.Fatal("the sequence was not attached")
		}
		// Re-index from scratch, as a reload does, and hide before the result is
		// delivered: the codec call finished, so only the generation can drop it.
		index := v.openAnimation(v.index)
		v.anim.index = -1 // pretend no frame has been committed yet
		v.Update(tea.BlurMsg{})
		if v.anim == nil {
			t.Fatal("hiding stopped the sequence before the index result arrived")
		}
		result := animationResult(t, index, 8)
		v.Update(result)
		if v.anim != nil {
			t.Fatal("a hidden viewer attached the sequence it indexed")
		}
		if result.anim.src != nil {
			t.Fatal("the dropped sequence kept its source open")
		}

		cmd := v.Update(tea.FocusMsg{})
		if cmd == nil || v.anim == nil || v.anim == first {
			t.Fatalf("the reveal did not index the sequence again: cmd %v anim %p", cmd != nil, v.anim)
		}
		revealed := animationResult(t, cmd, 8)
		v.Update(revealed)
		if v.anim == nil || v.anim.src == nil {
			t.Fatal("the revealed sequence never attached")
		}
		if !v.anim.frameInFlight {
			t.Fatal("the revealed sequence requested no frame")
		}
	})

	t.Run("command runs after the hide", func(t *testing.T) {
		cfg := config.DefaultImageViewer()
		cfg.AnimationAutoplay = true
		v, err := NewConfigured([]string{writeAnimatedGIF(t, 3, 0)}, false, "", cfg)
		if err != nil {
			t.Fatal(err)
		}
		v.Resize(40, 12)
		v.clock = NewManualClock()
		index := updateUntilIndexing(t, v, v.Init(), 8)
		if index == nil {
			t.Fatal("no indexing command was returned")
		}
		v.SetVisible(false)
		if v.anim == nil {
			t.Fatal("hiding dropped the sequence instead of its work")
		}
		result := animationResult(t, index, 8)
		if result.err == nil {
			t.Fatal("a hidden viewer indexed its sequence anyway")
		}
		v.Update(result)
		if v.anim != nil {
			t.Fatal("a cancelled index attached a sequence")
		}
		if v.items[0].animErr != nil {
			t.Fatalf("the cancellation was reported as an error: %v", v.items[0].animErr)
		}
		if cmd := v.SetVisible(true); cmd == nil {
			t.Fatal("the reveal returned no work")
		}
	})
}

// TestHelpOverlayPausesPlaybackAndResumes covers the overlay half of the
// animation contract: help covers the image, so it stops the clock without
// losing the saved delay, the committed frame or the loop counter.
func TestHelpOverlayPausesPlaybackAndResumes(t *testing.T) {
	v, clock, wait := playingViewer(t, 3, 0)
	clock.Advance(5 * time.Millisecond)
	if cmd := v.Update(tea.KeyPressMsg{Code: tea.KeyF1}); cmd != nil {
		t.Fatal("opening help returned work")
	}
	if !v.help {
		t.Fatal("F1 did not open help")
	}
	if msg := wait(); msg != nil {
		t.Fatalf("the covered boundary produced %T", msg)
	}
	if n := clock.Pending(); n != 0 {
		t.Fatalf("%d boundaries survived the help overlay", n)
	}
	armed := len(clock.delays)
	clock.Advance(time.Hour)
	if cmd := v.Update(nil); cmd != nil {
		t.Fatal("help scheduled playback work")
	}
	if len(clock.delays) != armed {
		t.Fatal("help armed a boundary")
	}
	if v.anim.index != 0 || v.anim.ctrl.Frame() != 0 || v.anim.ctrl.Plays() != 0 || v.anim.ctrl.Loop() != 1 {
		t.Fatalf("help moved the sequence: index %d frame %d plays %d loop %d",
			v.anim.index, v.anim.ctrl.Frame(), v.anim.ctrl.Plays(), v.anim.ctrl.Loop())
	}

	cmd := v.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	if v.help {
		t.Fatal("F1 did not close help")
	}
	if cmd == nil {
		t.Fatal("closing help armed no boundary")
	}
	if got := clock.last(); got != 15*time.Millisecond {
		t.Fatalf("resumed boundary %v, want the saved 15ms", got)
	}
	if v.anim.ctrl.Plays() != 0 || v.anim.ctrl.Loop() != 1 {
		t.Fatalf("closing help restarted the sequence: plays %d loop %d", v.anim.ctrl.Plays(), v.anim.ctrl.Loop())
	}
	clock.Advance(15 * time.Millisecond)
	tick, ok := cmd().(viewerTick)
	if !ok {
		t.Fatalf("the resumed boundary produced %T", tick)
	}
	next := v.Update(tick)
	if v.anim.ctrl.Frame() != 1 || next == nil {
		t.Fatalf("frame %d after the resumed boundary", v.anim.ctrl.Frame())
	}
}
