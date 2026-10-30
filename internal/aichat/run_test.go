package aichat

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestStandaloneQuitsAndUsesTheWholeScreen(t *testing.T) {
	r := newRig(t)
	s := standalone{r.chat}
	v := s.View()
	if !v.AltScreen || v.MouseMode != tea.MouseModeCellMotion {
		t.Errorf("view = alt %v mouse %v", v.AltScreen, v.MouseMode)
	}
	_, cmd := s.Update(tea.KeyPressMsg{Code: 'x'})
	if cmd != nil {
		t.Error("a letter produced a command")
	}
	_, cmd = s.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("not a quit command")
	}
}
