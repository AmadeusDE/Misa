package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestKeyToBytesAppCursor(t *testing.T) {
	tests := []struct {
		key       string
		appCursor bool
		want      string
	}{
		{"up", false, "\x1b[A"},
		{"up", true, "\x1bOA"},
		{"down", false, "\x1b[B"},
		{"down", true, "\x1bOB"},
		{"right", false, "\x1b[C"},
		{"right", true, "\x1bOC"},
		{"left", false, "\x1b[D"},
		{"left", true, "\x1bOD"},
		{"home", false, "\x1b[H"},
		{"home", true, "\x1bOH"},
		{"end", false, "\x1b[F"},
		{"end", true, "\x1bOF"},
	}
	for _, tt := range tests {
		t.Run(tt.key+"_app="+boolStr(tt.appCursor), func(t *testing.T) {
			msg := tea.KeyPressMsg{Code: keyCode(tt.key)}
			got := keyToBytesMode(msg, tt.appCursor)
			if string(got) != tt.want {
				t.Errorf("keyToBytesMode(%q, %v) = %q, want %q",
					tt.key, tt.appCursor, got, tt.want)
			}
		})
	}
}

func TestStripNativePrefix(t *testing.T) {
	tests := []struct {
		input      string
		wantNative bool
		wantCmd    string
	}{
		{"! ls -la", true, "ls -la"},
		{"! agy --continue", true, "agy --continue"},
		{"!  vim foo.go", true, "vim foo.go"},
		{"ls -la", false, ""},
		{"!ls", false, ""},
		{"!", false, ""},
		{"!!", false, ""},
	}
	for _, tt := range tests {
		native, cmd := stripNativePrefix(tt.input)
		if native != tt.wantNative || cmd != tt.wantCmd {
			t.Errorf("stripNativePrefix(%q) = (%v, %q), want (%v, %q)",
				tt.input, native, cmd, tt.wantNative, tt.wantCmd)
		}
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func keyCode(name string) rune {
	switch name {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "right":
		return tea.KeyRight
	case "left":
		return tea.KeyLeft
	case "home":
		return tea.KeyHome
	case "end":
		return tea.KeyEnd
	}
	return tea.KeyExtended
}
