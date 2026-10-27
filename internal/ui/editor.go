package ui

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/vi"
)

func (m *Model) enterEditor(filename string) {
	m.viEditor = vi.New(filename, m.dir)
	m.viEditor.Resize(m.width, m.height)
	if len(m.viRegister) > 0 {
		m.viEditor.SetRegister(m.viRegister, m.viRegLine)
	}
	m.editorReturn = m.screen
	m.screen = screenEditor
}

func (m *Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.viEditor.Update(msg)
	if m.viEditor.Quit {
		m.viRegister, m.viRegLine = m.viEditor.Register()
		m.screen = m.editorReturn
		m.viEditor = nil
		m.resize()
		switch m.screen {
		case screenFileBrowser:
			return m, m.reloadFileList()
		case screenShell:
			m.updateFullShellVP()
		default:
			m.updateShellView()
		}
		return m, nil
	}
	return m, cmd
}

func (m *Model) editorView() string {
	return m.viEditor.View()
}

func (m *Model) openInEditor(name string) tea.Cmd {
	if name == "" {
		m.enterEditor("")
		return nil
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.dir, path)
	}
	m.enterEditor(path)
	return nil
}
