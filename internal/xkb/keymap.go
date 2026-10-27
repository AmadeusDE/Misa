package xkb

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// Real modifier bits, in the order wl_keyboard.modifiers masks use.
const (
	ModShift uint32 = 1 << iota
	ModLock
	ModControl
	ModMod1
	ModMod2
	ModMod3
	ModMod4
	ModMod5
)

var realMods = map[string]uint32{
	"shift": ModShift, "lock": ModLock, "control": ModControl,
	"mod1": ModMod1, "mod2": ModMod2, "mod3": ModMod3, "mod4": ModMod4, "mod5": ModMod5,
}

// virtualModKeys are the keysyms whose modifier_map entry decides which real
// modifier a virtual one stands for, as the compat interprets would.
var virtualModKeys = map[string][]string{
	"numlock":    {"Num_Lock"},
	"alt":        {"Alt_L", "Alt_R"},
	"meta":       {"Meta_L", "Meta_R"},
	"super":      {"Super_L", "Super_R"},
	"hyper":      {"Hyper_L", "Hyper_R"},
	"levelthree": {"ISO_Level3_Shift"},
	"levelfive":  {"ISO_Level5_Shift"},
}

var virtualModDefaults = map[string]uint32{
	"numlock": ModMod2, "alt": ModMod1, "meta": ModMod1, "super": ModMod4,
	"hyper": ModMod4, "levelthree": ModMod5, "levelfive": ModMod3,
}

type keyType struct {
	mask   uint32
	levels map[uint32]int
}

type key struct {
	typeName string
	groups   [][]Keysym
}

// Keymap is a parsed xkb_keymap.
type Keymap struct {
	codes map[string]uint32
	types map[string]keyType
	keys  map[uint32]*key
	mods  map[string]uint32
}

var (
	reKeycode  = regexp.MustCompile(`<([^>]+)>\s*=\s*(\d+)\s*;`)
	reAlias    = regexp.MustCompile(`alias\s*<([^>]+)>\s*=\s*<([^>]+)>\s*;`)
	reVirtual  = regexp.MustCompile(`virtual_modifiers\s+([^;]+);`)
	reTypeHead = regexp.MustCompile(`type\s+"([^"]+)"\s*\{`)
	reTypeMods = regexp.MustCompile(`modifiers\s*=\s*([^;]+);`)
	reTypeMap  = regexp.MustCompile(`map\[([^\]]+)\]\s*=\s*(\w+)\s*;`)
	reKeyHead  = regexp.MustCompile(`key\s*<([^>]+)>\s*\{`)
	reModMap   = regexp.MustCompile(`modifier_map\s+(\w+)\s*\{([^}]*)\}`)
)

// Parse reads the keymap text a compositor sends (XKB_KEYMAP_FORMAT_TEXT_V1).
func Parse(text string) (*Keymap, error) {
	km := &Keymap{
		codes: map[string]uint32{},
		types: map[string]keyType{},
		keys:  map[uint32]*key{},
		mods:  map[string]uint32{},
	}
	codes, ok := section(text, "xkb_keycodes")
	if !ok {
		return nil, errors.New("xkb: keymap has no xkb_keycodes")
	}
	symbols, ok := section(text, "xkb_symbols")
	if !ok {
		return nil, errors.New("xkb: keymap has no xkb_symbols")
	}
	types, _ := section(text, "xkb_types")
	km.parseKeycodes(codes)
	km.parseSymbols(symbols)
	km.resolveVirtualMods(types, symbols)
	km.parseTypes(types)
	return km, nil
}

// section returns the brace-delimited body following name.
func section(text, name string) (string, bool) {
	i := strings.Index(text, name)
	if i < 0 {
		return "", false
	}
	open := strings.IndexByte(text[i:], '{')
	if open < 0 {
		return "", false
	}
	return block(text, i+open)
}

// block returns the text between the brace at open and its match, skipping
// quoted strings.
func block(text string, open int) (string, bool) {
	depth, quoted := 0, false
	for i := open; i < len(text); i++ {
		switch c := text[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return text[open+1 : i], true
			}
		}
	}
	return "", false
}

func (km *Keymap) parseKeycodes(body string) {
	for _, m := range reKeycode.FindAllStringSubmatch(body, -1) {
		if code, err := strconv.ParseUint(m[2], 10, 32); err == nil {
			km.codes[m[1]] = uint32(code)
		}
	}
	for _, m := range reAlias.FindAllStringSubmatch(body, -1) {
		if code, ok := km.codes[m[2]]; ok {
			km.codes[m[1]] = code
		}
	}
}

func (km *Keymap) parseSymbols(body string) {
	for _, loc := range reKeyHead.FindAllStringSubmatchIndex(body, -1) {
		code, ok := km.codes[body[loc[2]:loc[3]]]
		if !ok {
			continue
		}
		inner, ok := block(body, loc[1]-1)
		if !ok {
			continue
		}
		km.keys[code] = parseKey(inner)
	}
}

func parseKey(body string) *key {
	k := &key{}
	bare := 0
	for _, stmt := range splitTop(body, ',') {
		stmt = strings.TrimSpace(stmt)
		if strings.HasPrefix(stmt, "[") {
			k.setGroup(bare, strings.Trim(stmt, "[] \t\n"))
			bare++
			continue
		}
		lhs, rhs, ok := strings.Cut(stmt, "=")
		if !ok {
			continue
		}
		name, index, _ := strings.Cut(strings.TrimSpace(lhs), "[")
		rhs = strings.TrimSpace(rhs)
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "type":
			k.typeName = strings.Trim(rhs, `"`)
		case "symbols":
			k.setGroup(groupIndex(strings.TrimSuffix(index, "]")), strings.Trim(rhs, "[] \t\n"))
		}
	}
	return k
}

// splitTop splits s at sep outside brackets, braces, parentheses and quotes.
func splitTop(s string, sep byte) []string {
	var parts []string
	depth, quoted, start := 0, false, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '[' || c == '{' || c == '(':
			depth++
		case c == ']' || c == '}' || c == ')':
			depth--
		case c == sep && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func groupIndex(s string) int {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "group")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0
	}
	return n - 1
}

func (k *key) setGroup(i int, list string) {
	if i > 3 {
		return
	}
	for len(k.groups) <= i {
		k.groups = append(k.groups, nil)
	}
	k.groups[i] = parseLevels(list)
}

// parseLevels splits "a, A, { b, c }" into one keysym per level, taking the
// first of a multi-keysym level.
func parseLevels(list string) []Keysym {
	var syms []Keysym
	for _, item := range splitTop(list, ',') {
		item = strings.Trim(strings.TrimSpace(item), "{} \t\n")
		first, _, _ := strings.Cut(item, ",")
		syms = append(syms, ParseKeysym(strings.TrimSpace(first)))
	}
	return syms
}

func (km *Keymap) resolveVirtualMods(types, symbols string) {
	maps := modifierMaps(symbols)
	for name, keysyms := range virtualModKeys {
		km.mods[name] = virtualModDefaults[name]
		for _, sym := range keysyms {
			if real, ok := km.realModOf(ParseKeysym(sym), maps); ok {
				km.mods[name] = real
				break
			}
		}
	}
	if m := reVirtual.FindStringSubmatch(types); m != nil {
		for _, decl := range strings.Split(m[1], ",") {
			name, val, ok := strings.Cut(decl, "=")
			if !ok {
				continue
			}
			if v, err := strconv.ParseUint(strings.TrimSpace(val), 0, 32); err == nil {
				km.mods[strings.ToLower(strings.TrimSpace(name))] = uint32(v)
			}
		}
	}
	for name, bit := range realMods {
		km.mods[name] = bit
	}
	km.mods["none"] = 0
}

// modifierMaps maps key names to the real modifier their modifier_map line
// assigns.
func modifierMaps(symbols string) map[string]uint32 {
	out := map[string]uint32{}
	for _, m := range reModMap.FindAllStringSubmatch(symbols, -1) {
		bit, ok := realMods[strings.ToLower(m[1])]
		if !ok {
			continue
		}
		for _, k := range strings.Split(m[2], ",") {
			out[strings.Trim(strings.TrimSpace(k), "<>")] = bit
		}
	}
	return out
}

func (km *Keymap) realModOf(sym Keysym, maps map[string]uint32) (uint32, bool) {
	for name, bit := range maps {
		code, ok := km.codes[name]
		if !ok {
			continue
		}
		if k := km.keys[code]; k != nil && len(k.groups) > 0 && len(k.groups[0]) > 0 && k.groups[0][0] == sym {
			return bit, true
		}
	}
	return 0, false
}

func (km *Keymap) parseTypes(body string) {
	for _, loc := range reTypeHead.FindAllStringSubmatchIndex(body, -1) {
		inner, ok := block(body, loc[1]-1)
		if !ok {
			continue
		}
		t := keyType{levels: map[uint32]int{}}
		if m := reTypeMods.FindStringSubmatch(inner); m != nil {
			t.mask = km.modMask(m[1])
		}
		for _, m := range reTypeMap.FindAllStringSubmatch(inner, -1) {
			level, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(m[2]), "level"))
			if err == nil && level >= 1 {
				t.levels[km.modMask(m[1])] = level
			}
		}
		km.types[body[loc[2]:loc[3]]] = t
	}
}

func (km *Keymap) modMask(expr string) uint32 {
	var mask uint32
	for _, name := range strings.Split(expr, "+") {
		mask |= km.mods[strings.ToLower(strings.TrimSpace(name))]
	}
	return mask
}

// Mod is the real modifier mask a modifier name ("Shift", "Alt", "Super")
// stands for in this keymap.
func (km *Keymap) Mod(name string) uint32 { return km.mods[strings.ToLower(name)] }

// Keysym is what keycode (an xkb keycode: evdev + 8) types with the active
// modifier mask and layout group.
func (km *Keymap) Keysym(code, mods, group uint32) Keysym {
	k := km.keys[code]
	if k == nil || len(k.groups) == 0 {
		return NoSymbol
	}
	levels := k.groups[int(group)%len(k.groups)]
	if len(levels) == 0 {
		levels = k.groups[0]
	}
	level := km.level(k, levels, mods)
	if level < 0 || level >= len(levels) {
		return NoSymbol
	}
	return levels[level]
}

// BaseKeysyms lists what keycode types at the base level in every group, for
// finding a Latin letter behind a control combination on another layout.
func (km *Keymap) BaseKeysyms(code uint32) []Keysym {
	k := km.keys[code]
	if k == nil {
		return nil
	}
	var out []Keysym
	for _, g := range k.groups {
		if len(g) > 0 {
			out = append(out, g[0])
		}
	}
	return out
}

func (km *Keymap) level(k *key, levels []Keysym, mods uint32) int {
	t, ok := km.types[k.typeName]
	if !ok {
		t, ok = km.types[implicitType(levels)]
	}
	if !ok {
		return fallbackLevel(levels, mods)
	}
	if lvl, ok := t.levels[mods&t.mask]; ok {
		return lvl - 1
	}
	return 0
}

// implicitType is the type xkbcomp would pick for a key that names none.
func implicitType(levels []Keysym) string {
	switch {
	case len(levels) <= 1:
		return "ONE_LEVEL"
	case isKeypad(levels[0]) || isKeypad(levels[1]):
		return "KEYPAD"
	case len(levels) <= 2 && isCasePair(levels[0], levels[1]):
		return "ALPHABETIC"
	case len(levels) <= 2:
		return "TWO_LEVEL"
	case isCasePair(levels[0], levels[1]):
		return "FOUR_LEVEL_ALPHABETIC"
	}
	return "FOUR_LEVEL"
}

func fallbackLevel(levels []Keysym, mods uint32) int {
	shift := mods&ModShift != 0
	if mods&ModLock != 0 && len(levels) >= 2 && isCasePair(levels[0], levels[1]) {
		shift = !shift
	}
	if shift && len(levels) > 1 {
		return 1
	}
	return 0
}

func isKeypad(ks Keysym) bool { return ks >= 0xff80 && ks <= 0xffbd }

func isCasePair(lower, upper Keysym) bool {
	l, u := lower.Rune(), upper.Rune()
	return l != 0 && u != 0 && l != u && strings.ToUpper(string(l)) == string(u)
}
