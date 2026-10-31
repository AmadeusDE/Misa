package ui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/builtins"
)

func cardGameMenuItems() []list.Item {
	var items []list.Item
	for _, entry := range []struct{ name, title, description string }{
		{"solitaire", "Solitaire", "Klondike, draw one"},
		{"freecell", "FreeCell", "Eight columns and four free cells"},
		{"spider", "Spider Solitaire", "One-suit Spider; --suits 2 or 4 in the shell"},
		{"hearts", "Hearts", "Play three computer opponents to 100"},
	} {
		items = append(items, menuItem{title: entry.title, desc: entry.description, action: func(m *Model) tea.Cmd {
			g, err := builtins.CardGame(entry.name, nil)
			if err != nil {
				m.status = err.Error()
				return nil
			}
			m.enterProcMon(g)
			return g.Init()
		}})
	}
	return items
}
