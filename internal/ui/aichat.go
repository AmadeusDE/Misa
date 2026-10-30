package ui

import (
	"errors"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/procmon"
)

// newAIChat builds the chat screen from `aichat` arguments. It stays nil in builds with -tags nough.
var newAIChat func(args []string) (procmon.Viewer, error)

var errNoAI = errors.New("built without AI support (-tags nough)")

// openAIChat takes the pane over with the chat screen, like `mines`.
func (m *Model) openAIChat(args []string) (tea.Cmd, error) {
	if newAIChat == nil {
		return nil, errNoAI
	}
	chat, err := newAIChat(args)
	if err != nil {
		return nil, err
	}
	m.enterProcMon(chat)
	return chat.Init(), nil
}

// aiChatItems is the main menu entry for the chat, absent when AI support is not built in.
func aiChatItems() []list.Item {
	if newAIChat == nil {
		return nil
	}
	return []list.Item{menuItem{
		title: "AI Chat",
		desc:  "Chat with a model from your configured providers",
		action: func(m *Model) tea.Cmd {
			cmd, err := m.openAIChat(nil)
			if err != nil {
				m.status = err.Error()
			}
			return cmd
		},
	}}
}
