package aichat

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Keys avoid Misa's multiplexer: ctrl+w is its prefix and alt+1-9, alt+h/j/k/l and alt+arrows move between
// panes, so none of those are used here. ctrl+c is Misa's too (it closes the pane) and never reaches this screen
// inside Misa; ctrl+q quits it.
func (m *Chat) key(msg tea.KeyPressMsg) tea.Cmd {
	switch m.mode {
	case modePicker:
		return m.pickerKey(msg)
	case modeSystem:
		return m.systemKey(msg)
	case modeRename:
		return m.renameKey(msg)
	}
	k := msg.String()
	if m.confirmDel != "" {
		if k == "y" {
			m.deleteConversation(m.confirmDel)
		}
		m.confirmDel = ""
		return nil
	}
	if cmd, handled := m.globalKey(k); handled {
		return cmd
	}
	if m.focus == focusSidebar {
		m.sidebarKey(k)
		return nil
	}
	return m.inputKey(msg)
}

func (m *Chat) globalKey(k string) (tea.Cmd, bool) {
	switch k {
	case "ctrl+q":
		m.quit = true
		m.stop()
	case "ctrl+c":
		if m.job != nil {
			m.stop()
		} else {
			m.quit = true
		}
	case "ctrl+d":
		if m.focus != focusInput || m.input.Value() != "" {
			return nil, false
		}
		m.quit = true
	case "ctrl+b":
		m.showSidebar = !m.showSidebar
		m.focus = focusInput
		m.layout()
		m.invalidate()
		if m.w < sidebarMinWidth {
			m.say("the terminal is too narrow for the conversation list", true)
		}
	case "ctrl+n":
		m.newConversation()
	case "ctrl+o":
		return m.openPicker(), true
	case "ctrl+s":
		m.system.SetValue(m.conv.System)
		m.system.MoveToEnd()
		m.mode = modeSystem
	case "ctrl+g":
		m.toggleSearch()
	case "ctrl+r":
		return m.regenerate(), true
	case "ctrl+y":
		return m.copyLast(), true
	case "ctrl+t":
		m.showThinking = !m.showThinking
		m.invalidate()
		m.renderStreamNow()
	case "pgup":
		m.scrollBy(max(m.bodyHeight()-3, 1))
	case "pgdown":
		m.scrollBy(-max(m.bodyHeight()-3, 1))
	case "tab":
		if m.sideW() == 0 {
			return nil, false
		}
		m.toggleFocus()
	case "esc":
		switch {
		case m.job != nil:
			m.stop()
		case m.editing >= 0:
			m.cancelEdit()
		case m.focus == focusSidebar:
			m.focus = focusInput
		default:
			return nil, false
		}
	default:
		return nil, false
	}
	return nil, true
}

func (m *Chat) toggleFocus() {
	if m.focus == focusInput {
		m.focus = focusSidebar
		m.sideCursor = 0
		for i, meta := range m.metas {
			if m.saved && meta.ID == m.conv.ID {
				m.sideCursor = i + 1
			}
		}
		return
	}
	m.focus = focusInput
}

func (m *Chat) inputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		return m.send(m.input.Value())
	case "up":
		if m.input.Value() == "" && m.job == nil {
			m.editLast()
			return nil
		}
	}
	m.input, _ = m.input.Update(msg)
	return nil
}

func (m *Chat) sidebarKey(k string) {
	switch k {
	case "up", "k":
		m.sideCursor = max(m.sideCursor-1, 0)
	case "down", "j":
		m.sideCursor = min(m.sideCursor+1, m.entries()-1)
	case "home":
		m.sideCursor = 0
	case "end":
		m.sideCursor = m.entries() - 1
	case "enter":
		m.activate(m.sideCursor)
	case "n":
		m.newConversation()
	case "r":
		if id := m.sideID(); id != "" {
			m.beginRename(id)
		}
	case "d", "delete":
		m.confirmDel = m.sideID()
	}
}

// sideID is the conversation under the sidebar cursor, empty on "new chat".
func (m *Chat) sideID() string {
	if m.sideCursor == 0 || m.sideCursor > len(m.metas) {
		return ""
	}
	return m.metas[m.sideCursor-1].ID
}

// activate opens a sidebar entry.
func (m *Chat) activate(idx int) {
	m.sideCursor = idx
	if idx == 0 {
		m.newConversation()
		return
	}
	m.openConversation(m.metas[idx-1].ID)
}

func (m *Chat) beginRename(id string) {
	m.renameID = id
	m.rename = ""
	for _, meta := range m.metas {
		if meta.ID == id {
			m.rename = meta.Title
		}
	}
	m.mode = modeRename
}

func (m *Chat) renameKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "esc":
		m.mode = modeChat
	case "enter":
		m.mode = modeChat
		m.renameByID(m.renameID, m.rename)
	case "backspace":
		if r := []rune(m.rename); len(r) > 0 {
			m.rename = string(r[:len(r)-1])
		}
	case "ctrl+u":
		m.rename = ""
	default:
		m.rename += strings.ReplaceAll(msg.Text, "\n", " ")
	}
	return nil
}

func (m *Chat) systemKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = modeChat
	case "enter":
		m.conv.System = strings.TrimSpace(m.system.Value())
		m.mode = modeChat
		m.persist()
		m.say("system prompt saved", false)
	default:
		m.system, _ = m.system.Update(msg)
	}
	return nil
}

func (m *Chat) paste(msg tea.PasteMsg) tea.Cmd {
	switch m.mode {
	case modeChat:
		if m.focus == focusInput {
			m.input, _ = m.input.Update(msg)
		}
	case modeSystem:
		m.system, _ = m.system.Update(msg)
	case modeRename:
		m.rename += strings.ReplaceAll(msg.Content, "\n", " ")
	case modePicker:
		m.picker.filter += strings.TrimSpace(strings.ReplaceAll(msg.Content, "\n", " "))
		m.picker.cursor = 0
	}
	return nil
}

func (m *Chat) scrollBy(lines int) {
	m.scroll = max(min(m.scroll+lines, m.maxScroll()), 0)
}

func (m *Chat) maxScroll() int {
	room := max(m.bodyHeight()-1-m.input.Height(), 1)
	return max(len(m.transcript())-room, 0)
}
