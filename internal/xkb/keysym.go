// Package xkb reads the text keymaps Wayland compositors send and maps
// keycodes plus modifier state to keysyms — the small part of libxkbcommon a
// terminal needs, in pure Go. Dead keys and compose sequences are not handled.
package xkb

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

// Keysym is an X keysym.
type Keysym uint32

const (
	NoSymbol     Keysym = 0
	unicodeBase  Keysym = 0x1000000
	BackSpace    Keysym = 0xff08
	Tab          Keysym = 0xff09
	Return       Keysym = 0xff0d
	Escape       Keysym = 0xff1b
	Home         Keysym = 0xff50
	Left         Keysym = 0xff51
	Up           Keysym = 0xff52
	Right        Keysym = 0xff53
	Down         Keysym = 0xff54
	PageUp       Keysym = 0xff55
	PageDown     Keysym = 0xff56
	End          Keysym = 0xff57
	Insert       Keysym = 0xff63
	Delete       Keysym = 0xffff
	ISOLeftTab   Keysym = 0xfe20
	KPEnter      Keysym = 0xff8d
	KPHome       Keysym = 0xff95
	KPDelete     Keysym = 0xff9f
	F1           Keysym = 0xffbe
	F12          Keysym = 0xffc9
	F35          Keysym = 0xffe0
	modifierLow  Keysym = 0xffe1
	modifierHigh Keysym = 0xffee
	isoLockLow   Keysym = 0xfe01
	isoLockHigh  Keysym = 0xfe13
	NumLockSym   Keysym = 0xff7f
	ScrollLock   Keysym = 0xff14
)

// keysymTable is "name keysym [unicode]" per line, hex, taken from
// libxkbcommon's xkbcommon-keysyms.h (X11 licence, see docs/CREDITS.md).
//
//go:embed keysyms.txt
var keysymTable string

var (
	tableOnce sync.Once
	byName    map[string]Keysym
	toUnicode map[Keysym]rune
)

func loadTable() {
	byName = make(map[string]Keysym, 2700)
	toUnicode = make(map[Keysym]rune, 1500)
	for line := range strings.Lines(keysymTable) {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		ks, err := strconv.ParseUint(f[1], 16, 32)
		if err != nil {
			continue
		}
		byName[f[0]] = Keysym(ks)
		if len(f) > 2 {
			if u, err := strconv.ParseUint(f[2], 16, 32); err == nil {
				if _, seen := toUnicode[Keysym(ks)]; !seen {
					toUnicode[Keysym(ks)] = rune(u)
				}
			}
		}
	}
}

// ParseKeysym reads a keysym as a keymap writes it: a name, a hex number, a
// single digit, or Unnnn.
func ParseKeysym(s string) Keysym {
	tableOnce.Do(loadTable)
	switch {
	case len(s) == 1 && s[0] >= '0' && s[0] <= '9':
		return Keysym(s[0])
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		v, _ := strconv.ParseUint(s[2:], 16, 32)
		return Keysym(v)
	}
	if ks, ok := byName[s]; ok {
		return ks
	}
	if len(s) > 1 && s[0] == 'U' {
		if v, err := strconv.ParseUint(s[1:], 16, 32); err == nil {
			return unicodeBase + Keysym(v)
		}
	}
	return NoSymbol
}

// Rune is the character ks types, or 0 for keys that type none.
func (ks Keysym) Rune() rune {
	switch {
	case ks >= 0x20 && ks <= 0x7e, ks >= 0xa0 && ks <= 0xff:
		return rune(ks)
	case ks >= unicodeBase+0x20 && ks <= unicodeBase+0x10ffff:
		return rune(ks - unicodeBase)
	case ks >= 0xfd00:
		return keypadRune(ks)
	}
	tableOnce.Do(loadTable)
	return toUnicode[ks]
}

// keypadRune types the keypad's digits and operators; the remaining keysyms
// at 0xfd00 and above are function keys and type nothing.
func keypadRune(ks Keysym) rune {
	if ks >= 0xffaa && ks <= 0xffb9 || ks == 0xff80 || ks == 0xffbd {
		tableOnce.Do(loadTable)
		return toUnicode[ks]
	}
	return 0
}

// IsModifier reports keys that only change the state of others: shifts,
// locks and level switches, which never auto-repeat.
func (ks Keysym) IsModifier() bool {
	return ks >= modifierLow && ks <= modifierHigh ||
		ks >= isoLockLow && ks <= isoLockHigh ||
		ks == NumLockSym || ks == ScrollLock
}
