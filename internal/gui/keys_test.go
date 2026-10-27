package gui

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/ui"
	"github.com/AmadeusDE/Misa/internal/xkb"
)

func encoded(t *testing.T, sym xkb.Keysym, base []xkb.Keysym, m keyMods) string {
	t.Helper()
	k, ok := teaKey(sym, base, m)
	if !ok {
		return ""
	}
	return string(ui.KeyBytes(k, false))
}

func TestKeysEncodeLikeXterm(t *testing.T) {
	cyrEs := xkb.ParseKeysym("Cyrillic_es")
	tests := []struct {
		name string
		sym  xkb.Keysym
		base []xkb.Keysym
		m    keyMods
		want string
	}{
		{"letter", 'a', []xkb.Keysym{'a'}, keyMods{}, "a"},
		{"shifted letter", 'A', []xkb.Keysym{'a'}, keyMods{shift: true}, "A"},
		{"shifted symbol", '!', []xkb.Keysym{'1'}, keyMods{shift: true}, "!"},
		{"ctrl letter", 'c', []xkb.Keysym{'c'}, keyMods{ctrl: true}, "\x03"},
		{"ctrl shift letter", 'C', []xkb.Keysym{'c'}, keyMods{ctrl: true, shift: true}, "\x03"},
		{"alt letter", 'x', []xkb.Keysym{'x'}, keyMods{alt: true}, "\x1bx"},
		{"ctrl on cyrillic layout", cyrEs, []xkb.Keysym{cyrEs, 'c'}, keyMods{ctrl: true}, "\x03"},
		{"cyrillic text", cyrEs, []xkb.Keysym{cyrEs, 'c'}, keyMods{}, "с"},
		{"enter", xkb.Return, nil, keyMods{}, "\r"},
		{"keypad enter", xkb.KPEnter, nil, keyMods{}, "\r"},
		{"backtab", xkb.ISOLeftTab, nil, keyMods{shift: true}, "\x1b[Z"},
		{"shift arrow", xkb.Up, nil, keyMods{shift: true}, "\x1b[1;2A"},
		{"f5", xkb.F1 + 4, nil, keyMods{}, "\x1b[15~"},
		{"keypad digit", xkb.ParseKeysym("KP_7"), nil, keyMods{}, "7"},
		{"keypad home", xkb.ParseKeysym("KP_Home"), nil, keyMods{}, "\x1b[H"},
		{"space", ' ', []xkb.Keysym{' '}, keyMods{}, " "},
		{"ctrl space", ' ', []xkb.Keysym{' '}, keyMods{ctrl: true}, "\x00"},
		{"euro", xkb.ParseKeysym("EuroSign"), []xkb.Keysym{'e'}, keyMods{}, "€"},
		{"modifier alone", xkb.ParseKeysym("Shift_L"), nil, keyMods{shift: true}, ""},
		{"dead key", xkb.ParseKeysym("dead_acute"), nil, keyMods{}, ""},
	}
	for _, tt := range tests {
		if got := encoded(t, tt.sym, tt.base, tt.m); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestGUIShortcuts(t *testing.T) {
	ctrlShift := keyMods{ctrl: true, shift: true}
	tests := []struct {
		sym  xkb.Keysym
		base xkb.Keysym
		m    keyMods
		want shortcut
	}{
		{'V', 'v', ctrlShift, pasteShortcut},
		{'v', 'v', keyMods{ctrl: true}, noShortcut},
		{xkb.Insert, xkb.Insert, keyMods{shift: true}, pasteShortcut},
		{'+', '=', ctrlShift, zoomInShortcut},
		{'_', '-', ctrlShift, zoomOutShortcut},
		{')', '0', ctrlShift, zoomResetShortcut},
		{'-', '-', keyMods{ctrl: true}, noShortcut},
	}
	for _, tt := range tests {
		if got := guiShortcut(tt.sym, []xkb.Keysym{tt.base}, tt.m); got != tt.want {
			t.Errorf("%c %+v = %d, want %d", tt.sym, tt.m, got, tt.want)
		}
	}
}
