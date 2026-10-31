package vi

import tea "charm.land/bubbletea/v2"

type standaloneModel struct {
	editor *Editor
}

func (m *standaloneModel) Init() tea.Cmd { return nil }

func (m *standaloneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.editor.Update(msg)
	if m.editor.Quit {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *standaloneModel) View() tea.View {
	v := tea.NewView(m.editor.View())
	v.AltScreen = true
	return v
}

func RunStandalone(filename, dir string) error {
	return runStandalone(filename, dir, nil)
}

func newStandaloneEditor(filename, dir string, env []string) *Editor {
	ed := New(filename, dir)
	if env != nil {
		ed.shellEnv = append([]string{}, env...)
	}
	return ed
}

func runStandalone(filename, dir string, env []string) error {
	ed := newStandaloneEditor(filename, dir, env)
	p := tea.NewProgram(&standaloneModel{editor: ed})
	_, err := p.Run()
	return err
}
