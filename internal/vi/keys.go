package vi

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

var namedKeys = map[string]rune{
	"esc": tea.KeyEscape, "enter": tea.KeyEnter, "backspace": tea.KeyBackspace, "tab": tea.KeyTab,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight, "space": tea.KeySpace,
}

// keyFromName builds the key event whose String() is name, so replayed keys
// (macros, :normal) take the same paths as typed ones.
func keyFromName(name string) tea.KeyPressMsg {
	if code, ok := namedKeys[name]; ok {
		return tea.KeyPressMsg{Code: code}
	}
	if letter, ok := strings.CutPrefix(name, "ctrl+"); ok && len(letter) == 1 {
		return tea.KeyPressMsg{Code: rune(letter[0]), Mod: tea.ModCtrl}
	}
	if r, size := utf8.DecodeRuneInString(name); size == len(name) {
		return tea.KeyPressMsg{Code: r, Text: name}
	}
	return tea.KeyPressMsg{Code: tea.KeyExtended, Text: name}
}

// typedKey names a key the way the editor matches it: the typed text when
// there is any (so space is " "), otherwise the key's name ("esc", "ctrl+r").
func typedKey(msg tea.KeyPressMsg) string {
	if msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return msg.Text
	}
	return msg.String()
}
