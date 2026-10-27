package gui

import (
	"unicode"

	tea "charm.land/bubbletea/v2"
)

type shortcut int

const (
	noShortcut shortcut = iota
	pasteShortcut
	zoomInShortcut
	zoomOutShortcut
	zoomResetShortcut
)

// shortcutOf recognizes the window's own bindings on an already translated
// key, which never reach the program: ctrl+shift+v and shift+insert paste,
// ctrl+shift with = - 0 zoom.
func shortcutOf(k tea.Key) shortcut {
	if k.Code == tea.KeyInsert && k.Mod&tea.ModShift != 0 && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
		return pasteShortcut
	}
	const need = tea.ModCtrl | tea.ModShift
	if k.Mod&need != need || k.Mod&tea.ModAlt != 0 {
		return noShortcut
	}
	switch unicode.ToLower(k.Code) {
	case 'v':
		return pasteShortcut
	case '=', '+':
		return zoomInShortcut
	case '-':
		return zoomOutShortcut
	case '0':
		return zoomResetShortcut
	default:
		return noShortcut
	}
}
