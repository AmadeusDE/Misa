package term

import (
	"encoding/base64"
	"image"
	"testing"
)

// gfxAPC wraps a graphics body in the APC introducer and ST terminator.
func gfxAPC(body string) []byte { return []byte("\x1b_G" + body + "\x1b\\") }

// gfxPixels is base64 for a zero-filled image of w x h pixels with bpp bytes
// each, which is what a direct transmission carries.
func gfxPixels(w, h, bpp int) string {
	return base64.StdEncoding.EncodeToString(make([]byte, w*h*bpp))
}

func gfxTerm(t *testing.T, rows, cols int) *Terminal {
	t.Helper()
	tm := New(rows, cols)
	tm.SetGraphicsEnabled(true)
	return tm
}

func gfxSnapshot(t *testing.T, tm *Terminal) (uint64, []GraphicsPlacement) {
	t.Helper()
	var s Snapshot
	tm.Snapshot(&s)
	return s.GraphicsGeneration, s.Graphics
}

func TestGraphicsTransmitThenPlace(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=2,v=2;" + gfxPixels(2, 2, 4)))
	tm.ProcessOutput(gfxAPC("a=p,i=1,c=2,r=1"))
	gen, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	p := got[0]
	if p.ImageID != 1 || p.ImageGeneration != 1 {
		t.Errorf("placement = %+v, want image 1 generation 1", p)
	}
	if p.CellRect != image.Rect(0, 0, 2, 1) || p.Clip != p.CellRect {
		t.Errorf("cell rect %v clip %v, want 2x1 at the cursor", p.CellRect, p.Clip)
	}
	if p.Source != image.Rect(0, 0, 2, 2) {
		t.Errorf("source %v, want the whole 2x2 image", p.Source)
	}
	if bounds := p.Raster.Bounds(); bounds != image.Rect(0, 0, 2, 2) {
		t.Errorf("raster bounds %v", bounds)
	}
	if gen == 0 {
		t.Error("generation stayed zero")
	}
}

func TestGraphicsPlaceDerivesCellsFromPixels(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=2,s=11,v=21;" + gfxPixels(11, 21, 4)))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	if want := image.Rect(0, 0, 3, 3); got[0].CellRect != want {
		t.Errorf("cell rect %v, want %v from 11x21 pixels in 5x10 cells", got[0].CellRect, want)
	}
}

func TestGraphicsPlaceWithoutCellMetricsUsesFallback(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=25,v=45;" + gfxPixels(25, 45, 4)))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	if want := image.Rect(0, 0, 3, 3); got[0].CellRect != want {
		t.Errorf("cell rect %v, want %v from the 10x20 fallback", got[0].CellRect, want)
	}
}

func TestGraphicsSourceRectangle(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=8,v=8,x=2,y=4,w=4,h=2;" + gfxPixels(8, 8, 4)))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	if want := image.Rect(2, 4, 6, 6); got[0].Source != want {
		t.Errorf("source %v, want %v", got[0].Source, want)
	}
	if want := image.Rect(0, 0, 1, 1); got[0].CellRect != want {
		t.Errorf("cell rect %v, want %v", got[0].CellRect, want)
	}
}

func TestGraphicsCursorMovement(t *testing.T) {
	for _, c := range []struct {
		name   string
		keys   string
		column int
	}{{"default advances", "", 3}, {"C=1 stays", ",C=1", 1}} {
		t.Run(c.name, func(t *testing.T) {
			tm := gfxTerm(t, 10, 20)
			tm.SetCellPixels(5, 10)
			tm.ProcessOutput([]byte("\x1b[2G"))
			tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=6,v=4,c=2,r=1" + c.keys + ";" + gfxPixels(6, 4, 4)))
			if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].CellRect.Min.X != 1 {
				t.Fatalf("placement anchored at column %+v, want 1", got)
			}
			if col := tm.CursorCol(); col != c.column {
				t.Errorf("cursor column = %d, want %d", col, c.column)
			}
		})
	}
}

func TestGraphicsChunkedTransferAcrossReads(t *testing.T) {
	payload := gfxPixels(4, 4, 4)
	first := payload[:len(payload)/2]
	second := payload[len(payload)/2:]
	stream := append(gfxAPC("a=t,f=32,i=9,s=4,v=4,m=1;"+first), gfxAPC("m=0;"+second)...)
	tm := gfxTerm(t, 10, 20)
	for i := range stream {
		tm.ProcessOutput(stream[i : i+1])
	}
	tm.ProcessOutput(gfxAPC("a=p,i=9,c=1,r=1"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].ImageID != 9 {
		t.Fatalf("placements = %+v, want image 9", got)
	}
}

func TestGraphicsTextAroundAPC(t *testing.T) {
	tm := gfxTerm(t, 4, 20)
	tm.ProcessOutput([]byte("ab"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1,C=1;" + gfxPixels(2, 2, 4)))
	tm.ProcessOutput([]byte("cd"))
	var s Snapshot
	tm.Snapshot(&s)
	row := s.Cells[:4]
	if row[0].Ch != 'a' || row[1].Ch != 'b' || row[2].Ch != 'c' || row[3].Ch != 'd' {
		t.Fatalf("cells = %q%q%q%q, want abcd", row[0].Ch, row[1].Ch, row[2].Ch, row[3].Ch)
	}
	if len(s.Graphics) != 1 {
		t.Fatalf("placements = %+v, want one", s.Graphics)
	}
}

func TestGraphicsReplacementKeepsPlacement(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2;" + gfxPixels(2, 2, 4)))
	_, before := gfxSnapshot(t, tm)
	gen := before[0].ImageGeneration
	old := before[0].Raster
	tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=2,v=2;" + gfxPixels(2, 2, 4)))
	next, after := gfxSnapshot(t, tm)
	if len(after) != 1 {
		t.Fatalf("placements = %+v, want the placement kept", after)
	}
	if after[0].ImageGeneration <= gen {
		t.Errorf("image generation %d did not move past %d", after[0].ImageGeneration, gen)
	}
	if after[0].Raster == old {
		t.Error("replacement reused the old raster")
	}
	if next <= before[0].ImageGeneration {
		t.Error("graphics generation did not bump for an image-only change")
	}
	if before[0].Raster != old {
		t.Error("the old snapshot followed the replacement")
	}
}

func TestGraphicsDeleteBySpecifier(t *testing.T) {
	setup := func(t *testing.T) *Terminal {
		tm := gfxTerm(t, 10, 20)
		tm.SetCellPixels(5, 10)
		tm.ProcessOutput(gfxAPC("a=t,f=32,i=1,s=2,v=2;" + gfxPixels(2, 2, 4)))
		tm.ProcessOutput(gfxAPC("a=t,f=32,i=2,s=2,v=2;" + gfxPixels(2, 2, 4)))
		tm.ProcessOutput(gfxAPC("a=p,i=1,p=5,c=1,r=1"))
		tm.ProcessOutput(gfxAPC("a=p,i=1,c=1,r=1"))
		tm.ProcessOutput(gfxAPC("a=p,i=2,c=1,r=1"))
		return tm
	}
	cases := []struct {
		name string
		spec string
		want int
		kept uint32
	}{
		{"one placement", "a=d,d=i,i=1,p=5", 2, 1},
		{"all placements of an image", "a=d,d=i,i=1", 1, 2},
		{"all placements", "a=d,d=a", 0, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tm := setup(t)
			tm.ProcessOutput(gfxAPC(c.spec))
			_, got := gfxSnapshot(t, tm)
			if len(got) != c.want {
				t.Fatalf("placements = %d, want %d", len(got), c.want)
			}
			if tm.gfx.image(screenMain, c.kept) == nil {
				t.Errorf("image %d was dropped", c.kept)
			}
		})
	}
	t.Run("everything", func(t *testing.T) {
		tm := setup(t)
		tm.ProcessOutput(gfxAPC("a=d,d=A"))
		if _, got := gfxSnapshot(t, tm); len(got) != 0 {
			t.Fatalf("placements = %d, want none", len(got))
		}
		if tm.gfx.image(screenMain, 1) != nil || tm.gfx.image(screenMain, 2) != nil {
			t.Error("d=A kept image data")
		}
	})
	t.Run("by number", func(t *testing.T) {
		tm := gfxTerm(t, 10, 20)
		tm.SetCellPixels(5, 10)
		tm.ProcessOutput(gfxAPC("a=T,f=32,I=7,s=2,v=2;" + gfxPixels(2, 2, 4)))
		if _, got := gfxSnapshot(t, tm); len(got) != 1 {
			t.Fatalf("placements = %+v, want one", got)
		}
		tm.ProcessOutput(gfxAPC("a=d,d=n,I=7"))
		if _, got := gfxSnapshot(t, tm); len(got) != 0 {
			t.Fatalf("placements = %+v, want none after deleting the number", got)
		}
		if tm.gfx.imageByNumber(screenMain, 7) == nil {
			t.Error("d=n freed the image data")
		}
		tm.ProcessOutput(gfxAPC("a=d,d=N,I=7"))
		if tm.gfx.imageByNumber(screenMain, 7) != nil {
			t.Error("d=N kept the image data")
		}
	})
	t.Run("placement selectors are refused", func(t *testing.T) {
		for _, spec := range []string{"a=d,d=p,i=1,p=5", "a=d,d=P,i=1"} {
			tm := setup(t)
			br := pipeReplies(t, tm)
			tm.ProcessOutput(gfxAPC(spec))
			wantReplyCode(t, readAPCReply(t, br), "EINVAL")
			if _, got := gfxSnapshot(t, tm); len(got) != 3 {
				t.Fatalf("%s deleted %+v", spec, got)
			}
		}
	})
	t.Run("uppercase keeps data while placements remain", func(t *testing.T) {
		tm := setup(t)
		tm.ProcessOutput(gfxAPC("a=d,d=I,i=1,p=5"))
		if tm.gfx.image(screenMain, 1) == nil {
			t.Error("d=I freed data that another placement still uses")
		}
		tm.ProcessOutput(gfxAPC("a=d,d=I,i=1"))
		if tm.gfx.image(screenMain, 1) != nil {
			t.Error("d=I kept data with no placements left")
		}
	})
}

func TestGraphicsScrollsWithOutput(t *testing.T) {
	tm := gfxTerm(t, 5, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput([]byte("\x1b[4;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].CellRect.Min.Y != 3 {
		t.Fatalf("placement row = %+v, want 3", got)
	}
	tm.ProcessOutput([]byte("\n"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].CellRect.Min.Y != 3 {
		t.Fatalf("placement row after moving below it = %+v, want 3", got)
	}
	tm.ProcessOutput([]byte("\n"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].CellRect.Min.Y != 2 {
		t.Fatalf("placement row after a scroll = %+v, want 2", got)
	}
	tm.ProcessOutput([]byte("\n\n\n\n"))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("placement survived scrolling off screen: %+v", got)
	}
}

func TestGraphicsScrollRegionMovesOnlyItsRows(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput([]byte("\x1b[1;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	tm.ProcessOutput([]byte("\x1b[3;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	tm.ProcessOutput([]byte("\x1b[4;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	tm.ProcessOutput([]byte("\x1b[3;5r\x1b[5;1H\n"))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 2 {
		t.Fatalf("placements = %+v, want the row outside the region and the one still inside", got)
	}
	if got[0].CellRect.Min.Y != 0 || got[1].CellRect.Min.Y != 2 {
		t.Errorf("rows = %d,%d, want 0,2: only the region scrolls, and the row leaving it is evicted",
			got[0].CellRect.Min.Y, got[1].CellRect.Min.Y)
	}
}

func TestGraphicsAltScreenIsolation(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	tm.ProcessOutput([]byte("\x1b[?1049h\x1b[H"))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("alt screen shows main placements: %+v", got)
	}
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=4,v=4,c=2,r=2;" + gfxPixels(4, 4, 4)))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].CellRect != image.Rect(0, 0, 2, 2) {
		t.Fatalf("alt placement = %+v, want its own image 1", got)
	}
	if img := tm.gfx.image(screenAlt, 1); img == nil || img.raster.Bounds() != image.Rect(0, 0, 4, 4) {
		t.Fatalf("alt image 1 = %v, want the 4x4 one", img)
	}
	if img := tm.gfx.image(screenMain, 1); img == nil || img.raster.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("main image 1 = %v, want the 2x2 one", img)
	}
	tm.ProcessOutput(gfxAPC("a=d,d=I,i=1"))
	if tm.gfx.image(screenAlt, 1) != nil {
		t.Error("deleting on the alt screen kept its image")
	}
	if tm.gfx.image(screenMain, 1) == nil {
		t.Error("deleting on the alt screen destroyed the main screen's image")
	}
	tm.ProcessOutput([]byte("\x1b[?1049l"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].CellRect != image.Rect(0, 0, 1, 1) {
		t.Fatalf("main placement after leaving the alt screen = %+v", got)
	}
}

func TestGraphicsRISClearsEverything(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	gen, _ := gfxSnapshot(t, tm)
	tm.ProcessOutput([]byte("\x1bc"))
	next, got := gfxSnapshot(t, tm)
	if len(got) != 0 || next <= gen {
		t.Fatalf("RIS left %d placements, generation %d -> %d", len(got), gen, next)
	}
	if tm.gfx.image(screenMain, 1) != nil {
		t.Error("RIS kept image data")
	}
}

func TestGraphicsResizeKeepsAnchorsAndClips(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput([]byte("\x1b[8;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=2,r=2,C=1;" + gfxPixels(2, 2, 4)))
	tm.Resize(5, 10)
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements after shrinking = %+v, want one", got)
	}
	if got[0].CellRect.Min.Y != 4 {
		t.Errorf("anchor row = %d, want 4 after the screen moved up three rows", got[0].CellRect.Min.Y)
	}
	tm.Resize(1, 10)
	_, got = gfxSnapshot(t, tm)
	if len(got) != 1 || got[0].Clip != image.Rect(0, 0, 2, 1) {
		t.Fatalf("clip after shrinking to one row = %+v", got)
	}
	tm.ProcessOutput([]byte("\x1b[6G"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=2,s=2,v=2,c=1,r=1,C=1;" + gfxPixels(2, 2, 4)))
	tm.Resize(1, 2)
	_, got = gfxSnapshot(t, tm)
	if len(got) != 1 || got[0].ImageID != 1 {
		t.Fatalf("placements after shrinking the columns = %+v, want only image 1", got)
	}
}

// FuzzGraphicsAPC feeds graphics traffic one byte per write, so the parser's
// incremental states (partial APC, chunked payload, escape handling, a reset
// in the middle) are exercised in every split the fuzzer finds.
func FuzzGraphicsAPC(f *testing.F) {
	for _, seed := range []string{
		"",
		"\x1b_Ga=t,f=24,s=1,v=1,i=1;AAAA\x1b\\",
		"\x1b_Ga=T,f=32,s=1,v=1,i=1,p=1,c=1,r=1;AAAA\x1b\\",
		"\x1b_Ga=t,f=32,s=1,v=1,i=1,m=1;AAAA\x1b\\\x1b_Gm=0;AAAA\x1b\\",
		"\x1b_Ga=t,f=32,s=1,v=1,i=1,m=1;AAAA",
		"\x1b_Ga=q,i=1,s=1,v=1,f=24;AAAA\x1b\\",
		"\x1b_Ga=d,d=a\x1b\\",
		"\x1b_Ga=p,i=1,p=1\x1b\\",
		"\x1b_Ga=t,i=1;SECRET\x1bc",
		"\x1b_Ga=t,i=1;SECRET\a",
		"\x1b_Ga=t,i=1;SECRET\x1b[31m\x1b\\",
		"\x1b_other\x1b\\",
		"\x1b[?1049h\x1b_Ga=T,f=32,s=1,v=1,i=1;AAAA\x1b[?1049l",
		"\x1b[3;5r\x1b[5;1H\n\x1b[r",
		"text\x1b_G\x1b\\text",
		"\x1b_Ga=t,f=100,i=1;AAAA\x1b\\",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		tm := New(8, 16)
		tm.SetGraphicsEnabled(true)
		defer tm.Close()
		for i := range data {
			tm.ProcessOutput(data[i : i+1])
		}
		tm.Finish()
		var s Snapshot
		tm.Snapshot(&s)
		for _, p := range s.Graphics {
			if p.Raster == nil || p.Clip.Empty() || p.Raster.Bounds().Empty() {
				t.Fatalf("placement with no pixels: %+v", p)
			}
		}
		lim := defaultGraphicsLimits
		if tm.gfx.total > lim.totalBytes || len(tm.gfx.images) > lim.images ||
			len(tm.gfx.places) > lim.placements || len(tm.apc) > lim.payloadBytes {
			t.Fatalf("store over its limits: %d bytes, %d images, %d placements, %d APC bytes",
				tm.gfx.total, len(tm.gfx.images), len(tm.gfx.places), len(tm.apc))
		}
	})
}

// TestGraphicsScrollTallPlacementLeavesOnlyWhenFullyOff is the regression for
// a placement taller than one row: it must move with the region and only go
// away once every row it had inside the region has scrolled out.
func TestGraphicsScrollTallPlacementLeavesOnlyWhenFullyOff(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput([]byte("\x1b[1;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=4,v=12,c=1,r=3;" + gfxPixels(4, 12, 4)))
	tm.ProcessOutput([]byte("\x1b[10;1H\n"))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placement evicted while it was still on screen: %+v", got)
	}
	if got[0].CellRect.Min.Y != -1 || got[0].Clip != image.Rect(0, 0, 1, 2) {
		t.Fatalf("placement after one scroll = %+v, want the rows still on screen", got[0])
	}
	tm.ProcessOutput([]byte("\n\n"))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("a placement that scrolled fully off survived: %+v", got)
	}
}

// TestGraphicsScrollRegionNeverPinsATallPlacement: with a top margin the
// placement is clipped to the region, so it keeps only the rows still inside
// it and is dropped when none are left.
func TestGraphicsScrollRegionNeverPinsATallPlacement(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput([]byte("\x1b[1;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=4,v=20,c=1,r=5;" + gfxPixels(4, 20, 4)))
	tm.ProcessOutput([]byte("\x1b[3;10r\x1b[10;1H"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 {
		t.Fatalf("placements = %+v, want the tall one", got)
	}
	for i := 1; i <= 2; i++ {
		tm.ProcessOutput([]byte("\n"))
		_, got := gfxSnapshot(t, tm)
		if len(got) != 1 {
			t.Fatalf("scroll %d dropped the placement while rows were still inside the region: %+v", i, got)
		}
		if got[0].Clip.Min.Y != 2 {
			t.Fatalf("scroll %d painted above the top margin: %+v", i, got[0])
		}
	}
	tm.ProcessOutput([]byte("\n\n"))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("a placement that scrolled fully off survived: %+v", got)
	}
}

// TestGraphicsScrollRegionStopsAtTheMargin: a placement inside the region moves
// with it and is evicted when it would leave, so nothing is ever drawn in the
// rows above the top margin.
func TestGraphicsScrollRegionStopsAtTheMargin(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput([]byte("\x1b[4;1H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	tm.ProcessOutput([]byte("\x1b[3;10r\x1b[10;1H\n"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 || got[0].CellRect.Min.Y != 2 {
		t.Fatalf("placement after one scroll = %+v, want row 2", got)
	}
	tm.ProcessOutput([]byte("\n"))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("placement painted above the margin: %+v", got)
	}
}

func TestGraphicsResizeInvalidatesAndRedraws(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.SetCellPixels(5, 10)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=1,r=1;" + gfxPixels(2, 2, 4)))
	gen, _ := gfxSnapshot(t, tm)
	epoch := tm.gfx.epoch
	tm.Resize(9, 19)
	if tm.gfx.epoch == epoch {
		t.Error("resize left a decode from the old grid valid")
	}
	gen2, got := gfxSnapshot(t, tm)
	if gen2 == gen {
		t.Error("resize did not bump the generation")
	}
	if len(got) != 1 || got[0].CellRect != image.Rect(0, 0, 1, 1) {
		t.Fatalf("placement after resize = %+v", got)
	}
	tm.Resize(9, 19)
	if gen3, _ := gfxSnapshot(t, tm); gen3 != gen2 {
		t.Error("an unchanged resize bumped the generation")
	}
}

func TestGraphicsCellPixelsInvalidateAndRedraw(t *testing.T) {
	tm := gfxTerm(t, 10, 20)
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=20,v=40;" + gfxPixels(20, 40, 4)))
	gen, got := gfxSnapshot(t, tm)
	if len(got) != 1 || got[0].CellRect != image.Rect(0, 0, 2, 2) {
		t.Fatalf("placement with fallback metrics = %+v, want 2x2 cells", got)
	}
	epoch := tm.gfx.epoch
	tm.SetCellPixels(5, 10)
	if tm.gfx.epoch == epoch {
		t.Error("a cell size change left a decode with the old metrics valid")
	}
	if gen2, _ := gfxSnapshot(t, tm); gen2 == gen {
		t.Error("a cell size change did not bump the generation")
	}
	gen2, _ := gfxSnapshot(t, tm)
	tm.SetCellPixels(5, 10)
	if gen3, _ := gfxSnapshot(t, tm); gen3 != gen2 {
		t.Error("an unchanged cell size bumped the generation")
	}
}

func TestGraphicsFailedCommandLeavesCursorAlone(t *testing.T) {
	tm := gfxTerm(t, 6, 20)
	tm.SetCellPixels(5, 10)
	br := pipeReplies(t, tm)
	tm.ProcessOutput([]byte("\x1b[2;3H"))
	tm.ProcessOutput(gfxAPC("a=T,f=32,i=1,s=2,v=2,c=3,r=2;!!!!"))
	wantReplyCode(t, readAPCReply(t, br), "EINVAL")
	if tm.curRow != 1 || tm.curCol != 2 {
		t.Fatalf("cursor = %d,%d, want 1,2: a failed transmit moved it", tm.curRow, tm.curCol)
	}
	tm.ProcessOutput(gfxAPC("a=p,i=99,c=3,r=2"))
	wantReplyCode(t, readAPCReply(t, br), "ENOENT")
	if tm.curRow != 1 || tm.curCol != 2 {
		t.Fatalf("cursor = %d,%d, want 1,2: a failed placement moved it", tm.curRow, tm.curCol)
	}
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("failed commands stored %+v", got)
	}
}
