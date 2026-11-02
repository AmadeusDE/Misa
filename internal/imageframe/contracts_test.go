package imageframe

import (
	"context"
	"image"
	"testing"
	"time"
)

type fakeSource struct{ closed bool }

func (f *fakeSource) Metadata() SequenceMetadata {
	return SequenceMetadata{ID: 9, Size: image.Pt(2, 2), Frames: 1, Plays: 1}
}
func (f *fakeSource) DecodeFrame(ctx context.Context, index int) (DecodedFrame, error) {
	if err := ctx.Err(); err != nil {
		return DecodedFrame{}, err
	}
	return DecodedFrame{Index: index, Pixels: image.NewNRGBA(image.Rect(0, 0, 2, 2)), Duration: time.Second}, nil
}
func (f *fakeSource) Close() error { f.closed = true; return nil }

type fakeClock struct {
	now    time.Time
	timers []*fakeTimer
}
type fakeTimer struct {
	ch      chan time.Time
	stopped bool
}

func (f *fakeClock) Now() time.Time { return f.now }
func (f *fakeClock) NewTimer(time.Duration) Timer {
	timer := &fakeTimer{ch: make(chan time.Time, 1)}
	f.timers = append(f.timers, timer)
	return timer
}
func (f *fakeTimer) C() <-chan time.Time { return f.ch }
func (f *fakeTimer) Stop() bool {
	if f.stopped {
		return false
	}
	f.stopped = true
	return true
}
func (f *fakeTimer) fire(t time.Time) {
	if !f.stopped {
		f.ch <- t
		f.stopped = true
	}
}

func TestFrameSourceCancellationAndClose(t *testing.T) {
	f := &fakeSource{}
	var source FrameSource = f
	if source.Metadata().Frames != 1 {
		t.Fatal("metadata")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.DecodeFrame(ctx, 0); err != context.Canceled {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil || !f.closed {
		t.Fatalf("close: %v", err)
	}
}
func TestClockTimerCancellation(t *testing.T) {
	f := &fakeClock{}
	var clock Clock = f
	timer := clock.NewTimer(time.Second)
	if !timer.Stop() || timer.Stop() {
		t.Fatal("timer stop state")
	}
	f.timers[0].fire(clock.Now())
	select {
	case <-timer.C():
		t.Fatal("stopped timer fired")
	default:
	}
	next := clock.NewTimer(time.Second)
	f.timers[1].fire(clock.Now())
	select {
	case <-next.C():
	default:
		t.Fatal("active timer did not fire")
	}
	if next.Stop() {
		t.Fatal("fired timer stopped")
	}
}
func TestPlaybackStateActive(t *testing.T) {
	for _, tt := range []struct {
		state PlaybackState
		want  bool
	}{
		{PlaybackState{}, false}, {PlaybackState{Playing: true}, false},
		{PlaybackState{Visible: true}, false}, {PlaybackState{Playing: true, Visible: true}, true},
	} {
		if got := tt.state.Active(); got != tt.want {
			t.Errorf("%+v Active = %v", tt.state, got)
		}
	}
}

func TestFrameGenerationIsDistinctFromPresentationGeneration(t *testing.T) {
	frame := Frame{Generation: 7, Presentation: Presentation{Generation: 12}}
	if frame.Generation != 7 || frame.Presentation.Generation != 12 {
		t.Fatal(frame)
	}
}

func TestAcknowledgementIdentifiesEpoch(t *testing.T) {
	ack := Acknowledgement{Generation: 4, Presentation: Presentation{Generation: 1}}
	if ack.Generation != 4 || ack.Presentation.Generation != 1 {
		t.Fatal(ack)
	}
}
