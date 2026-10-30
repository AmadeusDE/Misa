package aichat

import tea "charm.land/bubbletea/v2"

type standalone struct{ chat *Chat }

func (s standalone) Init() tea.Cmd { return s.chat.Init() }

func (s standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := s.chat.Update(msg)
	if s.chat.IsQuit() {
		return s, tea.Quit
	}
	return s, cmd
}

func (s standalone) View() tea.View {
	v := tea.NewView(s.chat.View())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// Run shows the chat on the whole terminal until the user quits.
func Run(opt Options) error {
	chat, err := New(opt)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(standalone{chat}).Run()
	return err
}
