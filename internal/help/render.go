package help

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	h1Style   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	h2Style   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	h3Style   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	codeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	boldStyle = lipgloss.NewStyle().Bold(true)
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	bulletSty = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

var (
	reBold   = regexp.MustCompile(`\*\*(.*?)\*\*`)
	reItalic = regexp.MustCompile(`(?:^|[^*])\*([^*]+)\*(?:[^*]|$)`)
	reCode   = regexp.MustCompile("`([^`]+)`")
)

func RenderMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	var out []string
	inCode := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			if inCode {
				out = append(out, dimStyle.Render("────────────────────────"))
			} else {
				out = append(out, dimStyle.Render("────────────────────────"))
			}
			continue
		}

		if inCode {
			out = append(out, codeStyle.Render(line))
			continue
		}

		if strings.HasPrefix(line, "# ") {
			out = append(out, h1Style.Render(line[2:]))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			out = append(out, h2Style.Render(line[3:]))
			continue
		}
		if strings.HasPrefix(line, "### ") {
			out = append(out, h3Style.Render(line[4:]))
			continue
		}

		if strings.HasPrefix(line, "> ") {
			out = append(out, dimStyle.Render("│ "+line[2:]))
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			content := applyInline(trimmed[2:])
			out = append(out, "  "+bulletSty.Render("•")+" "+content)
			continue
		}

		out = append(out, applyInline(line))
	}

	return strings.Join(out, "\n")
}

func applyInline(s string) string {
	s = reCode.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1]
		return codeStyle.Render(inner)
	})
	s = reBold.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-2]
		return boldStyle.Render(inner)
	})
	return s
}
