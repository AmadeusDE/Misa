package vi

import (
	tea "charm.land/bubbletea/v2"
	"strings"
	"testing"
)

func TestShellNormalFilterPrompts(t *testing.T) {
	for _, tt := range []struct{ keys, want string }{
		{"!!", "1,1!"}, {"2!!", "1,2!"}, {"!j", "1,2!"}, {"2!2j", "1,5!"}, {"!fb", "1,1!"}, {"!/five<CR>", "1,4!"}, {"!w", "1,1!"}, {"!2w", "1,2!"}, {"!G", "1,6!"},
	} {
		t.Run(tt.keys, func(t *testing.T) {
			e := New("", t.TempDir())
			e.lines = []string{"abc", "two", "three", "four", "five", "six"}
			e.Resize(80, 24)
			for _, k := range replayKeys(t, tt.keys) {
				e.Update(k)
			}
			if e.mode != ModeCommand || e.cmdBuf != tt.want {
				t.Fatalf("mode=%v prompt=%q; want command %q", e.mode, e.cmdBuf, tt.want)
			}
		})
	}
}

func TestShellOutputModalScrollAndSanitize(t *testing.T) {
	e := New("", t.TempDir())
	e.Resize(24, 4)
	e.shellOutput = &shellOutput{text: "first\nsecond\n\x1b]0;evil\x07third\nfourth\nfifth"}
	if v := e.View(); !strings.Contains(v, "first") || strings.Contains(v, "evil") || strings.Contains(v, "fifth") {
		t.Fatalf("initial output %q", v)
	}
	e.Update(keyFromName("j"))
	e.Update(keyFromName("j"))
	if v := e.View(); !strings.Contains(v, "third") || strings.Contains(v, "evil") {
		t.Fatalf("scrolled output %q", v)
	}
	e.Update(tea.WindowSizeMsg{Width: 30, Height: 7})
	if v := e.View(); !strings.Contains(v, "fifth") {
		t.Fatalf("resized output %q", v)
	}
	e.Update(keyFromName("x"))
	if e.shellOutput == nil || e.lines[0] != "" {
		t.Fatal("output did not trap editing")
	}
	e.Update(keyFromName("enter"))
	if e.shellOutput != nil {
		t.Fatal("Enter did not dismiss")
	}
}

func TestShellFilterMotionCancelAndBackward(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"one", "two", "three", "four"}
	e.Resize(80, 24)
	e.row = 2
	for _, k := range replayKeys(t, "!k") {
		e.Update(k)
	}
	if e.mode != ModeCommand || e.cmdBuf != "2,3!" || e.row != 2 {
		t.Fatalf("backward motion: mode=%v cmd=%q row=%d", e.mode, e.cmdBuf, e.row)
	}
	e.Update(keyFromName("esc"))
	for _, k := range replayKeys(t, "!f<Esc>") {
		e.Update(k)
	}
	if e.mode != ModeNormal || e.pendingOp != 0 || e.pendingKey != "" {
		t.Fatal("motion cancellation leaked state")
	}
}
