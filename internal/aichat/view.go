package aichat

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// sideW is the sidebar's width, 0 when it is hidden or the terminal is too narrow.
func (m *Chat) sideW() int {
	if m.showSidebar && m.w >= sidebarMinWidth {
		return sidebarWidth
	}
	return 0
}

func (m *Chat) mainWidth() int {
	if sw := m.sideW(); sw > 0 {
		return m.w - sw - 1
	}
	return m.w
}

func (m *Chat) bodyHeight() int { return max(m.h-2, 1) }

// layout sizes the text inputs for the current window.
func (m *Chat) layout() {
	m.input.SetWidth(m.mainWidth())
	m.system.SetWidth(m.mainWidth())
}

// View draws the header, the sidebar beside the main area, and the footer, exactly w columns by h rows.
func (m *Chat) View() string {
	rows := []string{fit(m.header(), m.w)}
	body := m.bodyHeight()
	main := m.mainRows(body)
	if sw := m.sideW(); sw > 0 {
		side := m.sidebarRows(sw, body)
		sep := dim.Render("│")
		for i := range body {
			rows = append(rows, side[i]+sep+main[i])
		}
	} else {
		rows = append(rows, main...)
	}
	rows = append(rows, fit(m.footer(), m.w))
	return strings.Join(rows[:min(len(rows), m.h)], "\n")
}

func (m *Chat) header() string {
	model := m.conv.Model
	if model == "" {
		if ref, err := m.opt.ModelFor(); err == nil {
			model = ref
		} else {
			model = "no model: run aiconfig"
		}
	}
	parts := []string{accent.Render("misa chat"), m.conv.Title, dim.Render(model)}
	if m.conv.Title == "" {
		parts[1] = dim.Render("new chat")
	}
	if m.conv.Search {
		parts = append(parts, accent.Render("web"))
	}
	if u := m.conv.Usage; u.In+u.Out > 0 {
		use := fmt.Sprintf("%s in %s out", tokens(u.In), tokens(u.Out))
		if u.Cost > 0 {
			use += fmt.Sprintf(" $%.4f", u.Cost)
		}
		parts = append(parts, dim.Render(use))
	}
	return strings.Join(parts, dim.Render("  ·  "))
}

func tokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprint(n)
}

func (m *Chat) footer() string {
	switch {
	case m.confirmDel != "":
		return errStyle.Render("delete this conversation? y/n")
	case m.notice != "" && m.noticeErr:
		return errStyle.Render(m.notice)
	case m.notice != "":
		return accent.Render(m.notice)
	case m.mode == modePicker:
		return dim.Render("type to filter · enter choose · esc back")
	case m.mode == modeSystem:
		return dim.Render("enter save · alt+enter newline · esc cancel")
	case m.mode == modeRename:
		return dim.Render("enter save · esc cancel")
	case m.job != nil:
		return dim.Render("answering… esc stops")
	case m.focus == focusSidebar:
		return dim.Render("↑↓ move · enter open · n new · r rename · d delete · tab back")
	}
	return dim.Render("enter send · alt+enter newline · ctrl+o model · ctrl+g web · ctrl+s system · ctrl+n new · ctrl+q quit")
}

func (m *Chat) mainRows(h int) []string {
	w := m.mainWidth()
	switch m.mode {
	case modePicker:
		return m.pickerRows(w, h)
	case modeSystem:
		return m.systemRows(w, h)
	case modeRename:
		return pad([]string{accent.Render("rename conversation"), "", m.rename + "█"}, w, h)
	}
	inputRows := strings.Split(m.input.View(), "\n")
	room := max(h-1-len(inputRows), 1)
	lines := m.transcript()
	if len(lines) == 0 {
		lines = m.welcome()
	}
	m.scroll = min(m.scroll, max(len(lines)-room, 0))
	end := len(lines) - m.scroll
	start := max(end-room, 0)
	window := lines[start:end]
	rows := make([]string, 0, h)
	for range room - len(window) {
		rows = append(rows, fit("", w))
	}
	for _, l := range window {
		rows = append(rows, fit(l, w))
	}
	rows = append(rows, dim.Render(strings.Repeat("─", w)))
	for _, l := range inputRows {
		rows = append(rows, fit(l, w))
	}
	return rows[len(rows)-h:]
}

func (m *Chat) welcome() []string {
	return []string{
		accent.Render("Chat with a model"),
		dim.Render("Type a message and press enter. ctrl+o picks a model, ctrl+g lets it search the web."),
		dim.Render("Configure providers with `aiconfig`."),
	}
}

func (m *Chat) systemRows(w, h int) []string {
	rows := []string{accent.Render("system prompt for this conversation"), dim.Render("sent with every message; empty means none"), ""}
	for _, l := range strings.Split(m.system.View(), "\n") {
		rows = append(rows, l)
	}
	return pad(rows, w, h)
}

// pad fits rows to exactly h rows of w columns.
func pad(rows []string, w, h int) []string {
	out := make([]string, h)
	for i := range out {
		if i < len(rows) {
			out[i] = fit(rows[i], w)
		} else {
			out[i] = fit("", w)
		}
	}
	return out
}

// fit cuts or pads s to w columns.
func fit(s string, w int) string {
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "")
	}
	if gap := w - lipgloss.Width(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}
