package aiconfig

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	titleSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f")).Bold(true)
	headSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374")).Bold(true)
	textSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	dimSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	okSt     = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	warnSt   = lipgloss.NewStyle().Foreground(lipgloss.Color("#fe8019"))
	errSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true)
	tagSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598"))
	selSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#fabd2f")).Bold(true)
	labelSt  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8ec07c"))
	cursorSt = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f")).Bold(true)
)

// clip cuts s to width columns.
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// cell fits s into exactly width columns, padded on the right.
func cell(s string, width int) string {
	s = clip(s, width)
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}
