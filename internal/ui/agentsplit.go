package ui

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) focusedDir() string {
	if p := m.activePane(); p != nil && p.misa != nil {
		return p.misa.dir
	}
	return m.dir
}

// addAgentPane opens a split running ugh in dir. The split closes when ugh
// quits; closing the split detaches the session.
func (m *Model) addAgentPane(dir string) tea.Cmd {
	if startAgent == nil {
		return nil
	}
	open := m.addMuxPane()
	nested := m.activePane().misa
	nested.dir = dir
	_, start := nested.startAgentSession(nil)
	if nested.session != nil {
		nested.session.split = true
	}
	return tea.Batch(open, routeTo(nested, start))
}

// detachSession keeps a closing split's session running without a pane.
func (m *Model) detachSession(closing *Model) {
	if s := closing.session; s != nil && s.split {
		m.detached = append(m.detached, s)
		closing.session = nil
	}
}

// reattachAgent shows the most recently detached session in a new split.
func (m *Model) reattachAgent() tea.Cmd {
	if len(m.detached) == 0 {
		return nil
	}
	s := m.detached[len(m.detached)-1]
	m.detached = m.detached[:len(m.detached)-1]
	open := m.addMuxPane()
	nested := m.activePane().misa
	nested.session = s
	s.size = tea.WindowSizeMsg{}
	nested.enterFullShell()
	return open
}

func (m *Model) closeDetached() {
	for _, s := range m.detached {
		s.close()
	}
	m.detached = nil
}

// agentMsgOf unwraps a session's mail or command result, whether it comes
// straight from a detached session or tagged by the pane that held it.
func agentMsgOf(msg tea.Msg) (a agent, inner tea.Msg, mail, ok bool) {
	if p, isPane := msg.(paneMsg); isPane {
		msg = p.msg
	}
	switch msg := msg.(type) {
	case agentMail:
		return msg.a, msg.msg, true, true
	case agentResult:
		return msg.a, msg.msg, false, true
	}
	return nil, nil, false, false
}

// routeAgentMsg delivers to wherever the session lives now: sessions move
// between panes and the detached list while their messages are in flight.
func (m *Model) routeAgentMsg(a agent, msg tea.Msg, mail bool) (tea.Model, tea.Cmd) {
	for _, p := range m.allMuxPanes() {
		if s := paneSession(p); s != nil && s.a == a {
			_, cmd := p.misa.updateAgentMsg(a, msg, mail)
			cmd = routeTo(p.misa, cmd)
			if p.misa.wantsClose {
				cmd = tea.Batch(cmd, m.closePane(p))
			}
			return m, cmd
		}
	}
	for i, s := range m.detached {
		if s.a == a {
			cmd, exited, _ := s.handle(msg, mail)
			if exited {
				s.close()
				m.detached = slices.Delete(m.detached, i, i+1)
			}
			return m, cmd
		}
	}
	return m, nil
}

// closePane closes p, keeping focus where it was. A pane on a hidden tag is
// closed there without changing what is shown.
func (m *Model) closePane(p *muxPane) tea.Cmd {
	i := slices.Index(m.mux.panes, p)
	if i < 0 {
		for tag, t := range m.mux.tags {
			if tag != m.mux.activeTag && t != nil && slices.Contains(t.panes, p) {
				return m.onHiddenTag(tag, func() tea.Cmd { return m.closePane(p) })
			}
		}
		return nil
	}
	focused := m.activePane()
	m.mux.activeIdx = i
	cmd := m.closeMuxPane()
	if j := slices.Index(m.mux.panes, focused); j >= 0 {
		m.mux.activeIdx = j
	}
	return cmd
}

// onHiddenTag runs fn with tag's panes in place of the shown ones, then puts
// both back, leaving the shown tag's focus, layout and resize mode as they were.
func (m *Model) onHiddenTag(tag int, fn func() tea.Cmd) tea.Cmd {
	shown, resizing := m.mux.activeTag, m.mux.resizeMode
	m.mux.tags[shown] = &muxTag{panes: m.mux.panes, activeIdx: m.mux.activeIdx, layout: m.mux.layout, tree: m.mux.tree}
	load := func(i int) {
		t := m.mux.tags[i]
		m.mux.activeTag, m.mux.panes, m.mux.activeIdx, m.mux.layout, m.mux.tree = i, t.panes, t.activeIdx, t.layout, t.tree
	}
	load(tag)
	cmd := fn()
	m.mux.tags[tag] = &muxTag{panes: m.mux.panes, activeIdx: m.mux.activeIdx, layout: m.mux.layout, tree: m.mux.tree}
	load(shown)
	m.mux.resizeMode = resizing
	m.resize()
	return cmd
}
