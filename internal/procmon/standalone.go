package procmon

import tea "charm.land/bubbletea/v2"

// fullScreen frames a viewer run as its own program: the whole terminal, on
// the alternate screen.
func fullScreen(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
