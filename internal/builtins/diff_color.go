package builtins

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func diffColor(output string, o diffOptions) string {
	if o.color != "always" || o.style == "ed" || o.style == "rcs" || o.style == "side" {
		return output
	}
	red := lipgloss.NewStyle().Foreground(lipgloss.Red).TabWidth(lipgloss.NoTabConversion)
	green := lipgloss.NewStyle().Foreground(lipgloss.Green).TabWidth(lipgloss.NoTabConversion)
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Cyan).TabWidth(lipgloss.NoTabConversion)
	bold := lipgloss.NewStyle().Bold(true).TabWidth(lipgloss.NoTabConversion)
	lines := strings.SplitAfter(output, "\n")
	contextOld := false
	for i, line := range lines {
		text := strings.TrimSuffix(line, "\n")
		if text == "" {
			continue
		}
		var style *lipgloss.Style
		switch o.style {
		case "unified":
			switch {
			case i < 2:
				style = &bold
			case strings.HasPrefix(text, "@@"):
				style = &cyan
			case text[0] == '-':
				style = &red
			case text[0] == '+':
				style = &green
			}
		case "context":
			switch {
			case i < 2:
				style = &bold
			case strings.HasPrefix(text, "***************"):
			case strings.HasPrefix(text, "*** "):
				style = &cyan
				contextOld = true
			case strings.HasPrefix(text, "--- "):
				style = &cyan
				contextOld = false
			case strings.HasPrefix(text, "\\ No newline"):
			default:
				if contextOld {
					style = &red
				} else {
					style = &green
				}
			}
		default:
			switch {
			case text[0] >= '0' && text[0] <= '9':
				style = &cyan
			case text[0] == '<':
				style = &red
			case text[0] == '>':
				style = &green
			}
		}
		if style != nil {
			lines[i] = strings.TrimSuffix(style.Render(text), ansi.ResetStyle) + ansi.NewStyle(0).String() + strings.TrimPrefix(line, text)
		}
	}
	return strings.Join(lines, "")
}
