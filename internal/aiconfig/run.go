package aiconfig

import tea "charm.land/bubbletea/v2"

type standalone struct{ m *Model }

func (s *standalone) Init() tea.Cmd { return s.m.Init() }

func (s *standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := s.m.Update(msg)
	if s.m.IsQuit() {
		return s, tea.Quit
	}
	return s, cmd
}

func (s *standalone) View() tea.View {
	v := tea.NewView(s.m.View())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// Run shows the configurator on the whole terminal until the user quits.
func Run() error {
	_, err := tea.NewProgram(&standalone{m: New()}).Run()
	return err
}
