package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func fullShellModel(t *testing.T) *Model {
	t.Helper()
	m := New(t.TempDir())
	m.nestedMode = true
	m.width, m.height = 80, 24
	m.enterFullShell()
	t.Cleanup(m.closeTerminals)
	return m
}

func TestPasteReachesRunningProgram(t *testing.T) {
	m := fullShellModel(t)
	cmd := m.execArgv("sh", "-c", "read x; echo got $x")
	m.Update(tea.PasteMsg{Content: "hi\r"})
	pumpUntil(t, m, cmd, "exit", func() bool { return m.terminal == nil && bufferHas(m, "got hi") })
}

func TestPasteFillsPrompt(t *testing.T) {
	m := fullShellModel(t)
	m.Update(tea.PasteMsg{Content: "echo hi"})
	if got := m.cmdInput.Value(); got != "echo hi" {
		t.Errorf("prompt = %q, want the pasted text", got)
	}
}
