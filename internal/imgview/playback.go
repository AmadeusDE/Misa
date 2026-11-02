package imgview

import (
	"time"

	"github.com/AmadeusDE/Misa/internal/imageframe"
	"github.com/AmadeusDE/Misa/internal/imgview/animdecode"
)

const (
	// minFrameDelay keeps a zero-length frame from spinning the scheduler.
	minFrameDelay = time.Millisecond
	// defaultFrameDelay is used while a source cannot report a duration.
	defaultFrameDelay = 100 * time.Millisecond
	// maxSkipPerTick bounds the frames one boundary may cross when the fps cap
	// makes a single presentation window cover many short frames.
	maxSkipPerTick = 1024
)

// frameTiming is the read-only sequence view the controller needs. It is
// satisfied by *animdecode.Source and by test doubles.
type frameTiming interface {
	Metadata() imageframe.SequenceMetadata
	FrameTiming(int) (time.Duration, time.Duration, error)
}

// playbackController schedules animation presentation. It owns timing, loop
// accounting, stepping and pause/visibility state; it never decodes, writes
// output or owns a timer. The viewer asks NextDelay, waits that long, then calls
// Advance, so at most one boundary and one decode are outstanding. It is not
// concurrency-safe: call it from the UI loop that owns the sequence.
type playbackController struct {
	timing      frameTiming
	clock       imageframe.Clock
	frames      int
	total       uint64        // total plays; zero is infinite
	window      time.Duration // effective presentation window
	fpsWindow   time.Duration
	minInterval time.Duration
	visible     bool
	playing     bool // user intent, independent of visibility
	frame       int  // frame the viewer should present next
	plays       uint64
	ended       bool
	base        time.Duration // delay served for the current frame
	waiting     bool
	deadline    time.Time // absolute due time of the armed boundary
}

var _ animationPlayback = (*playbackController)(nil)

func init() { newPlayback = newAnimationPlayback }

func newAnimationPlayback(src *animdecode.Source, policy PlaybackPolicy, clock imageframe.Clock) animationPlayback {
	return newPlaybackController(src, policy, clock)
}

func newPlaybackController(timing frameTiming, policy PlaybackPolicy, clock imageframe.Clock) *playbackController {
	p := &playbackController{timing: timing, clock: clock, visible: true, base: defaultFrameDelay}
	if p.clock == nil {
		p.clock = SystemClock{}
	}
	if timing != nil {
		meta := timing.Metadata()
		p.frames = meta.Frames
		p.total = meta.Plays
	}
	switch policy.Loops {
	case LoopOnce:
		p.total = 1
	case LoopInfinite:
		p.total = 0
	}
	if policy.FPS > 0 {
		p.fpsWindow = time.Second / time.Duration(policy.FPS)
	}
	p.minInterval = max(0, policy.MinInterval)
	p.window = max(p.fpsWindow, p.minInterval)
	p.playing = policy.Autoplay
	p.base = p.initialDelay()
	return p
}

// SetMinInterval updates the bandwidth window while preserving time already
// spent on the current boundary. A host's already-armed timer may fire early;
// PresentationPending tells it to re-arm instead of advancing prematurely.
func (p *playbackController) SetMinInterval(d time.Duration) {
	d = max(0, d)
	if d == p.minInterval {
		return
	}
	old := p.initialDelay()
	p.minInterval = d
	p.window = max(p.fpsWindow, d)
	delta := p.initialDelay() - old
	if p.waiting {
		p.deadline = p.deadline.Add(delta)
	} else {
		p.base = max(0, p.base+delta)
	}
}

// PresentationPending reports an old timer firing before a revised deadline.
func (p *playbackController) PresentationPending() bool {
	return p.waiting && p.clock.Now().Before(p.deadline)
}

func (p *playbackController) Frames() int { return p.frames }
func (p *playbackController) Frame() int  { return p.frame }

// Playing reports effective playback: user intent, visibility, an unfinished
// sequence and more than one frame.
func (p *playbackController) Playing() bool {
	return p.playing && p.visible && !p.ended && p.frames > 1
}

func (p *playbackController) Ended() bool { return p.ended }

// Plays is the total play count; zero means the sequence loops forever.
func (p *playbackController) Plays() uint64 { return p.total }

// Loop is the one-based play number currently shown, capped at Plays.
func (p *playbackController) Loop() uint64 {
	if p.ended {
		return max(1, p.plays)
	}
	loop := p.plays + 1
	if p.total > 0 && loop > p.total {
		loop = p.total
	}
	return loop
}

// SetVisible suspends or resumes playback. Hiding freezes the remaining delay and
// keeps the position, the play counter and a user's own pause untouched.
func (p *playbackController) SetVisible(visible bool) {
	if p.visible == visible {
		return
	}
	if !visible {
		p.freeze()
	}
	p.visible = visible
}

// Toggle is the Ctrl+a action. A finished finite sequence restarts from its first
// frame; otherwise the saved position resumes.
func (p *playbackController) Toggle() {
	if p.frames <= 1 {
		return
	}
	if p.ended {
		p.ended = false
		p.plays = 0
		p.frame = 0
		p.playing = true
		p.waiting = false
		p.base = p.initialDelay()
		return
	}
	if p.playing {
		p.playing = false
		p.freeze()
		return
	}
	p.playing = true
}

// Step pauses and moves by n frames, clamped to the sequence. It never changes
// the play counter or the ended state, and a clamped step still reports the
// frame to present so the host can redraw.
func (p *playbackController) Step(n int) {
	if p.frames <= 1 || n == 0 {
		return
	}
	p.playing = false
	p.freeze()
	p.frame = max(0, min(p.frame+n, p.frames-1))
	p.waiting = false
	p.base = p.initialDelay()
}

// NextDelay is the wait before the next boundary. Calling it again for the same
// boundary returns the time left, and a resumed boundary keeps what it had left,
// so a hidden or paused owner never catches up in a burst.
func (p *playbackController) NextDelay() time.Duration {
	if !p.Playing() {
		return 0
	}
	now := p.clock.Now()
	if p.waiting {
		return max(p.deadline.Sub(now), 0)
	}
	p.waiting = true
	p.base = max(p.base, minFrameDelay)
	p.deadline = now.Add(p.base)
	return p.base
}

// Advance consumes one boundary and reports whether playback continues. The wait
// it schedules is measured on the source timeline, so frames shorter than the fps
// window are skipped instead of stretching the animation, and a late boundary
// keeps its absolute due time instead of drifting or bursting. The final frame of
// a finite sequence is always presented and keeps its own delay before playback
// reports ended.
func (p *playbackController) Advance() bool {
	if !p.Playing() {
		return false
	}
	now := p.clock.Now()
	armed := p.waiting
	p.waiting = false
	b := p.boundary()
	if b.ended {
		p.ended = true
		p.frame = b.frame
		if p.total > 0 {
			p.plays = p.total
		}
		p.base = 0
		return false
	}
	p.frame = b.frame
	p.plays += b.wraps
	wait := p.initialDelay()
	if armed {
		// The boundary was due at p.deadline, so the next one keeps that
		// absolute due time plus its source interval: a late tick is compensated
		// once instead of drifting or queueing a burst of catch-up frames.
		wait = p.deadline.Add(wait).Sub(now)
	}
	p.base = max(wait, minFrameDelay, p.minInterval)
	return true
}

// Prefetch reports the frame the next boundary will present, for hosts that
// decode early. It is false while paused, hidden or ended.
func (p *playbackController) Prefetch() (int, bool) {
	if !p.Playing() {
		return 0, false
	}
	b := p.boundary()
	if b.ended {
		return 0, false
	}
	return b.frame, true
}

// boundaryStep is the next presentation on the source timeline. wait covers every
// delay from the presented frame up to that presentation, so the animation keeps
// its source duration. ended means the final frame's delay has elapsed and the
// sequence is over.
type boundaryStep struct {
	frame int
	wait  time.Duration
	wraps uint64
	ended bool
}

// boundary walks at least one frame forward and keeps walking while the elapsed
// source time is still below one fps window. It never skips the final frame of a
// finite sequence, and it never wraps past the last play.
func (p *playbackController) boundary() boundaryStep {
	if p.frames <= 1 {
		return boundaryStep{frame: p.frame, wait: p.frameDelay(p.frame)}
	}
	wait := p.frameDelay(p.frame)
	frame := p.frame + 1
	wraps := uint64(0)
	for steps := 0; ; steps++ {
		last := p.total > 0 && p.plays+wraps+1 >= p.total
		if frame >= p.frames {
			if last {
				return boundaryStep{frame: p.frames - 1, wait: wait, wraps: wraps, ended: true}
			}
			wraps++
			frame = 0
		}
		// The last frame of the last play is always presented, even when the fps
		// window would otherwise skip past it.
		if wait >= p.window || last && frame == p.frames-1 || steps >= maxSkipPerTick {
			return boundaryStep{frame: frame, wait: wait, wraps: wraps}
		}
		wait += p.frameDelay(frame)
		frame++
	}
}

func (p *playbackController) initialDelay() time.Duration {
	return max(p.boundary().wait, minFrameDelay, p.minInterval)
}

func (p *playbackController) frameDelay(i int) time.Duration {
	if p.timing != nil {
		if _, d, err := p.timing.FrameTiming(i); err == nil {
			return max(d, minFrameDelay)
		}
	}
	return defaultFrameDelay
}

// freeze stores the part of the current boundary that has not elapsed yet.
func (p *playbackController) freeze() {
	if !p.waiting {
		return
	}
	p.waiting = false
	p.base = max(p.deadline.Sub(p.clock.Now()), 0)
}
