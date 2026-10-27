package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/term"
)

// requestHostColors asks the real terminal for its default colors, which
// programs running in panes query with OSC 10/11.
func requestHostColors() tea.Cmd {
	return tea.Batch(tea.RequestForegroundColor, tea.RequestBackgroundColor)
}

// recordHostColor stores a color reply for the panes and reports whether msg
// was one.
func recordHostColor(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.ForegroundColorMsg:
		term.SetHostForeground(msg.Color)
	case tea.BackgroundColorMsg:
		term.SetHostBackground(msg.Color)
	default:
		return false
	}
	return true
}
