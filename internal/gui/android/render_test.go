package android

import (
	"image"
	"testing"
)

// renderDensity is the surface density every render test configures.
const renderDensity = 3

// requireImage proves the window shows an image: marker's quadrant at the
// top left, then the three quadrants every render helper image shares.
func requireImage(r *renderRig, marker [4]uint8) (x, y, cellW, cellH int) {
	r.t.Helper()
	x, y, cellW, cellH = r.waitForColor(marker)
	pix, stride, _, _ := surfacePixels(r.surf)
	for _, tc := range []struct {
		name string
		px   image.Point
		want [4]uint8
	}{
		{"top right", image.Pt(x+cellW, y), renderGreen},
		{"bottom left", image.Pt(x, y+cellH), renderBlue},
		{"translucent", image.Pt(x+cellW, y+cellH), renderHalf},
	} {
		if got := at(pix, stride, tc.px.X, tc.px.Y); got != tc.want {
			r.t.Fatalf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
	return x, y, cellW, cellH
}

func TestRenderSessionPresentsKittyImageAsPremultipliedRGBA(t *testing.T) {
	rig := startRenderRig(t, 720, 1200)
	rig.resize(renderDensity)
	rig.waitPainted(1)
	x, y, cw, ch := rig.waitForPixels()

	pix, stride, w, h := surfacePixels(rig.surf)
	for _, tc := range []struct {
		name string
		px   image.Point
		want [4]uint8
	}{
		{"image origin", image.Pt(x, y), renderRed},
		{"top right", image.Pt(x+cw, y), renderGreen},
		{"bottom left", image.Pt(x, y+ch), renderBlue},
		{"translucent", image.Pt(x+cw, y+ch), renderHalf},
		{"background", image.Pt(w-1, h-91), renderBG},
	} {
		if got := at(pix, stride, tc.px.X, tc.px.Y); got != tc.want {
			t.Fatalf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
	if x+cw >= w || y+ch >= h {
		t.Fatalf("image at %d,%d cell %dx%d leaves the %dx%d window", x, y, cw, ch, w, h)
	}
	rig.requireSingleEmit()
}

func TestRenderSessionRebuildsImageAfterSurfaceRecreated(t *testing.T) {
	rig := startRenderRig(t, 720, 1200)
	rig.resize(renderDensity)
	rig.waitPainted(1)
	requireImage(rig, renderRed)

	rig.m.OnSurfaceDestroyed()
	if rig.m.SessionCount() != 1 || rig.m.ActiveSession() != rig.id {
		t.Fatalf("surface loss ended the session: count %d active %d", rig.m.SessionCount(), rig.m.ActiveSession())
	}

	next := newRenderSurface(rig.w, rig.h)
	rig.attach(next)
	rig.resize(renderDensity)
	requireFull(t, next, 1)
	requireImage(rig, renderRed)
	rig.requireSingleEmit()
}

func TestRenderSessionRebuildsImageAfterLowMemory(t *testing.T) {
	rig := startRenderRig(t, 720, 1200)
	rig.resize(renderDensity)
	rig.waitPainted(1)
	requireImage(rig, renderRed)
	before := surfaceFull(rig.surf)
	quiet(t, rig.surf, before)

	rig.m.OnLowMemory()
	requireFull(t, rig.surf, before+1)
	requireImage(rig, renderRed)
	rig.requireSingleEmit()
}

func TestRenderSessionRedrawsOnDensityChange(t *testing.T) {
	rig := startRenderRig(t, 720, 1200)
	rig.resize(renderDensity)
	rig.waitPainted(1)
	_, _, cellW, cellH := rig.waitForPixels()
	before := surfaceFull(rig.surf)
	quiet(t, rig.surf, before)

	rig.resize(4)
	requireFull(t, rig.surf, before+1)
	_, _, wider, taller := requireImage(rig, renderRed)
	if wider <= cellW || taller <= cellH {
		t.Fatalf("cell %dx%d at density 4 does not grow from %dx%d", wider, taller, cellW, cellH)
	}
	rig.requireSingleEmit()
}

func TestRenderSessionGatesTabsAndBackground(t *testing.T) {
	rig := startRenderRig(t, 720, 1200)
	rig.resize(renderDensity)
	rig.waitPainted(1)
	x, y, cw, ch := requireImage(rig, renderRed)
	first := rig.surf
	before := surfaceFull(first)
	quiet(t, first, before)

	// A backgrounded Activity presents nothing, not even for the invalidation
	// low memory deferred into the hidden session.
	rig.m.OnVisibility(false)
	rig.m.OnLowMemory()
	quiet(t, first, before)
	rig.m.OnVisibility(true)
	requireFull(t, first, before+1)
	requireImage(rig, renderRed)
	if rig.m.ActiveSession() != rig.id || rig.m.SessionCount() != 1 {
		t.Fatalf("foregrounding lost the session: active %d count %d", rig.m.ActiveSession(), rig.m.SessionCount())
	}

	// A second tab takes the window with its own image.
	t.Setenv(renderTintEnv, "yellow")
	second := rig.m.NewSession()
	if second == 0 || second == rig.id || rig.m.ActiveSession() != second {
		t.Fatalf("second tab %d, active %d", second, rig.m.ActiveSession())
	}
	rig.resize(renderDensity)
	rig.waitPainted(2)
	requireFull(t, first, before+2)
	if x2, y2, cw2, ch2 := requireImage(rig, renderYellow); x2 != x || y2 != y || cw2 != cw || ch2 != ch {
		t.Fatalf("second tab image at %d,%d cell %dx%d, want %d,%d %dx%d", x2, y2, cw2, ch2, x, y, cw, ch)
	}

	// Low memory invalidates both tabs; only the visible one repaints.
	rig.m.OnLowMemory()
	requireFull(t, first, before+3)
	requireImage(rig, renderYellow)

	// Switching back redraws the first tab from its own image store: red
	// again, never the stale window content of the second tab.
	rig.m.SwitchSession(rig.id)
	requireFull(t, first, before+4)
	requireImage(rig, renderRed)

	// Neither child wrote twice, so every frame above came from a terminal's
	// own image store.
	rig.requireEmits(2)
}
