package aichat

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// entries are the sidebar rows: "new chat", then the saved conversations.
func (m *Chat) entries() int { return len(m.metas) + 1 }

func (m *Chat) sidebarRows(w, h int) []string {
	m.sideScroll = max(min(m.sideScroll, m.entries()-h), 0)
	if m.sideCursor < m.sideScroll {
		m.sideScroll = m.sideCursor
	}
	if m.sideCursor >= m.sideScroll+h {
		m.sideScroll = m.sideCursor - h + 1
	}
	rows := make([]string, h)
	for i := range rows {
		idx := m.sideScroll + i
		rows[i] = fit("", w)
		if idx >= m.entries() {
			continue
		}
		title, current := "+ new chat", !m.saved && idx == 0
		if idx > 0 {
			meta := m.metas[idx-1]
			title, current = meta.Title, m.saved && meta.ID == m.conv.ID
		}
		mark := "  "
		if current {
			mark = "▌ "
		}
		line := fit(ansi.Truncate(mark+title, w, "…"), w)
		switch {
		case m.focus == focusSidebar && idx == m.sideCursor:
			line = lipgloss.NewStyle().Reverse(true).Render(line)
		case current:
			line = accent.Render(line)
		case idx == 0:
			line = dim.Render(line)
		}
		rows[i] = line
	}
	return rows
}
