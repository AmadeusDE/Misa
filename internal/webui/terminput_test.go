package webui

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AmadeusDE/Misa/internal/termkeys"
)

func TestKeyFromDOMEncoding(t *testing.T) {
	cases := []struct {
		name string
		ev   inputEvent
		want string
		app  bool
	}{
		{"letter", inputEvent{Key: "a", Code: "KeyA"}, "a", false},
		{"shift letter", inputEvent{Key: "A", Code: "KeyA", Shift: true}, "A", false},
		{"symbol", inputEvent{Key: "!", Code: "Digit1", Shift: true}, "!", false},
		{"utf8", inputEvent{Key: "é"}, "é", false},
		{"enter", inputEvent{Key: "Enter"}, "\r", false},
		{"space", inputEvent{Key: " ", Code: "Space"}, " ", false},
		{"ctrl space", inputEvent{Key: " ", Code: "Space", Ctrl: true}, "\x00", false},
		{"backspace", inputEvent{Key: "Backspace"}, "\x7f", false},
		{"ctrl a", inputEvent{Key: "a", Code: "KeyA", Ctrl: true}, "\x01", false},
		{"ctrl shift a", inputEvent{Key: "A", Code: "KeyA", Ctrl: true, Shift: true}, "\x01", false},
		{"ctrl on cyrillic layout", inputEvent{Key: "ф", Code: "KeyA", Ctrl: true}, "\x01", false},
		{"alt x", inputEvent{Key: "x", Code: "KeyX", Alt: true}, "\x1bx", false},
		{"ctrl [", inputEvent{Key: "[", Code: "BracketLeft", Ctrl: true}, "\x1b", false},
		{"escape", inputEvent{Key: "Escape"}, "\x1b", false},
		{"tab", inputEvent{Key: "Tab"}, "\t", false},
		{"shift tab", inputEvent{Key: "Tab", Shift: true}, "\x1b[Z", false},
		{"up", inputEvent{Key: "ArrowUp"}, "\x1b[A", false},
		{"up app mode", inputEvent{Key: "ArrowUp"}, "\x1bOA", true},
		{"ctrl right", inputEvent{Key: "ArrowRight", Ctrl: true}, "\x1b[1;5C", false},
		{"home", inputEvent{Key: "Home"}, "\x1b[H", false},
		{"delete", inputEvent{Key: "Delete"}, "\x1b[3~", false},
		{"page down", inputEvent{Key: "PageDown"}, "\x1b[6~", false},
		{"f1", inputEvent{Key: "F1"}, "\x1bOP", false},
		{"f5", inputEvent{Key: "F5"}, "\x1b[15~", false},
		{"shift f12", inputEvent{Key: "F12", Shift: true}, "\x1b[24;2~", false},
		{"emoji", inputEvent{Key: "👍"}, "👍", false},
		{"ctrl n", inputEvent{Key: "n", Code: "KeyN", Ctrl: true}, "\x0e", false},
		{"ctrl t", inputEvent{Key: "t", Code: "KeyT", Ctrl: true}, "\x14", false},
		{"ctrl w", inputEvent{Key: "w", Code: "KeyW", Ctrl: true}, "\x17", false},
		{"ctrl q", inputEvent{Key: "q", Code: "KeyQ", Ctrl: true}, "\x11", false},
		{"ctrl shift w", inputEvent{Key: "W", Code: "KeyW", Ctrl: true, Shift: true}, "\x17", false},
		{"ctrl alt b", inputEvent{Key: "b", Code: "KeyB", Ctrl: true, Alt: true}, "\x1b\x02", false},
		{"ctrl slash", inputEvent{Key: "/", Code: "Slash", Ctrl: true}, "\x1f", false},
		{"ctrl backslash", inputEvent{Key: "\\", Code: "Backslash", Ctrl: true}, "\x1c", false},
		{"ctrl ]", inputEvent{Key: "]", Code: "BracketRight", Ctrl: true}, "\x1d", false},
		{"ctrl minus", inputEvent{Key: "-", Code: "Minus", Ctrl: true}, "\x1f", false},
		{"ctrl backspace", inputEvent{Key: "Backspace", Ctrl: true}, "\x08", false},
		{"ctrl tab", inputEvent{Key: "Tab", Ctrl: true}, "\x1b[9;5u", false},
		{"ctrl shift tab", inputEvent{Key: "Tab", Ctrl: true, Shift: true}, "\x1b[9;6u", false},
		{"ctrl page up", inputEvent{Key: "PageUp", Ctrl: true}, "\x1b[5;5~", false},
		{"ctrl page down", inputEvent{Key: "PageDown", Ctrl: true}, "\x1b[6;5~", false},
		{"alt left", inputEvent{Key: "ArrowLeft", Alt: true}, "\x1b[1;3D", false},
		{"alt right", inputEvent{Key: "ArrowRight", Alt: true}, "\x1b[1;3C", false},
		{"alt backspace", inputEvent{Key: "Backspace", Alt: true}, "\x1b\x7f", false},
		{"alt option letter on mac", inputEvent{Key: "≈", Code: "KeyX", Alt: true}, "\x1bx", false},
		{"f10", inputEvent{Key: "F10"}, "\x1b[21~", false},
		{"ctrl f5", inputEvent{Key: "F5", Ctrl: true}, "\x1b[15;5~", false},
	}
	for _, c := range cases {
		k, ok := keyFromDOM(c.ev)
		if !ok {
			t.Errorf("%s: no key", c.name)
			continue
		}
		if got := string(termkeys.KeyPress(k, c.app)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestEveryCtrlLetterEncodes(t *testing.T) {
	for r := 'a'; r <= 'z'; r++ {
		ev := inputEvent{Key: string(r), Code: "Key" + string(r-32), Ctrl: true}
		k, ok := keyFromDOM(ev)
		if !ok {
			t.Fatalf("ctrl+%c: no key", r)
		}
		if got := termkeys.KeyPress(k, false); len(got) != 1 || got[0] != byte(r-'a'+1) {
			t.Errorf("ctrl+%c = %q", r, got)
		}
	}
}

func TestKeyFromDOMIgnoresNonKeys(t *testing.T) {
	for _, key := range []string{"Shift", "Control", "Alt", "Meta", "CapsLock", "Dead", "Process", "Unidentified", "F13", ""} {
		if _, ok := keyFromDOM(inputEvent{Key: key}); ok {
			t.Errorf("%q should not produce a key", key)
		}
	}
}

func TestClampSize(t *testing.T) {
	if r, c := clampSize(-3, 99999); r != minTermSize || c != maxTermSize {
		t.Fatalf("clampSize = %d,%d", r, c)
	}
}

func runNodeScript(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	static, _ := filepath.Abs("assets/static")
	out, err := exec.Command(node, filepath.Join("testdata", script), static).CombinedOutput()
	if err != nil {
		t.Fatalf("node %s: %v\n%s", script, err, out)
	}
}

func TestJSKeyMapping(t *testing.T) { runNodeScript(t, "termkeys_test.js") }

func TestJSInputBatchesSurviveLostResponses(t *testing.T) { runNodeScript(t, "termsend_test.js") }
