package term

// Independent regression tests for the bounded Kitty graphics store (PR 5a).
// They drive the emulator only through raw program output (never through
// kittygfx.Encode) and through the public Terminal API, so they keep checking
// the wire behaviour a real PTY client sees. Written from the security review:
// payloads are attacker controlled, replies are the only feedback a client
// gets, and image state must never leak across screens, resets or reads.

import (
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/kittygfx"
)

// revAPC wraps a graphics body in ESC _ G ... ESC \.
func revAPC(body string) string { return "\x1b_G" + body + "\x1b\\" }

// revPixels is base64 for a w x h image whose first pixel is c and whose
// remaining pixels are a different colour, so a replacement is visible.
func revPixels(w, h, bpp int, first, rest color.NRGBA) string {
	raw := make([]byte, 0, w*h*bpp)
	for i := 0; i < w*h; i++ {
		c := rest
		if i == 0 {
			c = first
		}
		if bpp == 3 {
			raw = append(raw, c.R, c.G, c.B)
		} else {
			raw = append(raw, c.R, c.G, c.B, c.A)
		}
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func revTerm(t *testing.T, rows, cols int) *Terminal {
	t.Helper()
	tm := New(rows, cols)
	tm.SetGraphicsEnabled(true)
	return tm
}

// revReplyPipe gives the terminal a PTY whose replies the test can read.
func revReplyPipe(t *testing.T, tm *Terminal) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	tm.ptm = w
	return r
}

// revNextReply returns the reply that arrives within a short deadline, or ""
// when the terminal stayed silent. Replies are written with one write each, so
// one read returns one reply.
func revNextReply(t *testing.T, r *os.File) string {
	t.Helper()
	if err := r.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 512)
	n, err := r.Read(buf)
	if n == 0 {
		if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("reading a reply: %v", err)
		}
		return ""
	}
	return string(buf[:n])
}

// revMustReply asserts that the next reply contains every want substring.
func revMustReply(t *testing.T, r *os.File, want ...string) string {
	t.Helper()
	got := revNextReply(t, r)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("reply %q does not contain %q", got, w)
		}
	}
	return got
}

// revQuiet asserts that nothing was replied before the next command, by
// sending a command that must answer and checking that it answers at once.
func revQuiet(t *testing.T, tm *Terminal, r *os.File) {
	t.Helper()
	tm.ProcessOutput([]byte(revAPC("a=q,i=99,s=1,v=1,f=24,t=d;AAAA")))
	revMustReply(t, r, "OK")
}

// revState renders the semantic screen state a split must not change: the
// visible text, the cursor and every placement (image, cell rect, clip, source
// and the first pixel). Generation counters are deliberately excluded: they
// count changes, not state.
func revState(t *testing.T, tm *Terminal) string {
	t.Helper()
	var s Snapshot
	tm.Snapshot(&s)
	var sb strings.Builder
	fmt.Fprintf(&sb, "screen %dx%d cursor %d,%d text %q\n", s.Rows, s.Cols, s.CursorRow, s.CursorCol,
		strings.Join(tm.PlainScreen(), "|"))
	places := make([]string, 0, len(s.Graphics))
	for _, p := range s.Graphics {
		c := p.Raster.At(0, 0)
		places = append(places, fmt.Sprintf("img=%d pid=%d gen=%d cell=%v clip=%v src=%v off=%v px=%v",
			p.ImageID, p.PlacementID, p.ImageGeneration, p.CellRect, p.Clip, p.Source, p.PixelOffset, c))
	}
	sort.Strings(places)
	sb.WriteString(strings.Join(places, "\n"))
	return sb.String()
}

// TestGraphicsReviewSplitIndependence feeds one program output sequence whole
// and at every possible split point, including one byte at a time. Graphics
// jobs are applied after the text of the read that carried them, so the order
// of placements relative to scrolling and text must not depend on how the
// kernel happened to split the writes.
func TestGraphicsReviewSplitIndependence(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	seq := "start\r\n" +
		revAPC("a=T,f=32,s=2,v=2,i=1,C=1;"+revPixels(2, 2, 4, red, color.NRGBA{})) +
		"middle\r\n" +
		strings.Repeat("filler\r\n", 6) + // scrolls the image off the screen
		revAPC("a=T,f=32,s=1,v=1,i=2,C=1;"+revPixels(1, 1, 4, red, red)) +
		revAPC("a=p,i=2,p=5,c=1,r=1") +
		"tail\x1b_ignored apc\x1b\\" +
		revAPC("a=t,f=32,s=1,v=1,i=3,m=1;AAAA") +
		revAPC("m=0;AA==") +
		revAPC("a=T,f=24,s=1,v=1,i=4,C=1;AAAA") +
		revAPC("a=d,d=a")
	whole := revTerm(t, 8, 20)
	whole.ProcessOutput([]byte(seq))
	whole.Finish()
	want := revState(t, whole)

	for cut := 0; cut <= len(seq); cut++ {
		tm := revTerm(t, 8, 20)
		tm.ProcessOutput([]byte(seq[:cut]))
		tm.ProcessOutput([]byte(seq[cut:]))
		tm.Finish()
		if got := revState(t, tm); got != want {
			t.Fatalf("split at %d changed the final state:\n got %s\nwant %s", cut, got, want)
		}
	}
	tm := revTerm(t, 8, 20)
	for i := 0; i < len(seq); i++ {
		tm.ProcessOutput([]byte(seq[i : i+1]))
	}
	tm.Finish()
	if got := revState(t, tm); got != want {
		t.Fatalf("byte-at-a-time feed changed the final state:\n got %s\nwant %s", got, want)
	}
}

// TestGraphicsReviewPlacementFollowsSameReadScroll pins the ordering that the
// split test only implies: an image placed on the last row and then scrolled
// away in the same read must be gone, exactly as it is when the scroll arrives
// in a later read.
func TestGraphicsReviewPlacementFollowsSameReadScroll(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	body := revAPC("a=T,f=32,s=1,v=1,i=1,C=1;" + revPixels(1, 1, 4, red, red))

	tm := revTerm(t, 6, 20)
	tm.ProcessOutput([]byte("\x1b[6;1H")) // last row
	tm.ProcessOutput([]byte(body + strings.Repeat("\r\n", 6)))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("placement survived a scroll in the same read: %+v", got)
	}

	// The same two writes in the other order: the image is placed after the
	// scroll, so it is still on the last row and must be visible.
	tm = revTerm(t, 6, 20)
	tm.ProcessOutput([]byte("\x1b[6;1H" + strings.Repeat("\r\n", 3)))
	tm.ProcessOutput([]byte(body))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placement after the scroll = %+v, want one", got)
	}
	if got[0].CellRect.Min.Y != 5 {
		t.Errorf("placement row = %d, want the last row 5", got[0].CellRect.Min.Y)
	}
}

// TestGraphicsReviewCanonicalQuery answers the capability probe every Kitty
// client sends first. It names an image id that does not exist, and the reply
// must still be OK: a client that gets ENOENT concludes the terminal cannot
// display images at all.
func TestGraphicsReviewCanonicalQuery(t *testing.T) {
	tm := revTerm(t, 6, 20)
	br := revReplyPipe(t, tm)
	tm.ProcessOutput([]byte(revAPC("a=q,t=d,f=24,s=1,v=1,i=31;AAAA")))
	revMustReply(t, br, "i=31", "OK")
	tm.ProcessOutput([]byte(revAPC("a=q,i=31,s=1,v=1,f=24;AA*A")))
	if got := revNextReply(t, br); strings.Contains(got, "OK") {
		t.Errorf("query with a malformed payload replied %q, want an error", got)
	}
}

// TestGraphicsReviewQuerySilentWhenDisabled: a terminal that cannot draw images
// must stay silent, which is what a Kitty-unaware terminal does. A positive
// answer here would make a viewer upload pixels into a void.
func TestGraphicsReviewQuerySilentWhenDisabled(t *testing.T) {
	tm := New(6, 20) // graphics off by default
	br := revReplyPipe(t, tm)
	tm.ProcessOutput([]byte(revAPC("a=q,t=d,f=24,s=1,v=1,i=31;AAAA")))
	tm.ProcessOutput([]byte(revAPC("a=T,f=24,s=1,v=1,i=1;AAAA")))
	// Nothing may have been answered before this cursor report.
	tm.ProcessOutput([]byte("\x1b[6n"))
	revMustReply(t, br, "\x1b[1;1R")
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("disabled terminal stored placements: %+v", got)
	}
	if screen := strings.Join(tm.PlainScreen(), ""); strings.TrimSpace(screen) != "" {
		t.Fatalf("graphics output reached the screen while disabled: %q", screen)
	}
}

// TestGraphicsReviewPayloadNeverReachesScreen drives every way an APC can fail
// and checks that no payload byte is printed, that the parser resynchronises,
// and that an escape inside an APC is not executed as a command.
func TestGraphicsReviewPayloadNeverReachesScreen(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"not graphics", "\x1b_SECRET_APC\x1b\\"},
		{"bad base64", revAPC("a=t,f=24,s=1,v=1,i=1;SECRET@@")},
		{"unsupported action", revAPC("a=f,i=1;SECRET")},
		{"oversized payload", revAPC("a=t,f=24,s=1,v=1,i=1;" + strings.Repeat("A", 2*defaultGraphicsLimits.payloadBytes))},
		{"esc inside", revAPC("a=t,f=24,s=1,v=1,i=1;SEC") + "RET\x1b[31m\x1b\\"},
		{"unterminated then reset", "\x1b_Ga=t,f=24,s=1,v=1,i=1;SECRET\x1bc"},
		{"bel terminated", "\x1b_Ga=t,f=24,s=1,v=1,i=1;SECRET\a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := revTerm(t, 4, 40)
			tm.ProcessOutput([]byte("A" + tt.in + "B"))
			tm.Finish()
			screen := strings.Join(tm.PlainScreen(), "")
			if strings.Contains(screen, "SECRET") || strings.Contains(screen, "AAAA") {
				t.Fatalf("payload reached the screen: %q", screen)
			}
			if !strings.HasSuffix(strings.TrimSpace(screen), "B") {
				t.Fatalf("parser did not resynchronise after the APC: %q", screen)
			}
			if strings.Contains(screen, "[31m") {
				t.Fatalf("an escape inside an APC was executed: %q", screen)
			}
		})
	}
}

// TestGraphicsReviewQuietLevels checks the reply rules: q=1 suppresses success
// but not errors, q=2 suppresses everything, and a suppressed reply must not
// leave a stray reply behind for the next command.
func TestGraphicsReviewQuietLevels(t *testing.T) {
	tm := revTerm(t, 6, 20)
	br := revReplyPipe(t, tm)
	good := "a=T,f=24,s=1,v=1,i=1,q=1;AAAA"

	tm.ProcessOutput([]byte(revAPC(good)))
	revQuiet(t, tm, br) // the successful transmit above must not have replied

	tm.ProcessOutput([]byte(revAPC("a=f,i=2,q=1;AAAA")))
	revMustReply(t, br, "i=2") // errors are not suppressed by q=1

	tm.ProcessOutput([]byte(revAPC("a=f,i=3,q=2;AAAA")))
	tm.ProcessOutput([]byte(revAPC("a=t,f=24,s=1,v=1,i=4,q=2;AAAA")))
	revQuiet(t, tm, br)

	tm.ProcessOutput([]byte(revAPC("a=T,f=24,s=1,v=1,i=5;AAAA")))
	revMustReply(t, br, "i=5", "OK")
}

// TestGraphicsReviewSingleChunkTransmit is the plainest possible success path:
// one APC that transmits and places a 2x2 image. It catches a duplicated or
// dropped payload, which shows up as an error reply instead of pixels.
func TestGraphicsReviewSingleChunkTransmit(t *testing.T) {
	tm := revTerm(t, 6, 20)
	br := revReplyPipe(t, tm)
	red := color.NRGBA{255, 0, 0, 255}
	tm.SetCellPixels(1, 1)
	tm.ProcessOutput([]byte(revAPC("a=T,f=32,s=2,v=2,i=1,C=1;" + revPixels(2, 2, 4, red, red))))
	revMustReply(t, br, "i=1", "OK")
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	if got[0].Raster.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("raster bounds %v, want 2x2", got[0].Raster.Bounds())
	}
	if c, ok := got[0].Raster.At(1, 1).(color.NRGBA); !ok || c != red {
		t.Errorf("pixel = %v, want %v", got[0].Raster.At(1, 1), red)
	}
}

// TestGraphicsReviewChunkedTransmit checks that a transfer split over several
// APCs, including across reads, produces exactly the pixels a single APC would.
func TestGraphicsReviewChunkedTransmit(t *testing.T) {
	blue := color.NRGBA{0, 0, 255, 255}
	payload := revPixels(1, 1, 4, blue, blue) // "AAAA..." style, 8 chars
	first, second := payload[:4], payload[4:]

	tm := revTerm(t, 6, 20)
	br := revReplyPipe(t, tm)
	tm.ProcessOutput([]byte(revAPC("a=T,f=32,s=1,v=1,i=7,m=1,C=1;" + first)))
	// A continuation chunk carries no identity keys; the terminal has to
	// remember the transfer it belongs to.
	tm.ProcessOutput([]byte(revAPC("m=0;" + second)))
	revMustReply(t, br, "i=7", "OK")
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v, want one", got)
	}
	if c, ok := got[0].Raster.At(0, 0).(color.NRGBA); !ok || c != blue {
		t.Errorf("pixel = %v, want %v", got[0].Raster.At(0, 0), blue)
	}
}

// TestGraphicsReviewResetAbortsTransfer: RIS must drop an incomplete transfer so
// the rest of it cannot resurrect an image after the reset.
func TestGraphicsReviewResetAbortsTransfer(t *testing.T) {
	payload := revPixels(1, 1, 4, color.NRGBA{1, 2, 3, 4}, color.NRGBA{1, 2, 3, 4})
	tm := revTerm(t, 6, 20)
	br := revReplyPipe(t, tm)
	tm.ProcessOutput([]byte(revAPC("a=T,f=32,s=1,v=1,i=1,m=1,C=1;" + payload[:4])))
	tm.ProcessOutput([]byte("\x1bc"))
	if tm.gfx.transfer != nil {
		t.Error("reset kept the transfer in flight")
	}
	tm.ProcessOutput([]byte(revAPC("m=0;" + payload[4:])))
	// The orphaned continuation may be answered with an error; either way it
	// must not have created the image or a placement.
	_ = revNextReply(t, br)
	if tm.gfx.image(screenMain, 1) != nil {
		t.Error("a transfer that a reset cancelled created an image")
	}
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("a reset transfer was resurrected: %+v", got)
	}
}

// TestGraphicsReviewReplacementIsAtomic: replacing an image with the same id
// keeps its placements, bumps the image generation, and leaves an older
// snapshot pointing at the old pixels.
func TestGraphicsReviewReplacementIsAtomic(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	green := color.NRGBA{0, 255, 0, 255}
	tm := revTerm(t, 6, 20)
	tm.ProcessOutput([]byte(revAPC("a=t,f=32,s=1,v=1,i=1;" + revPixels(1, 1, 4, red, red))))
	tm.ProcessOutput([]byte(revAPC("a=p,i=1,p=3,c=1,r=1")))
	var before Snapshot
	tm.Snapshot(&before)
	if len(before.Graphics) != 1 {
		t.Fatalf("placements before = %+v", before.Graphics)
	}

	tm.ProcessOutput([]byte(revAPC("a=t,f=32,s=1,v=1,i=1;" + revPixels(1, 1, 4, green, green))))
	var after Snapshot
	tm.Snapshot(&after)
	if len(after.Graphics) != 1 {
		t.Fatalf("replacement dropped the placement: %+v", after.Graphics)
	}
	if after.GraphicsGeneration <= before.GraphicsGeneration {
		t.Errorf("generation did not move on an image-only change: %d then %d",
			before.GraphicsGeneration, after.GraphicsGeneration)
	}
	if got := after.Graphics[0].Raster.At(0, 0); got != green {
		t.Errorf("new snapshot pixel = %v, want %v", got, green)
	}
	if got := before.Graphics[0].Raster.At(0, 0); got != red {
		t.Errorf("old snapshot followed the replacement: %v, want %v", got, red)
	}
}

// TestGraphicsReviewScreensAreIsolated: placements live on the screen that
// showed them, and deleting on the alternate screen must not touch the main
// screen's images.
func TestGraphicsReviewScreensAreIsolated(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	img := revPixels(1, 1, 4, red, red)

	tm := revTerm(t, 6, 20)
	tm.ProcessOutput([]byte(revAPC("a=T,f=32,s=1,v=1,i=1,C=1;" + img)))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 {
		t.Fatalf("main placements = %+v, want one", got)
	}
	tm.ProcessOutput([]byte("\x1b[?1049h"))
	if _, got := gfxSnapshot(t, tm); len(got) != 0 {
		t.Fatalf("main placement leaked onto the alternate screen: %+v", got)
	}
	tm.ProcessOutput([]byte(revAPC("a=T,f=32,s=1,v=1,i=1,C=1;" + img)))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 {
		t.Fatalf("alternate placements = %+v, want one", got)
	}
	// d=A and d=i must not reach the other screen.
	tm.ProcessOutput([]byte(revAPC("a=d,d=A")))
	tm.ProcessOutput([]byte("\x1b[?1049l"))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("main placements after deleting everything on the alternate screen = %+v, want one", got)
	}
	tm.ProcessOutput([]byte("\x1b[?1049h"))
	tm.ProcessOutput([]byte(revAPC("a=d,d=i,i=1")))
	tm.ProcessOutput([]byte("\x1b[?1049l"))
	if _, got := gfxSnapshot(t, tm); len(got) != 1 {
		t.Fatalf("main placements after deleting image 1 on the alternate screen = %+v, want one", got)
	}
}

// TestGraphicsReviewByteBudgetAcrossScreens: the byte cap is per terminal, not
// per screen, so filling the alternate screen must evict the oldest images even
// if they belong to the main screen.
func TestGraphicsReviewByteBudgetAcrossScreens(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	img := revPixels(2, 2, 4, red, red) // 16 bytes
	tm := revTerm(t, 6, 20)
	tm.SetGraphicsLimits(kittygfx.Limits{
		MaxWidth: 64, MaxHeight: 64, MaxImageBytes: 16, MaxTerminalBytes: 40,
		MaxImages: 8, MaxPlacements: 8, MaxInFlightBytes: 1 << 20, MaxPayload: 4096,
	})
	tm.ProcessOutput([]byte(revAPC("a=t,f=32,s=2,v=2,i=1;" + img)))
	tm.ProcessOutput([]byte("\x1b[?1049h"))
	tm.ProcessOutput([]byte(revAPC("a=t,f=32,s=2,v=2,i=2;" + img)))
	tm.ProcessOutput([]byte(revAPC("a=t,f=32,s=2,v=2,i=3;" + img)))
	if tm.gfx.total > 40 {
		t.Fatalf("store holds %d bytes over a 40 byte budget", tm.gfx.total)
	}
	if tm.gfx.image(screenMain, 1) != nil {
		t.Error("the oldest image on the main screen survived the byte budget")
	}
}

// TestGraphicsReviewImageThatDoesNotFitIsNotSilentlyDropped: an image larger
// than the whole terminal budget must be refused, not stored and then evicted
// by its own insertion while the client is told OK.
func TestGraphicsReviewImageThatDoesNotFitIsNotSilentlyDropped(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	img := revPixels(2, 2, 4, red, red) // 16 bytes, budget 8
	tm := revTerm(t, 6, 20)
	br := revReplyPipe(t, tm)
	tm.SetGraphicsLimits(kittygfx.Limits{
		MaxWidth: 64, MaxHeight: 64, MaxImageBytes: 64, MaxTerminalBytes: 8,
		MaxImages: 8, MaxPlacements: 8, MaxInFlightBytes: 1 << 20, MaxPayload: 4096,
	})
	tm.ProcessOutput([]byte(revAPC("a=t,f=32,s=2,v=2,i=1;" + img)))
	reply := revNextReply(t, br)
	stored := tm.gfx.image(screenMain, 1) != nil
	if stored && !strings.Contains(reply, "OK") {
		t.Fatalf("stored the image but replied %q", reply)
	}
	if !stored && strings.Contains(reply, "OK") {
		t.Fatalf("replied %q but dropped the image it acknowledged", reply)
	}
}

// TestGraphicsReviewLimitsEvictOldest checks the documented eviction order and
// that eviction removes the placements of the image it drops.
func TestGraphicsReviewLimitsEvictOldest(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	img := revPixels(1, 1, 4, red, red)
	tm := revTerm(t, 6, 20)
	tm.SetGraphicsLimits(kittygfx.Limits{
		MaxWidth: 64, MaxHeight: 64, MaxImageBytes: 64, MaxTerminalBytes: 1 << 20,
		MaxImages: 2, MaxPlacements: 2, MaxInFlightBytes: 1 << 20, MaxPayload: 4096,
	})
	for id := 1; id <= 3; id++ {
		tm.ProcessOutput([]byte(revAPC(fmt.Sprintf("a=T,f=32,s=1,v=1,i=%d,C=1;%s", id, img))))
	}
	if tm.gfx.image(screenMain, 1) != nil || tm.gfx.image(screenMain, 3) == nil {
		t.Fatalf("images after eviction: 1=%v 3=%v, want 1 gone and 3 present",
			tm.gfx.image(screenMain, 1), tm.gfx.image(screenMain, 3))
	}
	_, got := gfxSnapshot(t, tm)
	if len(got) != 2 {
		t.Fatalf("placements after eviction = %+v, want the two newest", got)
	}
	for _, p := range got {
		if p.ImageID == 1 {
			t.Errorf("placement of the evicted image survived: %+v", p)
		}
	}
}

// TestGraphicsReviewShrinkResizeMovesPlacements: a row shrink pushes the top
// rows into scrollback, so placements anchored on them move up with the text
// and ones that end up above the screen are dropped.
func TestGraphicsReviewShrinkResizeMovesPlacements(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	img := revPixels(1, 1, 4, red, red)
	tm := revTerm(t, 10, 20)
	tm.ProcessOutput([]byte("\x1b[7;1H" + revAPC("a=T,f=32,s=1,v=1,i=1,C=1;"+img)))
	tm.ProcessOutput([]byte("\x1b[3;1H" + revAPC("a=T,f=32,s=1,v=1,i=2,C=1;"+img)))
	tm.ProcessOutput([]byte("\x1b[10;1H")) // cursor on the last row: shrinking moves 6 rows up
	tm.Resize(4, 20)
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements after the shrink = %+v, want only the one that stayed", got)
	}
	if got[0].ImageID != 1 || got[0].CellRect.Min.Y != 0 {
		t.Errorf("placement after the shrink = %+v, want image 1 at row 0", got[0])
	}
}

// TestGraphicsReviewCursorAdvanceIsOptOut: C=1 keeps the cursor exactly where it
// was, and without it the following text does not land inside the image.
func TestGraphicsReviewCursorAdvanceIsOptOut(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	img := revPixels(2, 2, 4, red, red)
	tm := revTerm(t, 6, 20)
	tm.SetCellPixels(1, 1)
	tm.ProcessOutput([]byte("\x1b[2;3H" + revAPC("a=T,f=32,s=2,v=2,i=1,C=1;"+img)))
	if row, col := tm.CursorRow(), tm.CursorCol(); row != 1 || col != 2 {
		t.Errorf("cursor with C=1 = %d,%d, want 1,2", row, col)
	}

	tm = revTerm(t, 6, 20)
	tm.SetCellPixels(1, 1)
	tm.ProcessOutput([]byte("\x1b[2;3H" + revAPC("a=T,f=32,s=2,v=2,i=1;"+img) + "X"))
	_, got := gfxSnapshot(t, tm)
	if len(got) != 1 {
		t.Fatalf("placements = %+v", got)
	}
	row, col := tm.CursorRow(), tm.CursorCol()
	if row >= 0 && row < got[0].CellRect.Max.Y && col-1 >= got[0].CellRect.Min.X && col-1 < got[0].CellRect.Max.X {
		t.Errorf("text after the image landed inside it: cursor %d,%d image %v", row, col, got[0].CellRect)
	}
}

// TestGraphicsReviewSnapshotDuringTransfers is the race test: snapshots (and
// the rasters they share) must stay consistent while a program keeps replacing
// images and placements.
func TestGraphicsReviewSnapshotDuringTransfers(t *testing.T) {
	tm := revTerm(t, 10, 20)
	red := color.NRGBA{255, 0, 0, 255}
	img := revPixels(2, 2, 4, red, red)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				id := (i % 4) + 1
				tm.ProcessOutput([]byte(revAPC(fmt.Sprintf("a=T,f=32,s=2,v=2,i=%d;"+img, id))))
				tm.ProcessOutput([]byte(revAPC(fmt.Sprintf("a=d,d=i,i=%d", id))))
			}
		}(w)
	}
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var s Snapshot
			for {
				select {
				case <-stop:
					return
				default:
				}
				tm.Snapshot(&s)
				for _, p := range s.Graphics {
					_ = p.Raster.At(0, 0)
					_ = p.Raster.Bounds()
				}
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
	tm.Finish()
}

// FuzzGraphicsReviewOutput feeds arbitrary program output to a terminal with
// graphics enabled. FuzzProcessOutput builds its terminals with graphics off,
// so this is the only fuzz coverage of the APC parser and the store: no input
// may panic, leak payload text, or leave the store or the parser over its
// bounds.
func FuzzGraphicsReviewOutput(f *testing.F) {
	for _, seed := range []string{
		revAPC("a=T,f=32,s=1,v=1,i=1;AAAAAA=="),
		revAPC("a=T,f=24,s=2,v=2,i=1,p=2,c=2,r=2;AAAAAAAAAAAAAAAA"),
		revAPC("a=t,f=32,s=1,v=1,i=1,m=1;AAAA") + revAPC("m=0;AA=="),
		revAPC("a=p,i=1,p=1,c=2,r=2"),
		revAPC("a=d,d=a"),
		revAPC("a=d,d=A"),
		revAPC("a=q,i=31,s=1,v=1,f=24,t=d;AAAA"),
		"\x1b_G" + "a=t,f=24,s=1,v=1,i=1;" + strings.Repeat("A", 2*defaultGraphicsLimits.payloadBytes),
		"\x1b_Ga=t,i=1;SECRET\x1b[31m\x1b\\",
		"\x1b_Ga=t,i=1;SECRET\x1bc",
		"\x1b_other\x1b\\",
		"\x1b[?1049h\x1b_Ga=T,f=32,s=1,v=1,i=1;AAAA\x1b[?1049l",
		"\x1b[3;5r\x1b[5;1H\n\x1b[r",
		"text\r\n" + revAPC("a=T,f=32,s=1,v=1,i=1;AAAA") + "text\r\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		tm := New(10, 20)
		tm.SetGraphicsEnabled(true)
		defer tm.Close()
		tm.ProcessOutput(data)
		tm.Finish()
		var s Snapshot
		tm.Snapshot(&s)
		for _, p := range s.Graphics {
			_ = p.Raster.At(0, 0)
			_ = p.Raster.Bounds()
		}
		lim := defaultGraphicsLimits
		if tm.gfx.total > lim.totalBytes {
			t.Fatalf("store holds %d bytes over %d", tm.gfx.total, lim.totalBytes)
		}
		if len(tm.gfx.images) > lim.images {
			t.Fatalf("store holds %d images over %d", len(tm.gfx.images), lim.images)
		}
		if len(tm.gfx.places) > lim.placements {
			t.Fatalf("store holds %d placements over %d", len(tm.gfx.places), lim.placements)
		}
		if len(tm.apc) > lim.payloadBytes {
			t.Fatalf("APC buffer holds %d bytes over %d", len(tm.apc), lim.payloadBytes)
		}
		if tm.gfx.transfer != nil && int64(len(tm.gfx.transfer.data)) > lim.inFlightBytes {
			t.Fatalf("transfer holds %d bytes over %d", len(tm.gfx.transfer.data), lim.inFlightBytes)
		}
		if n := len(tm.gfxJobs); n > 1024 {
			t.Fatalf("%d graphics jobs queued", n)
		}
	})
}

// TestGraphicsReviewImageLargerThanTheWholeBudget pins what happens when a
// configured per-terminal total is smaller than the per-image cap: the store
// cannot hold the image, so the client must be told, not acked as if stored.
func TestGraphicsReviewImageLargerThanTheWholeBudget(t *testing.T) {
	tm := New(2, 1)
	tm.SetGraphicsLimits(kittygfx.Limits{
		MaxWidth: 64, MaxHeight: 64,
		MaxImageBytes: 1 << 20, MaxTerminalBytes: 16,
		MaxImages: 4, MaxPlacements: 4,
		MaxInFlightBytes: 1 << 20, MaxPayload: 4096,
	})
	tm.SetGraphicsEnabled(true)
	r, w, _ := os.Pipe()
	tm.ptm = w
	defer r.Close()

	// 4x4 RGB = 48 bytes: under the per-image cap, over the whole budget.
	raw := strings.Repeat("\x00\x00\x00", 16)
	tm.ProcessOutput([]byte(revAPC("a=T,f=24,s=4,v=4,i=7;" + base64.StdEncoding.EncodeToString([]byte(raw)))))

	stored := tm.gfx.image(screenMain, 7) != nil
	reply := revNextReply(t, r)
	switch {
	case stored && strings.Contains(reply, "OK"):
	case !stored && revIsError(reply):
	default:
		t.Fatalf("image of 48 bytes against a 16-byte budget: stored=%v reply=%q — "+
			"the client was acked for an image the store then evicted", stored, reply)
	}
}

// revIsError reports whether a reply is a kitty error rather than OK: the code
// starts with E (EINVAL, ENOSPC, EBADF, ...).
func revIsError(reply string) bool {
	body, ok := strings.CutPrefix(reply, "\x1b_G")
	if !ok {
		return false
	}
	if _, rest, ok := strings.Cut(body, ";"); ok {
		body = rest
	}
	code, _, _ := strings.Cut(body, ":")
	return strings.HasPrefix(code, "E")
}
