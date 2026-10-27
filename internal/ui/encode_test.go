package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

func TestKeyBytesExported(t *testing.T) {
	tests := []struct {
		key  tea.Key
		want string
	}{
		{tea.Key{Code: 'a', Text: "a"}, "a"},
		{tea.Key{Code: 'c', Mod: tea.ModCtrl}, "\x03"},
		{tea.Key{Code: 'x', Text: "x", Mod: tea.ModAlt}, "\x1bx"},
		{tea.Key{Code: tea.KeyUp, Mod: tea.ModShift}, "\x1b[1;2A"},
	}
	for _, tt := range tests {
		if got := string(KeyBytes(tt.key, false)); got != tt.want {
			t.Errorf("KeyBytes(%+v) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestMouseBytesExported(t *testing.T) {
	mode := term.MouseMode{Tracking: 1003, SGR: true}
	click := tea.MouseClickMsg{Button: tea.MouseLeft}
	if got := string(MouseBytes(click, 2, 4, mode)); got != "\x1b[<0;3;5M" {
		t.Errorf("click = %q", got)
	}
	release := tea.MouseReleaseMsg{Button: tea.MouseLeft}
	if got := string(MouseBytes(release, 0, 0, mode)); got != "\x1b[<0;1;1m" {
		t.Errorf("release = %q", got)
	}
	wheel := tea.MouseWheelMsg{Button: tea.MouseWheelDown}
	if got := string(MouseBytes(wheel, 0, 0, mode)); got != "\x1b[<65;1;1M" {
		t.Errorf("wheel = %q", got)
	}
}
