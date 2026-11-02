package gui

import (
	"encoding/base64"
	"image"
	"testing"

	"github.com/AmadeusDE/Misa/internal/term"
)

// imageAPC is a one-pixel RGB image the terminal stores and displays in the
// cell under the cursor.
func imageAPC() []byte {
	body := "a=T,f=24,s=1,v=1,i=1,c=1,r=1;" + base64.StdEncoding.EncodeToString([]byte{255, 0, 0})
	return []byte("\x1b_G" + body + "\x1b\\")
}

// coldBackend is a rotating backend that leaves damage tracking to the core's
// per-buffer frame history instead of a buffer it owns.
type coldBackend struct{ *fakeBackend }

func (c coldBackend) Buffer(pw, ph int) *Target {
	t := c.fakeBackend.Buffer(pw, ph)
	if t != nil {
		t.content = nil
	}
	return t
}

// idBackend hands out a fresh buffer id every frame, like a backend that
// retires and reallocates its buffers.
type idBackend struct {
	*fakeBackend
	id uint64
}

func (b *idBackend) Buffer(pw, ph int) *Target {
	t := b.fakeBackend.Buffer(pw, ph)
	if t == nil {
		return nil
	}
	b.id++
	return &Target{Pix: t.Pix, Stride: t.Stride, ID: b.id}
}

func TestInvalidateDiscardsCachesButKeepsTerminalImages(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, coldBackend{fb})
	a.logicalW, a.logicalH = 200, 80
	rows, cols := a.gridSize()
	a.term = term.New(rows, cols)
	a.term.SetGraphicsEnabled(true)
	a.term.ProcessOutput(imageAPC())
	a.requestDraw()
	if fb.frameCount() != 1 {
		t.Fatalf("frames %d", fb.frameCount())
	}
	var before term.Snapshot
	a.term.Snapshot(&before)
	if len(before.Graphics) != 1 || a.rend.gfx.scratchOf == nil || len(a.frames) == 0 || !a.shown.valid {
		t.Fatalf("image %d scratch %v frames %d shown %v", len(before.Graphics), a.rend.gfx.scratchOf != nil, len(a.frames), a.shown.valid)
	}
	drops := fb.drops

	fb.canPresent = false
	a.Handle(Event{Kind: EventInvalidate})

	if a.rend.gfx.scratchOf != nil || a.rend.gfx.scratch != nil || a.rend.gfx.generic != nil || a.rend.gfx.cols != nil {
		t.Fatalf("compositor scratch survived: %+v", a.rend.gfx)
	}
	if a.snap.Cells != nil || a.snap.Graphics != nil {
		t.Fatalf("snapshot refs survived: cells %d graphics %d", len(a.snap.Cells), len(a.snap.Graphics))
	}
	if a.shown.valid || len(a.shown.cells) != 0 || a.shown.gfx != nil || a.shown.rows != 0 || a.shown.cols != 0 {
		t.Fatalf("shown frame survived: %+v", a.shown)
	}
	if a.frames != nil {
		t.Fatalf("per-buffer frames survived: %d", len(a.frames))
	}
	if fb.drops != drops+1 {
		t.Fatalf("drops %d, want %d", fb.drops, drops+1)
	}
	if !a.drawPending {
		t.Fatal("hidden redraw was not left pending")
	}
	if fb.frameCount() != 1 {
		t.Fatalf("hidden invalidate presented %d frames", fb.frameCount()-1)
	}

	var after term.Snapshot
	a.term.Snapshot(&after)
	if len(after.Graphics) != 1 || after.Graphics[0].Raster != before.Graphics[0].Raster {
		t.Fatal("the terminal's protocol image store was dropped")
	}
}

func TestInvalidateWhileHiddenRedrawsWholeFrameOnce(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 200, 80
	rows, cols := a.gridSize()
	a.term = term.New(rows, cols)
	a.term.SetGraphicsEnabled(true)
	a.term.ProcessOutput(imageAPC())
	a.requestDraw()
	if fb.frameCount() != 1 {
		t.Fatalf("frames %d", fb.frameCount())
	}

	fb.canPresent = false
	for i := 0; i < 3; i++ {
		a.Handle(Event{Kind: EventInvalidate})
	}
	if fb.frameCount() != 1 {
		t.Fatalf("hidden invalidations presented %d frames", fb.frameCount()-1)
	}
	fb.canPresent = true
	a.Handle(Event{Kind: EventResize, Width: a.logicalW, Height: a.logicalH, Scale: a.scale})
	if fb.frameCount() != 2 {
		t.Fatalf("reattachment presented %d frames, want 1", fb.frameCount()-1)
	}
	pw, ph := a.pixelSize()
	if want := image.Rect(0, 0, pw, ph); len(fb.frames[1].damage) != 1 || fb.frames[1].damage[0] != want {
		t.Fatalf("reattachment damage %v, want %v", fb.frames[1].damage, want)
	}
	if !hasOpaqueRed(fb.frames[1].pix) {
		t.Fatal("the rebuilt frame lost the terminal's image")
	}
	a.Handle(Event{Kind: EventFrame})
	if fb.frameCount() != 2 {
		t.Fatalf("a settled frame presented again: %d", fb.frameCount())
	}
}

func TestInvalidateOnAVisibleFrameRepaintsWholeFrameOnce(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 200, 80
	rows, cols := a.gridSize()
	a.term = term.New(rows, cols)
	a.term.SetGraphicsEnabled(true)
	a.term.ProcessOutput(imageAPC())
	a.requestDraw()

	a.Handle(Event{Kind: EventInvalidate})
	if fb.frameCount() != 2 {
		t.Fatalf("frames %d, want 2", fb.frameCount())
	}
	pw, ph := a.pixelSize()
	if len(fb.frames[1].damage) != 1 || fb.frames[1].damage[0] != image.Rect(0, 0, pw, ph) {
		t.Fatalf("damage %v, want the whole buffer", fb.frames[1].damage)
	}
	if !hasOpaqueRed(fb.frames[1].pix) {
		t.Fatal("the repaint lost the image")
	}
	a.Handle(Event{Kind: EventInvalidate})
	if fb.frameCount() != 3 {
		t.Fatalf("a second invalidate presented %d frames", fb.frameCount()-2)
	}
}

func TestInvalidateBoundsFrameHistoryToTheLiveBuffer(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, &idBackend{fakeBackend: fb})
	a.logicalW, a.logicalH = 200, 80
	rows, cols := a.gridSize()
	a.term = term.New(rows, cols)
	a.term.ProcessOutput([]byte("x"))
	a.requestDraw()
	if len(a.frames) != 1 {
		t.Fatalf("frames %d, want 1", len(a.frames))
	}
	for i := 0; i < 5; i++ {
		a.Handle(Event{Kind: EventInvalidate})
		if len(a.frames) > 1 {
			t.Fatalf("invalidation %d left %d frame histories", i, len(a.frames))
		}
		if fb.frameCount() != i+2 {
			t.Fatalf("frames %d after invalidation %d, want %d", fb.frameCount(), i, i+2)
		}
	}
}

// hasOpaqueRed reports the fake backend's BGRA pixel of the image.
func hasOpaqueRed(pix []byte) bool {
	for i := 0; i+3 < len(pix); i += 4 {
		if pix[i] == 0 && pix[i+1] == 0 && pix[i+2] == 255 && pix[i+3] == 255 {
			return true
		}
	}
	return false
}
