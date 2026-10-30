package ui

import (
	"fmt"
	"sort"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/help"
)

func (m *Model) enterTldrMenu() tea.Cmd {
	pages := help.ListTldrPages()
	if len(pages) == 0 {
		m.status = "downloading tldr pages..."
		return func() tea.Msg {
			help.EnsureTldr()
			return tldrReady{}
		}
	}
	sort.Strings(pages)
	var items []list.Item
	for _, p := range pages {
		p := p
		items = append(items, menuItem{
			title: p,
			desc:  "TLDR page",
			action: func(m *Model) tea.Cmd {
				return m.showTldr(p)
			},
		})
	}
	m.pushFilterableMenu("TLDR pages", items)
	return nil
}

type tldrReady struct{}

func (m *Model) enterManMenu() tea.Cmd {
	pages := help.ListManPages()
	sort.Strings(pages)
	var items []list.Item
	for _, p := range pages {
		p := p
		items = append(items, menuItem{
			title: p,
			desc:  "Man page",
			action: func(m *Model) tea.Cmd {
				return m.showMan(p)
			},
		})
	}
	if len(items) == 0 {
		m.status = "no man pages found"
		return nil
	}
	m.pushFilterableMenu("Man pages", items)
	return nil
}

func (m *Model) showTldr(command string) tea.Cmd {
	content, err := help.GetTldrPage(command)
	if err != nil {
		m.status = fmt.Sprintf("tldr: %v", err)
		return nil
	}
	m.showHelpPage(command, content)
	return nil
}

func (m *Model) showMan(command string) tea.Cmd {
	path, ok := help.FindManPage(command)
	if !ok {
		m.status = fmt.Sprintf("man page for '%s' not found", command)
		return nil
	}
	content, err := help.ParseManPage(path)
	if err != nil {
		m.status = fmt.Sprintf("man: %v", err)
		return nil
	}
	m.showHelpPage(command, content)
	return nil
}

func (m *Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.helpAsk.prompting {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.PasteMsg:
			return m.updateHelpPrompt(msg)
		}
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "backspace", "q":
			m.screen = screenMenu
			m.resize()
			return m, nil
		case "a":
			return m, m.startHelpAsk()
		}
	}
	var cmd tea.Cmd
	m.helpView, cmd = m.helpView.Update(msg)
	return m, cmd
}
