package webui

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
	"github.com/AmadeusDE/Misa/internal/termkeys"
)

const (
	maxEventsPerBatch = 2000
	minTermSize       = 1
	maxTermSize       = 500
)

// inputEvent is one browser event; which fields matter depends on Type.
type inputEvent struct {
	Type   string `json:"type"`
	Key    string `json:"key"`
	Code   string `json:"code"`
	Ctrl   bool   `json:"ctrl"`
	Alt    bool   `json:"alt"`
	Shift  bool   `json:"shift"`
	Meta   bool   `json:"meta"`
	Text   string `json:"text"`
	Action string `json:"action"`
	Button string `json:"button"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Mods   int    `json:"mods"`
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

type inputBatch struct {
	Seq    uint64       `json:"seq"`
	Events []inputEvent `json:"events"`
}

const (
	modShift = 1
	modAlt   = 2
	modCtrl  = 4
)

var domNamedKeys = map[string]rune{
	"Enter": tea.KeyEnter, "Tab": tea.KeyTab, "Backspace": tea.KeyBackspace, "Escape": tea.KeyEscape,
	"Insert": tea.KeyInsert, "Delete": tea.KeyDelete, "Home": tea.KeyHome, "End": tea.KeyEnd,
	"PageUp": tea.KeyPgUp, "PageDown": tea.KeyPgDown,
	"ArrowUp": tea.KeyUp, "ArrowDown": tea.KeyDown, "ArrowLeft": tea.KeyLeft, "ArrowRight": tea.KeyRight,
}

var domPunctCodes = map[string]rune{
	"Minus": '-', "Equal": '=', "BracketLeft": '[', "BracketRight": ']', "Backslash": '\\',
	"Semicolon": ';', "Quote": '\'', "Backquote": '`', "Comma": ',', "Period": '.', "Slash": '/', "Space": ' ',
}

var mouseButtons = map[string]tea.MouseButton{
	"left": tea.MouseLeft, "middle": tea.MouseMiddle, "right": tea.MouseRight, "none": tea.MouseNone,
	"wheelup": tea.MouseWheelUp, "wheeldown": tea.MouseWheelDown,
	"wheelleft": tea.MouseWheelLeft, "wheelright": tea.MouseWheelRight,
	"back": tea.MouseBackward, "forward": tea.MouseForward,
}

var mouseActions = map[string]termkeys.Action{
	"press": termkeys.Press, "release": termkeys.Release, "motion": termkeys.Motion,
}

// latinFromCode is the unshifted ASCII key a physical key stands for, so
// control combinations keep working on non-Latin layouts.
func latinFromCode(code string) rune {
	switch {
	case strings.HasPrefix(code, "Key") && len(code) == 4:
		return unicode.ToLower(rune(code[3]))
	case strings.HasPrefix(code, "Digit") && len(code) == 6:
		return rune(code[5])
	}
	return domPunctCodes[code]
}

func functionKey(name string) (rune, bool) {
	if len(name) < 2 || name[0] != 'F' {
		return 0, false
	}
	n, err := strconv.Atoi(name[1:])
	if err != nil || n < 1 || n > 12 {
		return 0, false
	}
	return tea.KeyF1 + rune(n-1), true
}

func keyMod(ev inputEvent) tea.KeyMod {
	var m tea.KeyMod
	if ev.Shift {
		m |= tea.ModShift
	}
	if ev.Alt {
		m |= tea.ModAlt
	}
	if ev.Ctrl {
		m |= tea.ModCtrl
	}
	return m
}

// keyFromDOM maps a DOM KeyboardEvent to the key Bubble Tea would report.
// Modifier-only presses, dead keys and unknown names report false.
func keyFromDOM(ev inputEvent) (tea.KeyPressMsg, bool) {
	mod := keyMod(ev)
	if code, ok := domNamedKeys[ev.Key]; ok {
		return tea.KeyPressMsg{Code: code, Mod: mod}, true
	}
	if code, ok := functionKey(ev.Key); ok {
		return tea.KeyPressMsg{Code: code, Mod: mod}, true
	}
	if ev.Key == " " {
		k := tea.KeyPressMsg{Code: tea.KeySpace, Mod: mod}
		if !ev.Ctrl {
			k.Text = " "
		}
		return k, true
	}
	n := utf8.RuneCountInString(ev.Key)
	if n == 0 || n > 1 && isKeyName(ev.Key) {
		return tea.KeyPressMsg{}, false
	}
	r, _ := utf8.DecodeRuneInString(ev.Key)
	code := unicode.ToLower(r)
	text := ev.Key
	if (ev.Ctrl || ev.Alt) && code > unicode.MaxASCII {
		if latin := latinFromCode(ev.Code); latin != 0 {
			code = latin
			text = string(latin)
			if ev.Shift {
				text = strings.ToUpper(text)
			}
		}
	}
	k := tea.KeyPressMsg{Code: code, Mod: mod}
	if !ev.Ctrl {
		k.Text = text
	}
	return k, true
}

func isKeyName(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func clampSize(rows, cols int) (int, int) {
	return min(max(rows, minTermSize), maxTermSize), min(max(cols, minTermSize), maxTermSize)
}

// applyEvent delivers one event to the terminal.
func applyEvent(t *term.Terminal, ev inputEvent) {
	switch ev.Type {
	case "key":
		if k, ok := keyFromDOM(ev); ok && !ev.Meta {
			if b := termkeys.KeyPress(k, t.AppCursorKeys()); b != nil {
				t.SendBytes(b)
			}
		}
	case "mouse":
		sendMouse(t, ev)
	case "paste":
		termkeys.SendPaste(t, ev.Text)
	case "resize":
		if ev.Rows > 0 && ev.Cols > 0 {
			t.Resize(clampSize(ev.Rows, ev.Cols))
		}
	}
}

func sendMouse(t *term.Terminal, ev inputEvent) {
	btn, ok := mouseButtons[ev.Button]
	action, ok2 := mouseActions[ev.Action]
	if !ok || !ok2 {
		return
	}
	var mod tea.KeyMod
	if ev.Mods&modShift != 0 {
		mod |= tea.ModShift
	}
	if ev.Mods&modAlt != 0 {
		mod |= tea.ModAlt
	}
	if ev.Mods&modCtrl != 0 {
		mod |= tea.ModCtrl
	}
	if b := termkeys.MouseReport(tea.Mouse{Button: btn, Mod: mod}, action, ev.X, ev.Y, t.Mouse()); b != nil {
		t.SendBytes(b)
	}
}
