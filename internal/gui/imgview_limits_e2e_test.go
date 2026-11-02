package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/term"
)

// Limits are configured exactly where both the GUI terminal and child runner
// load graphics.yaml, rather than injecting a limit only into either side.
func TestImgviewGUILocalGraphicsLimits(t *testing.T) {
	tm := imgviewTerminal(t, true, "kitty:\n  max_width: 32\n  max_height: 16\n  max_payload: 256\n  max_in_flight_mib: 1\n")
	limits := tm.GraphicsLimits()
	if limits.MaxWidth != 32 || limits.MaxHeight != 16 || limits.MaxPayload != 256 || limits.MaxInFlightBytes != 1<<20 {
		t.Fatalf("limits not loaded: %+v", limits)
	}
	s := imgviewWait(t, tm, "real pixels under reduced GUI limits", func(s *term.Snapshot) bool { return tm.InAltScreen() && len(s.Graphics) == 1 })
	bounds := s.Graphics[0].Raster.Bounds()
	if bounds.Dx() > 32 || bounds.Dy() > 16 {
		t.Fatalf("raster %v exceeds configured 32x16", bounds)
	}
	imgviewPixels(t, &s, rgb{255, 0, 0}, rgb{0, 0, 255}, false)
	tm.SendBytes([]byte("q"))
	select {
	case <-tm.Exited():
	case <-time.After(8 * time.Second):
		t.Fatal("q did not exit")
	}
	tm.Snapshot(&s)
	if tm.ExitCode() != 0 || tm.InAltScreen() || len(s.Graphics) != 0 {
		t.Fatalf("exit=%d alt=%v placements=%d", tm.ExitCode(), tm.InAltScreen(), len(s.Graphics))
	}
}

func TestImgviewGUITinyPayloadFallsBackWithNotice(t *testing.T) {
	tm := imgviewTerminal(t, true, "kitty:\n  max_payload: 1\n")
	if got := tm.GraphicsLimits().MaxPayload; got != 1 {
		t.Fatalf("max_payload = %d, want 1", got)
	}
	s := imgviewWait(t, tm, "halfblock fallback and visible limit notice", func(s *term.Snapshot) bool {
		var text strings.Builder
		halfblock := false
		for _, c := range s.Cells {
			text.WriteRune(c.Ch)
			if c.Ch == '▀' || c.Ch == '▄' {
				halfblock = true
			}
		}
		return halfblock && strings.Contains(strings.ToLower(text.String()), "limit")
	})
	if len(s.Graphics) != 0 {
		t.Fatalf("fallback has %d placements", len(s.Graphics))
	}
	tm.SendBytes([]byte("q"))
	select {
	case <-tm.Exited():
	case <-time.After(8 * time.Second):
		t.Fatal("q did not exit")
	}
	tm.Snapshot(&s)
	if tm.ExitCode() != 0 || tm.InAltScreen() || len(s.Graphics) != 0 {
		t.Fatalf("exit=%d alt=%v placements=%d", tm.ExitCode(), tm.InAltScreen(), len(s.Graphics))
	}
}
