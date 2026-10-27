package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

func TestMouseReportEncoding(t *testing.T) {
	sgr1002 := term.MouseMode{Tracking: 1002, SGR: true}
	legacy1000 := term.MouseMode{Tracking: 1000}
	any1003 := term.MouseMode{Tracking: 1003, SGR: true}
	press := func(b tea.MouseButton) mouseMsg {
		return mouseMsg{Mouse: tea.Mouse{Button: b}, Action: mousePress}
	}
	tests := []struct {
		name string
		msg  mouseMsg
		mode term.MouseMode
		want string
	}{
		{"sgr left press", press(tea.MouseLeft), sgr1002, "\x1b[<0;5;3M"},
		{"sgr right release", mouseMsg{Mouse: tea.Mouse{Button: tea.MouseRight}, Action: mouseRelease}, sgr1002, "\x1b[<2;5;3m"},
		{"sgr drag", mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft}, Action: mouseMotion}, sgr1002, "\x1b[<32;5;3M"},
		{"sgr wheel up", press(tea.MouseWheelUp), sgr1002, "\x1b[<64;5;3M"},
		{"sgr ctrl click", mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft, Mod: tea.ModCtrl}, Action: mousePress}, sgr1002, "\x1b[<16;5;3M"},
		{"hover in 1003", mouseMsg{Mouse: tea.Mouse{Button: tea.MouseNone}, Action: mouseMotion}, any1003, "\x1b[<35;5;3M"},
		{"hover ignored in 1002", mouseMsg{Mouse: tea.Mouse{Button: tea.MouseNone}, Action: mouseMotion}, sgr1002, ""},
		{"drag ignored in 1000", mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft}, Action: mouseMotion}, legacy1000, ""},
		{"legacy press", press(tea.MouseLeft), legacy1000, "\x1b[M %#"},
		{"legacy release", mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft}, Action: mouseRelease}, legacy1000, "\x1b[M#%#"},
		{"off", press(tea.MouseLeft), term.MouseMode{}, ""},
	}
	for _, tt := range tests {
		if got := string(mouseReport(tt.msg, 4, 2, tt.mode)); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestShiftBypassesMouseTracking(t *testing.T) {
	tm := term.New(5, 10)
	tm.ProcessOutput([]byte("\x1b[?1000h"))
	click := mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft}, Action: mousePress}
	if !forwardMouse(tm, click, 1, 1) {
		t.Error("plain click not forwarded to tracking program")
	}
	click.Mod = tea.ModShift
	if forwardMouse(tm, click, 1, 1) {
		t.Error("shift+click forwarded; it should select instead")
	}
	if forwardMouse(tm, mouseMsg{Mouse: tea.Mouse{Button: tea.MouseLeft}, Action: mousePress}, 1, 9) {
		t.Error("click below the terminal forwarded")
	}
}
