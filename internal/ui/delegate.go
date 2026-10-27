package ui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type compactDelegate struct{}

var (
	compactNormalTitleStyle = lipgloss.NewStyle().PaddingLeft(2)
	compactNormalDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#665c54"))
	compactSelectedStyle    = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(lipgloss.Color("#d3869b")).
				Bold(true)
)

func (d compactDelegate) Height() int                             { return 1 }
func (d compactDelegate) Spacing() int                            { return 0 }
func (d compactDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	type titled interface {
		Title() string
		Description() string
	}
	it, ok := listItem.(titled)
	if !ok {
		return
	}
	title, desc := it.Title(), it.Description()

	avail := m.Width() - 2
	if avail < 0 {
		avail = 0
	}

	if index == m.Index() {
		line := title
		if desc != "" {
			line = title + " - " + desc
		}
		fmt.Fprint(w, compactSelectedStyle.Render("> "+truncate(line, avail)))
		return
	}

	titleOut := truncate(title, avail)
	remaining := avail - len([]rune(titleOut))
	descOut := ""
	if desc != "" && remaining > 3 {
		descOut = truncate(" - "+desc, remaining)
	}
	fmt.Fprint(w, compactNormalTitleStyle.Render("  "+titleOut)+compactNormalDescStyle.Render(descOut))
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "..."
	}
	return string(r[:width-1]) + "..."
}
