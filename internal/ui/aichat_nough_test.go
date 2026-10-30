//go:build nough

package ui

import (
	"strings"
	"testing"
)

func TestAIChatSaysItIsNotBuiltIn(t *testing.T) {
	m := fullShellModel(t)
	typeCommand(m, "aichat")
	if m.screen == screenProcMon {
		t.Fatal("aichat opened without AI support")
	}
	if last := m.shellBuffer[len(m.shellBuffer)-1]; !strings.Contains(last, "built without AI support (-tags nough)") {
		t.Errorf("last line %q", last)
	}
	if got := aiChatItems(); got != nil {
		t.Errorf("menu entries without AI: %v", got)
	}
}
