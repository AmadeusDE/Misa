package imgview

import tea "charm.land/bubbletea/v2"

type standalone struct{ viewer *Viewer }

func (s *standalone) Init() tea.Cmd { return s.viewer.Init() }
func (s *standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := s.viewer.Update(msg)
	if s.viewer.IsQuit() {
		return s, tea.Quit
	}
	return s, cmd
}
func (s *standalone) View() tea.View {
	v := tea.NewView(s.viewer.View())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// Run displays a viewer in a standalone terminal.
func Run(v *Viewer) error { _, err := tea.NewProgram(&standalone{viewer: v}).Run(); return err }
