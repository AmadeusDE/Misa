package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/procmon"
)

// newAIConfig builds the AI provider configurator, or is nil in builds without ugh (-tags nough).
var newAIConfig func() procmon.Viewer

const noAIMessage = "built without AI support (-tags nough)"

// enterAIConfig opens the configurator as an in-process screen.
func (m *Model) enterAIConfig() tea.Cmd {
	if newAIConfig == nil {
		m.status = "aiconfig: " + noAIMessage
		return nil
	}
	m.enterProcMon(newAIConfig())
	return m.procViewer.Init()
}

// aiConfigCommand runs the `aiconfig` command typed at the prompt.
func (m *Model) aiConfigCommand() (bool, tea.Cmd) {
	m.cmdInput.SetValue("")
	if newAIConfig == nil {
		m.shellBuffer = append(m.shellBuffer, "aiconfig: "+noAIMessage)
		m.updateShellView()
		return true, nil
	}
	return true, m.enterAIConfig()
}
