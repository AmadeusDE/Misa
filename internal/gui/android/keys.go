package android

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/gui"
)

// Android KeyEvent meta bits. The aggregate *_ON flags are what Java sets
// when either side is down; the side bits are accepted on their own too.
const (
	MetaShiftOn    = 0x1
	MetaAltOn      = 0x2
	MetaShiftLeft  = 0x40
	MetaShiftRight = 0x80
	MetaAltLeft    = 0x10
	MetaAltRight   = 0x20
	MetaCtrlOn     = 0x1000
	MetaCtrlLeft   = 0x2000
	MetaCtrlRight  = 0x4000
	MetaMetaOn     = 0x10000
	MetaMetaLeft   = 0x20000
	MetaMetaRight  = 0x40000
	MetaCapsLockOn = 0x100000
)

const (
	keyBack        = 4
	keyDpadUp      = 19
	keyDpadDown    = 20
	keyDpadLeft    = 21
	keyDpadRight   = 22
	keyDpadCenter  = 23
	keyA           = 29
	keyZ           = 54
	keyComma       = 55
	keyPeriod      = 56
	keyAltLeft     = 57
	keyAltRight    = 58
	keyShiftLeft   = 59
	keyShiftRight  = 60
	keyTab         = 61
	keySpace       = 62
	keyEnter       = 66
	keyDel         = 67
	keyGrave       = 68
	keyMinus       = 69
	keyEquals      = 70
	keyLBracket    = 71
	keyRBracket    = 72
	keyBackslash   = 73
	keySemicolon   = 74
	keyApostrophe  = 75
	keySlash       = 76
	keyPageUp      = 92
	keyPageDown    = 93
	keyEscape      = 111
	keyForwardDel  = 112
	keyCtrlLeft    = 113
	keyCtrlRight   = 114
	keyMetaLeft    = 117
	keyMetaRight   = 118
	keyMoveHome    = 122
	keyMoveEnd     = 123
	keyInsert      = 124
	keyF1          = 131
	keyF12         = 142
	keyNumpad0     = 144
	keyNumpad9     = 153
	keyNumpadDiv   = 154
	keyNumpadMul   = 155
	keyNumpadSub   = 156
	keyNumpadAdd   = 157
	keyNumpadDot   = 158
	keyNumpadEnter = 160
	key0           = 7
	key9           = 16
)

// TranslateKey maps an Android key code and meta state onto a gui key
// event. Key-up and bare modifiers produce nothing: modifiers ride on the
// next key's meta state, and the core treats every key event as a press.
// Printing keys carry Text. Ctrl suppresses Text so the core emits a
// control byte. BACK is Escape (the activity may consume it first).
func TranslateKey(code, meta int, down bool) (gui.Event, bool) {
	if !down || modifierOnly(code) {
		return gui.Event{}, false
	}
	shift, ctrl, alt, super, caps := metaMods(meta)
	k, ok := keyCode(code, shift || caps)
	if !ok {
		return gui.Event{}, false
	}
	if shift {
		k.Mod |= tea.ModShift
	}
	if ctrl {
		k.Mod |= tea.ModCtrl
		k.Text = ""
	}
	if alt {
		k.Mod |= tea.ModAlt
	}
	if super {
		k.Mod |= tea.ModSuper
	}
	return gui.Event{Kind: gui.EventKey, Key: k, DismissHold: true}, true
}

func modifierOnly(code int) bool {
	switch code {
	case keyShiftLeft, keyShiftRight, keyAltLeft, keyAltRight, keyCtrlLeft, keyCtrlRight, keyMetaLeft, keyMetaRight:
		return true
	default:
		return false
	}
}

func metaMods(meta int) (shift, ctrl, alt, super, caps bool) {
	shift = meta&(MetaShiftOn|MetaShiftLeft|MetaShiftRight) != 0
	ctrl = meta&(MetaCtrlOn|MetaCtrlLeft|MetaCtrlRight) != 0
	alt = meta&(MetaAltOn|MetaAltLeft|MetaAltRight) != 0
	super = meta&(MetaMetaOn|MetaMetaLeft|MetaMetaRight) != 0
	caps = meta&MetaCapsLockOn != 0
	return
}

func keyCode(code int, shift bool) (tea.Key, bool) {
	if code >= keyA && code <= keyZ {
		c := rune('a' + code - keyA)
		k := tea.Key{Code: c}
		if shift {
			k.Text = string(c - 'a' + 'A')
		} else {
			k.Text = string(c)
		}
		return k, true
	}
	if code >= key0 && code <= key9 {
		d := rune('0' + code - key0)
		k := tea.Key{Code: d, Text: string(d)}
		if shift {
			k.Text = string(shiftedDigit[code-key0])
		}
		return k, true
	}
	if code >= keyF1 && code <= keyF12 {
		return tea.Key{Code: tea.KeyF1 + rune(code-keyF1)}, true
	}
	if code >= keyNumpad0 && code <= keyNumpad9 {
		d := rune('0' + code - keyNumpad0)
		return tea.Key{Code: d, Text: string(d)}, true
	}
	if r, text, ok := symbol(code, shift); ok {
		return tea.Key{Code: r, Text: text}, true
	}
	if named, ok := namedKey(code); ok {
		return tea.Key{Code: named}, true
	}
	return tea.Key{}, false
}

var shiftedDigit = [10]rune{')', '!', '@', '#', '$', '%', '^', '&', '*', '('}

func symbol(code int, shift bool) (codePoint rune, text string, ok bool) {
	type pair struct{ plain, shifted rune }
	var p pair
	switch code {
	case keySpace:
		p = pair{' ', ' '}
	case keyComma:
		p = pair{',', '<'}
	case keyPeriod:
		p = pair{'.', '>'}
	case keyGrave:
		p = pair{'`', '~'}
	case keyMinus:
		p = pair{'-', '_'}
	case keyEquals:
		p = pair{'=', '+'}
	case keyLBracket:
		p = pair{'[', '{'}
	case keyRBracket:
		p = pair{']', '}'}
	case keyBackslash:
		p = pair{'\\', '|'}
	case keySemicolon:
		p = pair{';', ':'}
	case keyApostrophe:
		p = pair{'\'', '"'}
	case keySlash:
		p = pair{'/', '?'}
	case keyNumpadDiv:
		p = pair{'/', '/'}
	case keyNumpadMul:
		p = pair{'*', '*'}
	case keyNumpadSub:
		p = pair{'-', '-'}
	case keyNumpadAdd:
		p = pair{'+', '+'}
	case keyNumpadDot:
		p = pair{'.', '.'}
	default:
		return 0, "", false
	}
	r := p.plain
	if shift {
		r = p.shifted
	}
	if code == keySpace {
		return tea.KeySpace, " ", true
	}
	return p.plain, string(r), true
}

func namedKey(code int) (rune, bool) {
	switch code {
	case keyEnter, keyNumpadEnter, keyDpadCenter:
		return tea.KeyEnter, true
	case keyTab:
		return tea.KeyTab, true
	case keyDel:
		return tea.KeyBackspace, true
	case keyForwardDel:
		return tea.KeyDelete, true
	case keyEscape, keyBack:
		return tea.KeyEscape, true
	case keyDpadUp:
		return tea.KeyUp, true
	case keyDpadDown:
		return tea.KeyDown, true
	case keyDpadLeft:
		return tea.KeyLeft, true
	case keyDpadRight:
		return tea.KeyRight, true
	case keyMoveHome:
		return tea.KeyHome, true
	case keyMoveEnd:
		return tea.KeyEnd, true
	case keyPageUp:
		return tea.KeyPgUp, true
	case keyPageDown:
		return tea.KeyPgDown, true
	case keyInsert:
		return tea.KeyInsert, true
	default:
		return 0, false
	}
}
