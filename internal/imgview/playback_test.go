package imgview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/imageframe"
	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
)

type fakeTiming struct {
	meta   imageframe.SequenceMetadata
	delays []time.Duration
	fail   bool
}

func (f *fakeTiming) Metadata() imageframe.SequenceMetadata { return f.meta }

func (f *fakeTiming) FrameTiming(i int) (time.Duration, time.Duration, error) {
	if f.fail || i < 0 || i >= len(f.delays) {
		return 0, 0, animdecode.ErrClosed
	}
	return f.delays[i], f.delays[i], nil
}

func playbackMeta(frames int, plays uint64) imageframe.SequenceMetadata {
	return imageframe.SequenceMetadata{ID: 7, Size: image.Pt(4, 4), Frames: frames, Plays: plays}
}

func newTestPlayback(t *testing.T, frames int, plays uint64, delay time.Duration, policy PlaybackPolicy) (*playbackController, *ManualClock) {
	t.Helper()
	delays := make([]time.Duration, frames)
	for i := range delays {
		delays[i] = delay
	}
	clock := NewManualClock()
	return newPlaybackController(&fakeTiming{meta: playbackMeta(frames, plays), delays: delays}, policy, clock), clock
}

func TestPlaybackFiniteLoopsEnd(t *testing.T) {
	p, _ := newTestPlayback(t, 3, 2, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	if p.Frames() != 3 || p.Frame() != 0 || !p.Playing() || p.Ended() || p.Plays() != 2 || p.Loop() != 1 {
		t.Fatalf("initial state %d %d %v %v %d %d", p.Frames(), p.Frame(), p.Playing(), p.Ended(), p.Plays(), p.Loop())
	}
	if d := p.NextDelay(); d != 40*time.Millisecond {
		t.Fatalf("first delay = %v", d)
	}
	for _, want := range []int{1, 2, 0, 1, 2} {
		if !p.Advance() || p.Frame() != want {
			t.Fatalf("advance to %d reported frame %d", want, p.Frame())
		}
	}
	if p.Advance() {
		t.Fatal("playback continued after the last play")
	}
	if !p.Ended() || p.Playing() || p.Frame() != 2 || p.Loop() != 2 || p.NextDelay() != 0 {
		t.Fatalf("final state %d %v %v %d %v", p.Frame(), p.Playing(), p.Ended(), p.Loop(), p.NextDelay())
	}
}

func TestPlaybackInfiniteLoopsContinue(t *testing.T) {
	p, _ := newTestPlayback(t, 2, 0, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	for i := 0; i < 9; i++ {
		if !p.Advance() {
			t.Fatalf("infinite sequence stopped after %d advances", i)
		}
	}
	if p.Ended() || p.Plays() != 0 || p.Loop() != 5 || !p.Playing() {
		t.Fatalf("state %v %d %d %v", p.Ended(), p.Plays(), p.Loop(), p.Playing())
	}
}

func TestPlaybackLoopPolicyOverridesFile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy LoopMode
		plays  uint64
		total  uint64
		ends   bool
	}{
		{"file", LoopFile, 3, 3, true},
		{"once", LoopOnce, 5, 1, true},
		{"infinite", LoopInfinite, 1, 0, false},
		{"file infinite", LoopFile, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := newTestPlayback(t, 2, tc.plays, 40*time.Millisecond, PlaybackPolicy{Autoplay: true, Loops: tc.policy})
			if p.Plays() != tc.total {
				t.Fatalf("plays = %d, want %d", p.Plays(), tc.total)
			}
			ended := false
			for i := 0; i < 8; i++ {
				if !p.Advance() {
					ended = true
					break
				}
			}
			if ended != tc.ends {
				t.Fatalf("ended = %v, want %v", ended, tc.ends)
			}
			if tc.ends && p.Loop() != tc.total {
				t.Fatalf("final loop = %d, want %d", p.Loop(), tc.total)
			}
		})
	}
}

func TestPlaybackPauseFreezesRemainingDelay(t *testing.T) {
	p, clock := newTestPlayback(t, 3, 0, 100*time.Millisecond, PlaybackPolicy{Autoplay: true})
	if d := p.NextDelay(); d != 100*time.Millisecond {
		t.Fatalf("delay = %v", d)
	}
	clock.Advance(40 * time.Millisecond)
	p.Toggle()
	if p.Playing() || p.NextDelay() != 0 {
		t.Fatalf("pause did not stop playback: %v %v", p.Playing(), p.NextDelay())
	}
	clock.Advance(time.Hour)
	p.Toggle()
	if d := p.NextDelay(); d != 60*time.Millisecond {
		t.Fatalf("resumed delay = %v, want the saved 60ms", d)
	}
	clock.Advance(30 * time.Millisecond)
	if d := p.NextDelay(); d != 30*time.Millisecond {
		t.Fatalf("remaining delay = %v", d)
	}
	if !p.Advance() || p.Frame() != 1 {
		t.Fatalf("advance = %d", p.Frame())
	}
}

func TestPlaybackHiddenFreezesAndResumesWithoutLosingLoops(t *testing.T) {
	p, clock := newTestPlayback(t, 2, 0, 100*time.Millisecond, PlaybackPolicy{Autoplay: true})
	p.Advance()
	if !p.Advance() || p.Loop() != 2 {
		t.Fatalf("wrap did not count a play: %d", p.Loop())
	}
	if d := p.NextDelay(); d != 100*time.Millisecond {
		t.Fatalf("delay = %v", d)
	}
	clock.Advance(30 * time.Millisecond)
	p.SetVisible(false)
	if p.Playing() || p.NextDelay() != 0 {
		t.Fatalf("hidden playback kept running: %v", p.Playing())
	}
	clock.Advance(time.Hour)
	p.SetVisible(true)
	if !p.Playing() {
		t.Fatal("revealing did not resume playback")
	}
	if d := p.NextDelay(); d != 70*time.Millisecond {
		t.Fatalf("resumed delay = %v", d)
	}
	if p.Loop() != 2 {
		t.Fatalf("revealing reset the play counter: %d", p.Loop())
	}
	if !p.Advance() || p.Loop() != 2 {
		t.Fatalf("play counter changed while playing: %d", p.Loop())
	}
}

func TestPlaybackStepPausesAndClamps(t *testing.T) {
	p, clock := newTestPlayback(t, 3, 0, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	p.Step(2)
	if p.Frame() != 2 || p.Playing() || p.NextDelay() != 0 {
		t.Fatalf("step = %d %v %v", p.Frame(), p.Playing(), p.NextDelay())
	}
	p.Step(5)
	if p.Frame() != 2 {
		t.Fatalf("forward clamp = %d", p.Frame())
	}
	p.Step(-9)
	if p.Frame() != 0 {
		t.Fatalf("backward clamp = %d", p.Frame())
	}
	p.Step(0)
	if p.Frame() != 0 || p.Loop() != 1 {
		t.Fatalf("zero step changed state: %d %d", p.Frame(), p.Loop())
	}
	p.Toggle()
	if d := p.NextDelay(); d != 40*time.Millisecond {
		t.Fatalf("delay after stepping = %v", d)
	}
	clock.Advance(40 * time.Millisecond)
	if !p.Advance() || p.Frame() != 1 {
		t.Fatalf("advance = %d", p.Frame())
	}
}

func TestPlaybackToggleRestartsEndedSequence(t *testing.T) {
	p, _ := newTestPlayback(t, 2, 1, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	p.Advance()
	if p.Advance() {
		t.Fatal("finite sequence did not end")
	}
	p.Toggle()
	if p.Ended() || !p.Playing() || p.Frame() != 0 || p.Loop() != 1 {
		t.Fatalf("restart state %d %v %v %d", p.Frame(), p.Playing(), p.Ended(), p.Loop())
	}
	if d := p.NextDelay(); d != 40*time.Millisecond {
		t.Fatalf("restart delay = %v", d)
	}
	if !p.Advance() || p.Frame() != 1 {
		t.Fatalf("advance after restart = %d", p.Frame())
	}
}

func TestPlaybackFPSCapSkipsFrames(t *testing.T) {
	p, clock := newTestPlayback(t, 10, 0, 20*time.Millisecond, PlaybackPolicy{Autoplay: true, FPS: 10})
	if d := p.NextDelay(); d != 100*time.Millisecond {
		t.Fatalf("capped delay = %v", d)
	}
	clock.Advance(100 * time.Millisecond)
	if !p.Advance() || p.Frame() != 5 {
		t.Fatalf("window presented frame %d", p.Frame())
	}
	if d := p.NextDelay(); d != 100*time.Millisecond {
		t.Fatalf("second delay = %v", d)
	}
	if !p.Advance() || p.Frame() != 0 || p.Loop() != 2 {
		t.Fatalf("wrap = frame %d loop %d", p.Frame(), p.Loop())
	}
}

func TestPlaybackShortDelaysUseFloor(t *testing.T) {
	p, clock := newTestPlayback(t, 2, 0, 0, PlaybackPolicy{Autoplay: true})
	if d := p.NextDelay(); d != minFrameDelay {
		t.Fatalf("zero delay = %v", d)
	}
	clock.Advance(minFrameDelay)
	if !p.Advance() || p.Frame() != 1 {
		t.Fatalf("advance = %d", p.Frame())
	}
	if d := p.NextDelay(); d != minFrameDelay {
		t.Fatalf("delay after advance = %v", d)
	}
}

// The first boundary is the accumulated source time of the frames it covers, so
// a short first frame is not stretched to the fps window.
func TestPlaybackFirstBoundaryKeepsSourceTime(t *testing.T) {
	delays := []time.Duration{20 * time.Millisecond, 200 * time.Millisecond}
	clock := NewManualClock()
	p := newPlaybackController(&fakeTiming{meta: playbackMeta(2, 0), delays: delays}, PlaybackPolicy{Autoplay: true, FPS: 10}, clock)
	if d := p.NextDelay(); d != 220*time.Millisecond {
		t.Fatalf("first boundary = %v, want 20ms+200ms of source time", d)
	}
	clock.Advance(220 * time.Millisecond)
	if !p.Advance() || p.Frame() != 0 || p.Loop() != 2 {
		t.Fatalf("boundary presented frame %d loop %d", p.Frame(), p.Loop())
	}
	if d := p.NextDelay(); d != 220*time.Millisecond {
		t.Fatalf("second boundary = %v", d)
	}
}

func TestPlaybackUnevenDelaysAccumulate(t *testing.T) {
	delays := []time.Duration{200 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond}
	clock := NewManualClock()
	p := newPlaybackController(&fakeTiming{meta: playbackMeta(3, 0), delays: delays}, PlaybackPolicy{Autoplay: true, FPS: 10}, clock)
	// A first frame longer than the window is presented for its own duration.
	if d := p.NextDelay(); d != 200*time.Millisecond {
		t.Fatalf("first boundary = %v", d)
	}
	clock.Advance(200 * time.Millisecond)
	if !p.Advance() || p.Frame() != 1 {
		t.Fatalf("frame %d", p.Frame())
	}
	// 20ms+20ms are still below the 100ms window, so the next boundary covers
	// the wrap and one more frame: 20+20+200 = 240ms.
	if d := p.NextDelay(); d != 240*time.Millisecond {
		t.Fatalf("second boundary = %v", d)
	}
}

// A finite sequence presents its final frame and honors that frame's delay before
// reporting ended, with and without an fps cap.
func TestPlaybackFinalFrameHonorsItsDelay(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fps   int
		first time.Duration
		frame int
	}{
		{"no cap", 0, 20 * time.Millisecond, 1},
		{"capped", 10, 40 * time.Millisecond, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, clock := newTestPlayback(t, 3, 1, 20*time.Millisecond, PlaybackPolicy{Autoplay: true, FPS: tc.fps})
			if d := p.NextDelay(); d != tc.first {
				t.Fatalf("first boundary = %v", d)
			}
			clock.Advance(tc.first)
			if !p.Advance() || p.Frame() != tc.frame || p.Ended() {
				t.Fatalf("frame %d ended %v", p.Frame(), p.Ended())
			}
			for p.Frame() != 2 {
				d := p.NextDelay()
				if d <= 0 {
					t.Fatal("no boundary scheduled")
				}
				clock.Advance(d)
				if !p.Advance() {
					t.Fatal("sequence ended before its final frame")
				}
			}
			if p.Ended() {
				t.Fatal("ended before the final frame was displayed")
			}
			d := p.NextDelay()
			if d != 20*time.Millisecond {
				t.Fatalf("final frame delay = %v", d)
			}
			clock.Advance(d)
			if p.Advance() || !p.Ended() || p.Frame() != 2 || p.Loop() != 1 {
				t.Fatalf("end state frame %d ended %v loop %d", p.Frame(), p.Ended(), p.Loop())
			}
			if d := p.NextDelay(); d != 0 {
				t.Fatalf("ended sequence scheduled %v", d)
			}
		})
	}
}

// A late boundary keeps its absolute due time: the overshoot is compensated once
// and a long stall does not turn into a burst of catch-up ticks.
func TestPlaybackBoundariesDoNotDrift(t *testing.T) {
	p, clock := newTestPlayback(t, 4, 0, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	if d := p.NextDelay(); d != 40*time.Millisecond {
		t.Fatalf("boundary = %v", d)
	}
	clock.Advance(45 * time.Millisecond) // 5ms late
	if !p.Advance() {
		t.Fatal("advance rejected")
	}
	if d := p.NextDelay(); d != 35*time.Millisecond {
		t.Fatalf("late boundary = %v, want the 5ms overshoot compensated", d)
	}
	clock.Advance(40 * time.Millisecond) // 5ms late again
	if !p.Advance() {
		t.Fatal("advance rejected")
	}
	if d := p.NextDelay(); d != 35*time.Millisecond {
		t.Fatalf("drifted boundary = %v", d)
	}
	clock.Advance(time.Hour) // stalled far beyond one frame
	if !p.Advance() {
		t.Fatal("advance rejected")
	}
	if d := p.NextDelay(); d != minFrameDelay {
		t.Fatalf("stall catch-up = %v, want a single floor tick", d)
	}
}

func TestPlaybackSkipIsBounded(t *testing.T) {
	p, _ := newTestPlayback(t, 2000, 0, time.Millisecond, PlaybackPolicy{Autoplay: true, FPS: 1})
	if !p.Advance() {
		t.Fatal("advance rejected")
	}
	if p.Frame() != 1000 {
		t.Fatalf("one second window presented frame %d", p.Frame())
	}
	if p.Frame() > maxSkipPerTick {
		t.Fatalf("skip exceeded its bound: %d", p.Frame())
	}
}

func TestPlaybackTimingFailureStaysBounded(t *testing.T) {
	clock := NewManualClock()
	p := newPlaybackController(&fakeTiming{meta: playbackMeta(3, 0), delays: []time.Duration{time.Second}, fail: true}, PlaybackPolicy{Autoplay: true}, clock)
	if d := p.NextDelay(); d < minFrameDelay || d > defaultFrameDelay {
		t.Fatalf("fallback delay = %v", d)
	}
	if !p.Advance() || p.Frame() != 1 {
		t.Fatalf("advance = %d", p.Frame())
	}
	if d := p.NextDelay(); d < minFrameDelay {
		t.Fatalf("delay after a failed timing query = %v", d)
	}
}

func TestPlaybackStaticSequenceIsInert(t *testing.T) {
	p, _ := newTestPlayback(t, 1, 0, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	if p.Playing() || p.NextDelay() != 0 || p.Advance() || p.Ended() {
		t.Fatalf("static sequence played: %v %v %v", p.Playing(), p.Advance(), p.Ended())
	}
	p.Step(1)
	p.Toggle()
	if p.Frame() != 0 || p.Playing() || p.Frames() != 1 {
		t.Fatalf("static sequence changed: %d %v %d", p.Frame(), p.Playing(), p.Frames())
	}
}

func TestPlaybackAdvanceNeedsVisiblePlayback(t *testing.T) {
	p, _ := newTestPlayback(t, 3, 0, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	p.SetVisible(false)
	if p.Advance() || p.Frame() != 0 {
		t.Fatalf("hidden playback advanced to %d", p.Frame())
	}
	p.SetVisible(true)
	p.Toggle()
	if p.Advance() || p.Frame() != 0 {
		t.Fatalf("paused playback advanced to %d", p.Frame())
	}
}

func TestPlaybackPrefetchIsOneFrameAhead(t *testing.T) {
	p, _ := newTestPlayback(t, 3, 0, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	if f, ok := p.Prefetch(); !ok || f != 1 {
		t.Fatalf("prefetch = %d %v", f, ok)
	}
	p.Advance()
	if f, ok := p.Prefetch(); !ok || f != 2 {
		t.Fatalf("prefetch = %d %v", f, ok)
	}
	p.Advance()
	if f, ok := p.Prefetch(); !ok || f != 0 {
		t.Fatalf("wrap prefetch = %d %v", f, ok)
	}
	p.Toggle()
	if _, ok := p.Prefetch(); ok {
		t.Fatal("paused playback prefetched")
	}
}

func TestPlaybackPrefetchSkipsFinalWrap(t *testing.T) {
	p, _ := newTestPlayback(t, 2, 1, 40*time.Millisecond, PlaybackPolicy{Autoplay: true})
	if f, ok := p.Prefetch(); !ok || f != 1 {
		t.Fatalf("prefetch = %d %v", f, ok)
	}
	p.Advance()
	if _, ok := p.Prefetch(); ok {
		t.Fatal("prefetched the wrap that ends the sequence")
	}
}

func TestPlaybackFinalFramePresentedOnEnd(t *testing.T) {
	p, _ := newTestPlayback(t, 10, 1, 20*time.Millisecond, PlaybackPolicy{Autoplay: true, FPS: 10})
	if !p.Advance() || p.Frame() != 5 {
		t.Fatalf("capped boundary presented frame %d", p.Frame())
	}
	// The last frame of the last play is presented, never skipped by the cap.
	if !p.Advance() || p.Frame() != 9 || p.Ended() {
		t.Fatalf("final frame %d ended %v", p.Frame(), p.Ended())
	}
	if p.Advance() {
		t.Fatal("sequence did not end")
	}
	if p.Frame() != 9 || !p.Ended() || p.Loop() != 1 {
		t.Fatalf("end state frame %d ended %v loop %d", p.Frame(), p.Ended(), p.Loop())
	}
}

func TestPlaybackNilClockUsesSystemClock(t *testing.T) {
	p := newPlaybackController(&fakeTiming{meta: playbackMeta(2, 0), delays: []time.Duration{time.Second, time.Second}}, PlaybackPolicy{Autoplay: true}, nil)
	if d := p.NextDelay(); d != time.Second {
		t.Fatalf("delay = %v", d)
	}
}

func TestPlaybackWithGIFSource(t *testing.T) {
	pal := color.Palette{color.NRGBA{A: 255}, color.NRGBA{R: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	second := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	second.SetColorIndex(0, 0, 1)
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{2, 2}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	src, err := OpenAnimation(context.Background(), animationFile(t, b.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	clock := NewManualClock()
	playback := newAnimationPlayback(src, PlaybackPolicy{Autoplay: true}, clock)
	if playback == nil {
		t.Fatal("factory returned no controller")
	}
	if playback.Frames() != 2 || playback.Plays() != 0 || !playback.Playing() {
		t.Fatalf("sequence %d %d %v", playback.Frames(), playback.Plays(), playback.Playing())
	}
	if d := playback.NextDelay(); d != 20*time.Millisecond {
		t.Fatalf("first delay = %v", d)
	}
	clock.Advance(20 * time.Millisecond)
	if !playback.Advance() || playback.Frame() != 1 {
		t.Fatalf("advance = %d", playback.Frame())
	}
	if d := playback.NextDelay(); d != 20*time.Millisecond {
		t.Fatalf("second delay = %v", d)
	}
	if !playback.Advance() || playback.Frame() != 0 || playback.Loop() != 2 {
		t.Fatalf("wrap = %d loop %d", playback.Frame(), playback.Loop())
	}
	if _, err := src.DecodeFrame(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
}
