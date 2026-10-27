package vi

import "testing"

func TestKeyFromNameRoundTrips(t *testing.T) {
	for _, name := range []string{"esc", "enter", "backspace", "tab", "up", "left", "ctrl+r", "ctrl+w", "x", "G", "é", "space"} {
		if got := keyFromName(name).String(); got != name {
			t.Errorf("keyFromName(%q).String() = %q", name, got)
		}
	}
}

func TestTypedKeyKeepsSpaceAsText(t *testing.T) {
	space := keyFromName(" ")
	if got := typedKey(space); got != " " {
		t.Errorf("typedKey(space) = %q, want a space", got)
	}
	e := New("", t.TempDir())
	e.processKey("i")
	for _, k := range []string{"a", " ", "b", "esc"} {
		e.processKey(k)
	}
	if got := e.lineRunes(0); string(got) != "a b" {
		t.Errorf("line = %q, want %q", string(got), "a b")
	}
}
