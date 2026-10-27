package ui

import (
	"reflect"

	tea "charm.land/bubbletea/v2"
)

// paneMsg carries a message produced by a nested pane's command back to that
// pane, so results land where they were started even after focus moved or
// the pane's tag was hidden.
type paneMsg struct {
	owner *Model
	msg   tea.Msg
}

const teaPkg = "charm.land/bubbletea/v2"

// routeTo tags cmd's eventual messages with owner.
func routeTo(owner *Model, cmd tea.Cmd) tea.Cmd {
	return tagCmd(cmd, func(msg tea.Msg) tea.Msg { return paneMsg{owner: owner, msg: msg} })
}

// tagCmd wraps cmd's eventual messages with tag. Bubble Tea's own messages
// (quit, clipboard, colour queries, runtime internals) pass through for the
// program to act on; batches are tagged command by command.
func tagCmd(cmd tea.Cmd, tag func(tea.Msg) tea.Msg) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		switch msg := msg.(type) {
		case nil:
			return nil
		case tea.BatchMsg:
			tagged := make(tea.BatchMsg, len(msg))
			for i, c := range msg {
				tagged[i] = tagCmd(c, tag)
			}
			return tagged
		}
		if reflect.TypeOf(msg).PkgPath() == teaPkg {
			return msg
		}
		return tag(msg)
	}
}

func (m *Model) paneOf(owner *Model) *muxPane {
	for _, p := range m.allMuxPanes() {
		if p.misa == owner {
			return p
		}
	}
	return nil
}

func (m *Model) updateOwnedPane(msg paneMsg) (tea.Model, tea.Cmd) {
	p := m.paneOf(msg.owner)
	if p == nil {
		return m, nil
	}
	_, cmd := p.misa.Update(msg.msg)
	if p.misa.wantsClose && p == m.activePane() {
		return m, m.closeMuxPane()
	}
	return m, routeTo(p.misa, cmd)
}
