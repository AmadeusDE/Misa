package term

import (
	"image"
	"image/color"
	"testing"
)

func testRaster(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

func snapshotGraphics(t *testing.T, tm *Terminal) (uint64, []GraphicsPlacement) {
	t.Helper()
	var s Snapshot
	tm.Snapshot(&s)
	return s.GraphicsGeneration, s.Graphics
}

func TestGraphicsSnapshotSharesRasterAndBumpsGeneration(t *testing.T) {
	tm := New(6, 10)
	tm.gfx.setEnabled(true)
	first := testRaster(4, 2, color.RGBA{R: 255, A: 255})
	tm.gfx.putImage(screenMain, 1, 0, first, 32)
	tm.gfx.place(&gfxPlacement{
		imageID: 1, screen: screenMain,
		cell: image.Rect(2, 1, 6, 3), source: image.Rect(0, 0, 4, 2),
	})
	gen, got := snapshotGraphics(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %d, want 1", len(got))
	}
	p := got[0]
	if p.ImageID != 1 || p.ImageGeneration != 1 || p.Raster != image.Image(first) {
		t.Errorf("placement = %+v, want image 1 generation 1 sharing the raster", p)
	}
	if p.CellRect != image.Rect(2, 1, 6, 3) || p.Clip != p.CellRect {
		t.Errorf("cell rect %v clip %v", p.CellRect, p.Clip)
	}

	second := testRaster(4, 2, color.RGBA{G: 255, A: 255})
	tm.gfx.putImage(screenMain, 1, 0, second, 32)
	nextGen, got := snapshotGraphics(t, tm)
	if nextGen <= gen {
		t.Fatalf("image-only change kept generation %d", gen)
	}
	if got[0].Raster != image.Image(second) || got[0].ImageGeneration != 2 {
		t.Errorf("replacement = %+v, want the new raster and generation 2", got[0])
	}
	if p.Raster != image.Image(first) {
		t.Error("an old snapshot followed the replacement instead of keeping its raster")
	}
	if gen2, _ := snapshotGraphics(t, tm); gen2 != nextGen {
		t.Errorf("generation moved without a change: %d then %d", nextGen, gen2)
	}
}

func TestGraphicsPlacementIDsReplaceAndDelete(t *testing.T) {
	tm := New(8, 20)
	tm.gfx.setEnabled(true)
	tm.gfx.putImage(screenMain, 1, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.place(&gfxPlacement{imageID: 1, pid: 7, screen: screenMain, cell: image.Rect(0, 0, 2, 2)})
	tm.gfx.place(&gfxPlacement{imageID: 1, pid: 8, screen: screenMain, cell: image.Rect(2, 0, 4, 2)})
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(4, 0, 6, 2)})
	if _, got := snapshotGraphics(t, tm); len(got) != 3 {
		t.Fatalf("placements = %d, want 3", len(got))
	}
	tm.gfx.place(&gfxPlacement{imageID: 1, pid: 7, screen: screenMain, cell: image.Rect(0, 4, 2, 6)})
	if _, got := snapshotGraphics(t, tm); len(got) != 3 || got[0].CellRect.Min.Y != 4 {
		t.Fatalf("placement 7 was not replaced in place: %+v", got)
	}
	tm.gfx.deletePlacement(screenMain, 1, 7)
	if _, got := snapshotGraphics(t, tm); len(got) != 2 {
		t.Fatalf("placements after delete = %d, want 2", len(got))
	}
	tm.gfx.deletePlacementsOf(screenMain, 1)
	if _, got := snapshotGraphics(t, tm); len(got) != 0 {
		t.Fatalf("placements after deleting the image = %d, want 0", len(got))
	}
	if tm.gfx.image(screenMain, 1) == nil {
		t.Error("deleting placements dropped the image data")
	}
	tm.gfx.freeIfUnplaced(screenMain, 1)
	if tm.gfx.image(screenMain, 1) != nil {
		t.Error("image survived deletion")
	}
}

func TestGraphicsDeleteOnOneScreenKeepsTheOther(t *testing.T) {
	tm := New(8, 20)
	tm.gfx.setEnabled(true)
	tm.gfx.putImage(screenMain, 1, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(0, 0, 2, 2)})
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenAlt, cell: image.Rect(0, 0, 2, 2)})
	tm.gfx.deleteOn(screenAlt)
	if _, got := snapshotGraphics(t, tm); len(got) != 1 || got[0].CellRect.Min != image.Pt(0, 0) {
		t.Fatalf("main placements after clearing the alt screen: %+v", got)
	}
	tm.ProcessOutput([]byte("\x1b[?1049h"))
	if _, got := snapshotGraphics(t, tm); len(got) != 0 {
		t.Fatalf("alt screen still shows main placements: %+v", got)
	}
}

func TestGraphicsScrollMovesAndEvicts(t *testing.T) {
	tm := New(10, 20)
	tm.gfx.setEnabled(true)
	tm.gfx.putImage(screenMain, 1, 0, testRaster(4, 4, color.RGBA{A: 255}), 64)
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(0, 3, 4, 5)})
	tm.gfx.scroll(screenMain, 0, 2, 9, 20, true)
	if _, got := snapshotGraphics(t, tm); len(got) != 1 || got[0].CellRect.Min.Y != 1 {
		t.Fatalf("after scrolling two rows: %+v", got)
	}
	tm.gfx.scroll(screenMain, 0, 5, 9, 20, true)
	if _, got := snapshotGraphics(t, tm); len(got) != 0 {
		t.Fatalf("placement stayed after scrolling off screen: %+v", got)
	}
}

func TestGraphicsScrollLeavesOtherRowsAndRegions(t *testing.T) {
	tm := New(10, 20)
	tm.gfx.setEnabled(true)
	tm.gfx.putImage(screenMain, 1, 0, testRaster(4, 4, color.RGBA{A: 255}), 64)
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(0, 1, 4, 3)})
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(4, 4, 8, 6)})
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(8, 8, 12, 10)})
	tm.gfx.scroll(screenMain, 3, 1, 6, 20, true)
	_, got := snapshotGraphics(t, tm)
	if len(got) != 3 {
		t.Fatalf("placements = %d, want 3", len(got))
	}
	if got[0].CellRect.Min.Y != 1 || got[1].CellRect.Min.Y != 3 || got[2].CellRect.Min.Y != 8 {
		t.Errorf("rows after a region scroll = %d,%d,%d, want 1,3,8",
			got[0].CellRect.Min.Y, got[1].CellRect.Min.Y, got[2].CellRect.Min.Y)
	}
}

func TestGraphicsResizeShiftsMainAndClips(t *testing.T) {
	tm := New(10, 20)
	tm.gfx.setEnabled(true)
	tm.gfx.putImage(screenMain, 1, 0, testRaster(4, 4, color.RGBA{A: 255}), 64)
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(0, 5, 4, 7)})
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(4, 9, 8, 11)})
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenAlt, cell: image.Rect(0, 9, 4, 11)})
	tm.gfx.resize(3, 6, 20)
	_, got := snapshotGraphics(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements after resize = %+v, want only the shifted one", got)
	}
	if got[0].CellRect.Min.Y != 2 {
		t.Errorf("main placement row = %d, want 2 after a three-row shift", got[0].CellRect.Min.Y)
	}
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(0, 0, 4, 2)})
	tm.Resize(1, 20)
	_, got = snapshotGraphics(t, tm)
	if len(got) != 1 || got[0].CellRect.Min.Y != 0 || got[0].Clip != image.Rect(0, 0, 4, 1) {
		t.Fatalf("clip after shrinking to one row: %+v", got)
	}
}

func TestGraphicsClearResetsEverything(t *testing.T) {
	tm := New(6, 10)
	tm.gfx.setEnabled(true)
	tm.gfx.putImage(screenMain, 1, 3, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(0, 0, 2, 2)})
	gen, _ := snapshotGraphics(t, tm)
	epoch := tm.gfx.epoch
	tm.gfx.clear()
	nextGen, got := snapshotGraphics(t, tm)
	if len(got) != 0 || nextGen <= gen {
		t.Fatalf("clear left %d placements, generation %d -> %d", len(got), gen, nextGen)
	}
	if tm.gfx.epoch <= epoch {
		t.Error("clear did not invalidate work in flight")
	}
	if tm.gfx.image(screenMain, 1) != nil || tm.gfx.imageByNumber(screenMain, 3) != nil {
		t.Error("clear kept image data or numbers")
	}
}

func TestGraphicsLimitsEvictOldest(t *testing.T) {
	tm := New(8, 20)
	tm.gfx.setEnabled(true)
	tm.gfx.setLimits(graphicsLimits{imageWidth: 64, imageHeight: 64, imageBytes: 1 << 20, totalBytes: 1 << 20,
		images: 2, placements: 2, inFlightBytes: 1 << 20, payloadBytes: 1024})
	tm.gfx.putImage(screenMain, 1, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.putImage(screenMain, 2, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.putImage(screenMain, 3, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	if tm.gfx.image(screenMain, 1) != nil || tm.gfx.image(screenMain, 2) == nil || tm.gfx.image(screenMain, 3) == nil {
		t.Fatalf("images after eviction: 1=%v 2=%v 3=%v", tm.gfx.image(screenMain, 1), tm.gfx.image(screenMain, 2), tm.gfx.image(screenMain, 3))
	}
	for i := 0; i < 4; i++ {
		tm.gfx.place(&gfxPlacement{imageID: 2, screen: screenMain, cell: image.Rect(i, 0, i+1, 1)})
	}
	if _, got := snapshotGraphics(t, tm); len(got) != 2 || got[0].CellRect.Min.X != 2 {
		t.Fatalf("placements after eviction: %+v", got)
	}
}

func TestGraphicsByteBudgetEvicts(t *testing.T) {
	tm := New(4, 8)
	tm.gfx.setEnabled(true)
	tm.gfx.setLimits(graphicsLimits{imageWidth: 64, imageHeight: 64, imageBytes: 1 << 20, totalBytes: 40,
		images: 8, placements: 8, inFlightBytes: 1 << 20, payloadBytes: 1024})
	tm.gfx.putImage(screenMain, 1, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.putImage(screenMain, 2, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.putImage(screenMain, 3, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	if tm.gfx.total > 40 {
		t.Fatalf("stored %d bytes over a 40 byte budget", tm.gfx.total)
	}
	if tm.gfx.image(screenMain, 1) != nil {
		t.Error("the oldest image survived the byte budget")
	}
}

func TestGraphicsSnapshotKeepsSlicesSmall(t *testing.T) {
	tm := New(4, 8)
	tm.gfx.setEnabled(true)
	tm.gfx.putImage(screenMain, 1, 0, testRaster(2, 2, color.RGBA{A: 255}), 16)
	tm.gfx.place(&gfxPlacement{imageID: 1, screen: screenMain, cell: image.Rect(0, 0, 2, 2)})
	var s Snapshot
	tm.Snapshot(&s)
	held := s.Graphics[0].Raster
	tm.gfx.dropScreen(screenMain)
	tm.Snapshot(&s)
	if len(s.Graphics) != 0 {
		t.Fatalf("snapshot kept %d placements after deletion", len(s.Graphics))
	}
	if held == nil {
		t.Fatal("snapshot lost its raster")
	}
}
