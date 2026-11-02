package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/term"
)

func imgviewSnapshotText(s *term.Snapshot) string {
	var text strings.Builder
	for _, c := range s.Cells {
		text.WriteRune(c.Ch)
	}
	return text.String()
}

func imgviewAnimationColour(t *testing.T, s *term.Snapshot) rgb {
	t.Helper()
	if len(s.Graphics) != 1 {
		return rgb{}
	}
	r, c := imgviewRender(t, s)
	return colourAt(c, r.pad+100, r.pad+110)
}

func TestImgviewRealRunnerAnimation(t *testing.T) {
	t.Setenv("MISA_IMGVIEW_ANIM_TEST", "1")
	tm := imgviewTerminal(t, true)
	red, green := rgb{255, 0, 0}, rgb{0, 255, 0}
	imgviewWait(t, tm, "animated red frame and focus reporting", func(s *term.Snapshot) bool { return tm.FocusReporting() && imgviewAnimationColour(t, s) == red })
	imgviewWait(t, tm, "animation advances to green", func(s *term.Snapshot) bool { return imgviewAnimationColour(t, s) == green })
	// Pause while green is displayed, then wait for the actual status update so
	// a timer already in flight cannot make the stability check pass spuriously.
	tm.SendBytes([]byte{1})
	imgviewWait(t, tm, "paused green frame", func(s *term.Snapshot) bool {
		return imgviewAnimationColour(t, s) == green && strings.Contains(imgviewSnapshotText(s), "paused")
	})
	time.Sleep(1400 * time.Millisecond)
	var s term.Snapshot
	tm.Snapshot(&s)
	if got := imgviewAnimationColour(t, &s); got != green {
		t.Fatalf("paused frame changed to %v", got)
	}
	tm.SendBytes([]byte{16}) // Ctrl+p
	imgviewWait(t, tm, "step backward to red", func(s *term.Snapshot) bool { return imgviewAnimationColour(t, s) == red })
	tm.SendBytes([]byte{14}) // Ctrl+n
	imgviewWait(t, tm, "step forward to green", func(s *term.Snapshot) bool { return imgviewAnimationColour(t, s) == green })
	// Resume, then blur while it is red. Focus reports use the actual terminal's
	// DEC 1004 path, not direct messages to Viewer.
	tm.SendBytes([]byte{1})
	imgviewWait(t, tm, "resumed red frame", func(s *term.Snapshot) bool { return imgviewAnimationColour(t, s) == red })
	tm.SetFocused(false)
	// Allow the focus report to be processed before the observation window.
	time.Sleep(150 * time.Millisecond)
	tm.Snapshot(&s)
	frozen := imgviewAnimationColour(t, &s)
	if frozen != red && frozen != green {
		t.Fatalf("no frame after focus out: %v", frozen)
	}
	time.Sleep(1400 * time.Millisecond)
	tm.Snapshot(&s)
	if got := imgviewAnimationColour(t, &s); got != frozen {
		t.Fatalf("focus-out frame changed from %v to %v", frozen, got)
	}
	tm.SetFocused(true)
	imgviewWait(t, tm, "focus-in resumes animation", func(s *term.Snapshot) bool {
		got := imgviewAnimationColour(t, s)
		return got != (rgb{}) && got != frozen
	})
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
