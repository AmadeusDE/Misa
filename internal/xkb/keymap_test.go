package xkb

import (
	"strings"
	"testing"
)

const sampleKeymap = `xkb_keymap {
xkb_keycodes "test" {
	minimum = 8;
	<AE01> = 10;
	<AC01> = 38;
	<LFSH> = 50;
	<LALT> = 64;
	<NMLK> = 77;
	<KP1> = 87;
	<RALT> = 108;
	<FK01> = 67;
	<AB01> = 52;
	alias <ALGR> = <RALT>;
};
xkb_types "complete" {
	virtual_modifiers NumLock,Alt,LevelThree,Hyper=0x4000;
	type "ONE_LEVEL" { modifiers= none; };
	type "TWO_LEVEL" { modifiers= Shift; map[Shift]= 2; level_name[1]= "Base"; };
	type "ALPHABETIC" { modifiers= Shift+Lock; map[Shift]= 2; map[Lock]= 2; };
	type "KEYPAD" { modifiers= Shift+NumLock; map[None]= 1; map[Shift]= 2; map[NumLock]= 2; map[Shift+NumLock]= 1; };
	type "FOUR_LEVEL_ALPHABETIC" {
		modifiers= Shift+Lock+LevelThree;
		map[Shift]= Level2; map[Lock]= Level2;
		map[LevelThree]= Level3; map[Shift+LevelThree]= Level4;
	};
	type "CTRL+ALT" { modifiers= Shift+Control+Alt+LevelThree; map[Shift]= 2; map[Control+Alt]= 5; };
};
xkb_compatibility "complete" { interpret Any+AnyOf(all) { action= SetMods(modifiers=modMapMods,clearLocks); }; };
xkb_symbols "pc+us+ru:2" {
	key <AE01> {	[ 1, exclam ] };
	key <AC01> {
		type[Group1]= "FOUR_LEVEL_ALPHABETIC",
		symbols[Group1]= [ a, A, aacute, Aacute ],
		symbols[Group2]= [ Cyrillic_ef, Cyrillic_EF ]
	};
	key <AB01> {	[ 0x7a, 0x5a ], [ 0x6ff, 0x6df ] };
	key <LFSH> {	[ Shift_L ] };
	key <LALT> {	[ Alt_L, Meta_L ] };
	key <NMLK> {	[ Num_Lock ] };
	key <KP1> {	[ KP_End, KP_1 ] };
	key <RALT> {	type= "ONE_LEVEL", symbols[1]= [ ISO_Level3_Shift ], actions[1]= [ SetMods(modifiers=LevelThree) ] };
	key <FK01> {
		type= "CTRL+ALT",
		symbols[1]= [ 0xffbe, 0xffbe, 0xffbe, 0xffbe, 0x1008fe01 ]
	};
	modifier_map Shift { <LFSH> };
	modifier_map Mod1 { <LALT> };
	modifier_map Mod2 { <NMLK> };
	modifier_map Mod5 { <RALT> };
};
};`

func parseSample(t *testing.T) *Keymap {
	t.Helper()
	km, err := Parse(sampleKeymap)
	if err != nil {
		t.Fatal(err)
	}
	return km
}

func TestKeysymLevels(t *testing.T) {
	km := parseSample(t)
	tests := []struct {
		name       string
		code, mods uint32
		group      uint32
		want       Keysym
	}{
		{"digit", 10, 0, 0, '1'},
		{"shifted digit", 10, ModShift, 0, '!'},
		{"letter", 38, 0, 0, 'a'},
		{"shift letter", 38, ModShift, 0, 'A'},
		{"caps letter", 38, ModLock, 0, 'A'},
		{"level three", 38, ModMod5, 0, ParseKeysym("aacute")},
		{"shift level three", 38, ModMod5 | ModShift, 0, ParseKeysym("Aacute")},
		{"control ignored", 38, ModControl, 0, 'a'},
		{"second group", 38, 0, 1, ParseKeysym("Cyrillic_ef")},
		{"bare second group", 52, ModShift, 1, 0x6df},
		{"implicit keypad", 87, 0, 0, ParseKeysym("KP_End")},
		{"numlock keypad", 87, ModMod2, 0, ParseKeysym("KP_1")},
		{"hex function key", 67, 0, 0, F1},
		{"ctrl alt function key", 67, ModControl | ModMod1, 0, 0x1008fe01},
		{"unknown key", 200, 0, 0, NoSymbol},
	}
	for _, tt := range tests {
		if got := km.Keysym(tt.code, tt.mods, tt.group); got != tt.want {
			t.Errorf("%s: got %#x, want %#x", tt.name, got, tt.want)
		}
	}
}

func TestVirtualModifiers(t *testing.T) {
	km := parseSample(t)
	for name, want := range map[string]uint32{"Alt": ModMod1, "NumLock": ModMod2, "LevelThree": ModMod5, "Hyper": 0x4000, "Super": ModMod4} {
		if got := km.Mod(name); got != want {
			t.Errorf("Mod(%s) = %#x, want %#x", name, got, want)
		}
	}
}

func TestBaseKeysyms(t *testing.T) {
	km := parseSample(t)
	got := km.BaseKeysyms(38)
	if len(got) != 2 || got[0] != 'a' || got[1] != ParseKeysym("Cyrillic_ef") {
		t.Fatalf("BaseKeysyms = %#x", got)
	}
}

func TestKeysymRune(t *testing.T) {
	tests := map[Keysym]rune{
		'a': 'a', 0xe9: 'é', ParseKeysym("Cyrillic_a"): 'а', ParseKeysym("EuroSign"): '€',
		ParseKeysym("U1F600"): '😀', ParseKeysym("KP_1"): '1', ParseKeysym("KP_Add"): '+',
		Return: 0, F1: 0, ParseKeysym("Shift_L"): 0, ParseKeysym("KP_End"): 0,
	}
	for ks, want := range tests {
		if got := ks.Rune(); got != want {
			t.Errorf("Rune(%#x) = %q, want %q", ks, got, want)
		}
	}
}

func TestIsModifier(t *testing.T) {
	for _, name := range []string{"Shift_L", "Control_R", "Alt_L", "Super_L", "ISO_Level3_Shift", "Caps_Lock", "Num_Lock"} {
		if !ParseKeysym(name).IsModifier() {
			t.Errorf("%s is not a modifier", name)
		}
	}
	if Keysym('a').IsModifier() || Return.IsModifier() {
		t.Error("ordinary keys reported as modifiers")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse("not a keymap"); err == nil {
		t.Fatal("no error")
	}
}

func TestLevelZeroDoesNotPanic(t *testing.T) {
	km, err := Parse(strings.Replace(sampleKeymap, `map[Shift]= 2; level_name[1]= "Base";`, `map[Shift]= Level0;`, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := km.Keysym(10, ModShift, 0); got != '1' && got != NoSymbol {
		t.Errorf("Level0 mapping: got %#x", got)
	}
}
