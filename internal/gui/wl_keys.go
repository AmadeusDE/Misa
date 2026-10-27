//go:build unix && !android

package gui

import (
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/xkb"
)

var namedKeys = map[xkb.Keysym]rune{
	xkb.Return: tea.KeyEnter, xkb.KPEnter: tea.KeyEnter,
	xkb.Tab: tea.KeyTab, xkb.ISOLeftTab: tea.KeyTab,
	xkb.BackSpace: tea.KeyBackspace, xkb.Escape: tea.KeyEscape,
	xkb.Insert: tea.KeyInsert, xkb.Delete: tea.KeyDelete,
	xkb.Home: tea.KeyHome, xkb.End: tea.KeyEnd,
	xkb.PageUp: tea.KeyPgUp, xkb.PageDown: tea.KeyPgDown,
	xkb.Up: tea.KeyUp, xkb.Down: tea.KeyDown, xkb.Left: tea.KeyLeft, xkb.Right: tea.KeyRight,
	0xff95: tea.KeyHome, 0xff96: tea.KeyLeft, 0xff97: tea.KeyUp, 0xff98: tea.KeyRight,
	0xff99: tea.KeyDown, 0xff9a: tea.KeyPgUp, 0xff9b: tea.KeyPgDown, 0xff9c: tea.KeyEnd,
	0xff9e: tea.KeyInsert, xkb.KPDelete: tea.KeyDelete,
}

func init() {
	for i := xkb.Keysym(0); i <= xkb.F12-xkb.F1; i++ {
		namedKeys[xkb.F1+i] = tea.KeyF1 + rune(i)
	}
}

type keyMods struct{ shift, ctrl, alt, super bool }

func (m keyMods) tea() tea.KeyMod {
	var mod tea.KeyMod
	if m.shift {
		mod |= tea.ModShift
	}
	if m.ctrl {
		mod |= tea.ModCtrl
	}
	if m.alt {
		mod |= tea.ModAlt
	}
	if m.super {
		mod |= tea.ModSuper
	}
	return mod
}

// teaKey describes a key press the way Bubble Tea would: Code is the
// unshifted key (a Latin one, for control combinations on other layouts),
// Text what it types. base lists the key's level-one keysyms per layout.
func teaKey(sym xkb.Keysym, base []xkb.Keysym, m keyMods) (tea.Key, bool) {
	mod := m.tea()
	if code, ok := namedKeys[sym]; ok {
		if sym == xkb.ISOLeftTab {
			mod |= tea.ModShift
		}
		return tea.Key{Code: code, Mod: mod}, true
	}
	r := sym.Rune()
	if r == 0 {
		return tea.Key{}, false
	}
	k := tea.Key{Code: baseRune(r, base, m.ctrl || m.alt), Mod: mod}
	if !m.ctrl {
		k.Text = string(r)
	}
	return k, true
}

func baseRune(r rune, base []xkb.Keysym, wantLatin bool) rune {
	if len(base) == 0 || base[0].Rune() == 0 {
		return unicode.ToLower(r)
	}
	first := unicode.ToLower(base[0].Rune())
	if wantLatin && first > unicode.MaxASCII {
		for _, ks := range base[1:] {
			if b := unicode.ToLower(ks.Rune()); b != 0 && b <= unicode.MaxASCII {
				return b
			}
		}
	}
	return first
}

// guiShortcut recognizes the window's own bindings from a keysym. The core
// decides the same thing from the tea.Key this produces.
func guiShortcut(sym xkb.Keysym, base []xkb.Keysym, m keyMods) shortcut {
	k, ok := teaKey(sym, base, m)
	if !ok {
		return noShortcut
	}
	return shortcutOf(k)
}
