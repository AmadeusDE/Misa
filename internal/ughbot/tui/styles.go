package tui

import "charm.land/lipgloss/v2"

var (
	accent = lipgloss.Color("39")
	good   = lipgloss.Color("42")
	warn   = lipgloss.Color("214")
	bad    = lipgloss.Color("203")
	subtle = lipgloss.Color("244")
	faint  = lipgloss.Color("240")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	tabStyle      = lipgloss.NewStyle().Padding(0, 1).Foreground(subtle)
	tabActive     = lipgloss.NewStyle().Padding(0, 1).Bold(true).Reverse(true).Foreground(accent)
	hintStyle     = lipgloss.NewStyle().Foreground(faint)
	goodStyle     = lipgloss.NewStyle().Foreground(good)
	errStyle      = lipgloss.NewStyle().Foreground(bad)
	warnStyle     = lipgloss.NewStyle().Bold(true).Foreground(warn)
	dimStyle      = lipgloss.NewStyle().Foreground(subtle)
	userStyle     = lipgloss.NewStyle().Bold(true).Foreground(accent)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	headStyle     = lipgloss.NewStyle().Bold(true).Foreground(subtle)
)

func stateStyle(state string) lipgloss.Style {
	switch state {
	case "running", "idle", "ready", "ok":
		return lipgloss.NewStyle().Foreground(good)
	case "error", "failed", "stopped":
		return lipgloss.NewStyle().Foreground(bad)
	}
	return lipgloss.NewStyle().Foreground(warn)
}
