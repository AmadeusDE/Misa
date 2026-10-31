package cardui

import tea "charm.land/bubbletea/v2"

type standalone struct{ game *Game }

func (s *standalone) Init() tea.Cmd { return s.game.Init() }
func (s *standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := s.game.Update(msg)
	if s.game.IsQuit() {
		return s, tea.Quit
	}
	return s, cmd
}
func (s *standalone) View() tea.View {
	v := tea.NewView(s.game.View())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
func Run(g *Game) error { defer g.Close(); _, err := tea.NewProgram(&standalone{g}).Run(); return err }
