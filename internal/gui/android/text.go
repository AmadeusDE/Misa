package android

import "github.com/AmadeusDE/Misa/internal/gui"

// EventsForText turns an IME commit into GUI events. With no Ctrl or Alt in
// meta the commit stays one text event, so a word is not split into keys.
// A latched modifier applies to every rune: letters and the symbols the key
// map knows become key events (Ctrl+w is a control byte, not the letter w),
// and anything else is delivered as text.
func EventsForText(text string, meta int) []gui.Event {
	if text == "" {
		return nil
	}
	if meta&(MetaCtrlOn|MetaCtrlLeft|MetaCtrlRight|MetaAltOn|MetaAltLeft|MetaAltRight) == 0 {
		return []gui.Event{{Kind: gui.EventText, Text: text}}
	}
	out := make([]gui.Event, 0, len(text))
	for _, r := range text {
		code, extra, ok := codeForRune(r)
		if !ok {
			out = append(out, gui.Event{Kind: gui.EventText, Text: string(r), DismissHold: true})
			continue
		}
		ev, ok := TranslateKey(code, meta|extra, true)
		if !ok {
			out = append(out, gui.Event{Kind: gui.EventText, Text: string(r), DismissHold: true})
			continue
		}
		out = append(out, ev)
	}
	return out
}

func codeForRune(r rune) (code, meta int, ok bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return keyA + int(r-'a'), 0, true
	case r >= 'A' && r <= 'Z':
		return keyA + int(r-'A'), MetaShiftOn, true
	case r >= '0' && r <= '9':
		return key0 + int(r-'0'), 0, true
	}
	if k, ok := runeKeys[r]; ok {
		return k.code, k.meta, true
	}
	switch r {
	case ' ':
		return keySpace, 0, true
	case '\n', '\r':
		return keyEnter, 0, true
	case '\t':
		return keyTab, 0, true
	default:
		return 0, 0, false
	}
}

type runeKey struct{ code, meta int }

// runeKeys is the inverse of symbol, including the shifted glyph of each key.
var runeKeys = map[rune]runeKey{
	')':  {key0, MetaShiftOn},
	'!':  {key0 + 1, MetaShiftOn},
	'@':  {key0 + 2, MetaShiftOn},
	'#':  {key0 + 3, MetaShiftOn},
	'$':  {key0 + 4, MetaShiftOn},
	'%':  {key0 + 5, MetaShiftOn},
	'^':  {key0 + 6, MetaShiftOn},
	'&':  {key0 + 7, MetaShiftOn},
	'*':  {key0 + 8, MetaShiftOn},
	'(':  {key0 + 9, MetaShiftOn},
	',':  {keyComma, 0},
	'<':  {keyComma, MetaShiftOn},
	'.':  {keyPeriod, 0},
	'>':  {keyPeriod, MetaShiftOn},
	'`':  {keyGrave, 0},
	'~':  {keyGrave, MetaShiftOn},
	'-':  {keyMinus, 0},
	'_':  {keyMinus, MetaShiftOn},
	'=':  {keyEquals, 0},
	'+':  {keyEquals, MetaShiftOn},
	'[':  {keyLBracket, 0},
	'{':  {keyLBracket, MetaShiftOn},
	']':  {keyRBracket, 0},
	'}':  {keyRBracket, MetaShiftOn},
	'\\': {keyBackslash, 0},
	'|':  {keyBackslash, MetaShiftOn},
	';':  {keySemicolon, 0},
	':':  {keySemicolon, MetaShiftOn},
	'\'': {keyApostrophe, 0},
	'"':  {keyApostrophe, MetaShiftOn},
	'/':  {keySlash, 0},
	'?':  {keySlash, MetaShiftOn},
}
