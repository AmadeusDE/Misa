// Package termkeys encodes Bubble Tea keys, mouse events and pastes as the bytes a
// terminal program reads from its PTY, so every frontend shares one xterm encoding.
package termkeys

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

// KeyPress encodes msg the way xterm sends it; appCursor selects application cursor keys.
func KeyPress(msg tea.KeyPressMsg, appCursor bool) []byte { return keyToBytesMode(msg, appCursor) }

// SendKey forwards a key to the session. Ctrl+C abandons an in-process
// builtin, which has no process to signal.
func SendKey(t *term.Terminal, msg tea.KeyPressMsg) {
	if msg.String() == "ctrl+c" && t.Interrupt() {
		return
	}
	if b := keyToBytesMode(msg, t.AppCursorKeys()); b != nil {
		t.SendBytes(b)
	}
}

const pasteEnd = "\x1b[201~"

// PasteBytes encodes pasted text for a child PTY, bracketed when the child
// asked for bracketed paste. End markers inside the text are dropped so a
// paste cannot end the bracket early and run the rest as typed input.
func PasteBytes(text string, t *term.Terminal) []byte {
	if text == "" {
		return nil
	}
	if t.BracketedPaste() {
		for strings.Contains(text, pasteEnd) {
			text = strings.ReplaceAll(text, pasteEnd, "")
		}
		return []byte("\x1b[200~" + text + pasteEnd)
	}
	return []byte(text)
}

// SendPaste writes pasted text to t.
func SendPaste(t *term.Terminal, text string) {
	if b := PasteBytes(text, t); b != nil {
		t.SendBytes(b)
	}
}

// keyToBytesMode encodes a key the way xterm would send it. Keys it has no
// encoding for return nil rather than their display name.
func keyToBytesMode(msg tea.KeyPressMsg, appCursor bool) []byte {
	k := msg.Key()
	b := plainKeyBytes(k)
	if b == nil {
		return namedKeyBytes(k, appCursor)
	}
	if k.Mod.Contains(tea.ModAlt) {
		b = append([]byte{0x1b}, b...)
	}
	return b
}

var (
	singleByteKeys = map[rune]byte{tea.KeyEnter: '\r', tea.KeyTab: '\t', tea.KeyBackspace: 0x7f, tea.KeyEscape: 0x1b, tea.KeySpace: ' '}
	ctrlSymbolKeys = map[rune]byte{' ': 0, '@': 0, '2': 0, '[': 0x1b, '\\': 0x1c, ']': 0x1d, '^': 0x1e, '_': 0x1f, '/': 0x1f, '?': 0x7f,
		'-': 0x1f, '3': 0x1b, '4': 0x1c, '5': 0x1d, '6': 0x1e, '7': 0x1f, '8': 0x7f}
)

// plainKeyBytes encodes keys that reach a program as bytes of their own,
// before any alt prefix: text, control characters and the C0 keys.
func plainKeyBytes(k tea.Key) []byte {
	shift := k.Mod.Contains(tea.ModShift)
	switch {
	case k.Mod.Contains(tea.ModCtrl):
		return ctrlKeyBytes(k.Code)
	case k.Code == tea.KeyTab && shift:
		return nil
	case k.Text != "":
		return []byte(k.Text)
	}
	if b, ok := singleByteKeys[k.Code]; ok {
		return []byte{b}
	}
	if k.Code <= unicode.MaxRune && unicode.IsPrint(k.Code) {
		if shift {
			return []byte(string(unicode.ToUpper(k.Code)))
		}
		return []byte(string(k.Code))
	}
	return nil
}

func ctrlKeyBytes(code rune) []byte {
	switch {
	case code >= 'a' && code <= 'z':
		return []byte{byte(code - 'a' + 1)}
	case code == tea.KeyBackspace:
		return []byte{0x08}
	}
	if b, ok := ctrlSymbolKeys[code]; ok {
		return []byte{b}
	}
	return nil
}

var (
	cursorKeyFinals = map[rune]byte{tea.KeyUp: 'A', tea.KeyDown: 'B', tea.KeyRight: 'C', tea.KeyLeft: 'D', tea.KeyHome: 'H', tea.KeyEnd: 'F'}
	ss3KeyFinals    = map[rune]byte{tea.KeyF1: 'P', tea.KeyF2: 'Q', tea.KeyF3: 'R', tea.KeyF4: 'S'}
	tildeKeyCodes   = map[rune]int{
		tea.KeyInsert: 2, tea.KeyDelete: 3, tea.KeyPgUp: 5, tea.KeyPgDown: 6,
		tea.KeyF5: 15, tea.KeyF6: 17, tea.KeyF7: 18, tea.KeyF8: 19, tea.KeyF9: 20, tea.KeyF10: 21, tea.KeyF11: 23, tea.KeyF12: 24,
	}
)

// xtermModifier is xterm's modifier parameter: 1 + shift + 2·alt + 4·ctrl.
func xtermModifier(mod tea.KeyMod) int {
	n := 1
	if mod.Contains(tea.ModShift) {
		n++
	}
	if mod.Contains(tea.ModAlt) {
		n += 2
	}
	if mod.Contains(tea.ModCtrl) {
		n += 4
	}
	return n
}

// namedKeyBytes encodes special keys with xterm modifier parameters.
func namedKeyBytes(k tea.Key, appCursor bool) []byte {
	mod := xtermModifier(k.Mod)
	if k.Code == tea.KeyTab && mod == 2 {
		return []byte("\x1b[Z")
	}
	if k.Code == tea.KeyTab && mod > 1 {
		return fmt.Appendf(nil, "\x1b[9;%du", mod)
	}
	if f, ok := cursorKeyFinals[k.Code]; ok {
		switch {
		case mod > 1:
			return fmt.Appendf(nil, "\x1b[1;%d%c", mod, f)
		case appCursor:
			return []byte{0x1b, 'O', f}
		}
		return []byte{0x1b, '[', f}
	}
	if f, ok := ss3KeyFinals[k.Code]; ok {
		if mod > 1 {
			return fmt.Appendf(nil, "\x1b[1;%d%c", mod, f)
		}
		return []byte{0x1b, 'O', f}
	}
	if n, ok := tildeKeyCodes[k.Code]; ok {
		if mod > 1 {
			return fmt.Appendf(nil, "\x1b[%d;%d~", n, mod)
		}
		return fmt.Appendf(nil, "\x1b[%d~", n)
	}
	return nil
}
