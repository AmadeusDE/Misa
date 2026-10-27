package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/procmon"
)

func (m *Model) enterProcMon(v procmon.Viewer) {
	m.procViewer = v
	m.procMonReturn = m.screen
	m.screen = screenProcMon
	if m.hasSplit() {
		pw, ph := m.mux.primaryW, m.mux.primaryH
		v.Resize(pw, ph)
	} else {
		v.Resize(m.width, m.height)
	}
}

func (m *Model) updateProcMon(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouse, ok := msg.(mouseMsg); ok {
		msg = mouse.toTea()
	}
	cmd := m.procViewer.Update(msg)
	if m.procViewer.IsQuit() {
		m.screen = m.procMonReturn
		m.procViewer = nil
		m.resize()
		m.updateShellView()
		return m, nil
	}
	return m, cmd
}
