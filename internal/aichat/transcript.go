package aichat

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"charm.land/glamour/v2"
	glamouransi "charm.land/glamour/v2/ansi"
	glamourstyles "charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	colAccent = lipgloss.Color("214")
	colDim    = lipgloss.Color("244")
	colUser   = lipgloss.Color("109")
	colErr    = lipgloss.Color("167")

	dim       = lipgloss.NewStyle().Foreground(colDim)
	accent    = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	userStyle = lipgloss.NewStyle().Foreground(colUser).Bold(true)
	errStyle  = lipgloss.NewStyle().Foreground(colErr)
)

// block is the cached rendering of one message.
type block struct{ lines []string }

// contentWidth is the width of the transcript text.
func (m *Chat) contentWidth() int { return max(m.mainWidth()-1, 10) }

// transcript is every line of the conversation, oldest first.
func (m *Chat) transcript() []string {
	if len(m.blocks) != len(m.conv.Messages) {
		m.blocks = make([]block, len(m.conv.Messages))
	}
	var lines []string
	for i, msg := range m.conv.Messages {
		switch {
		case i == len(m.conv.Messages)-1 && m.job != nil:
			lines = append(lines, m.streamLines...)
		default:
			if m.blocks[i].lines == nil {
				m.blocks[i].lines = m.messageLines(msg, false)
			}
			lines = append(lines, m.blocks[i].lines...)
		}
		lines = append(lines, "")
	}
	return lines
}

func (m *Chat) messageLines(msg Message, streaming bool) []string {
	w := m.contentWidth()
	if msg.Role == "user" {
		return append([]string{userStyle.Render("you")}, wrap(msg.Text, w)...)
	}
	return m.assistantLines(msg, streaming)
}

func (m *Chat) assistantLines(msg Message, streaming bool) []string {
	w := m.contentWidth()
	head := accent.Render("assistant")
	if msg.Model != "" {
		head += dim.Render("  " + msg.Model)
	}
	lines := []string{head}
	switch {
	case msg.Thinking == "":
	case m.showThinking:
		lines = append(lines, dim.Render("▾ thinking"))
		for _, l := range wrap(strings.TrimSpace(msg.Thinking), w-2) {
			lines = append(lines, dim.Italic(true).Render("  "+l))
		}
	default:
		n := strings.Count(strings.TrimSpace(msg.Thinking), "\n") + 1
		lines = append(lines, dim.Render(fmt.Sprintf("▸ thinking (%d lines, ctrl+t shows it)", n)))
	}
	for _, note := range msg.Notes {
		lines = append(lines, dim.Render(ansi.Truncate("  · "+note, w, "…")))
	}
	switch {
	case msg.Text != "":
		lines = append(lines, m.markdown(msg.Text, w)...)
	case streaming && msg.Thinking == "" && len(msg.Notes) == 0:
		lines = append(lines, dim.Render("…"))
	}
	switch msg.Error {
	case "":
	case errStopped:
		lines = append(lines, dim.Render("[stopped]"))
	default:
		for _, l := range wrap("error: "+msg.Error, w) {
			lines = append(lines, errStyle.Render(l))
		}
	}
	return lines
}

// renderStreamNow re-renders the answer being written.
func (m *Chat) renderStreamNow() {
	if m.job == nil {
		m.streamLines = nil
		return
	}
	before := len(m.streamLines)
	m.streamLines = m.assistantLines(*m.last(), true)
	m.lastRender = time.Now()
	if m.scroll > 0 {
		m.scroll = max(m.scroll+len(m.streamLines)-before, 0)
	}
}

func wrap(text string, w int) []string {
	return strings.Split(ansi.Wrap(text, max(w, 1), ""), "\n")
}

// markdown renders text for a terminal; a renderer failure falls back to the plain text.
func (m *Chat) markdown(text string, w int) []string {
	if m.md == nil || m.mdWidth != w {
		r, err := glamour.NewTermRenderer(glamour.WithStyles(chatStyle()), glamour.WithWordWrap(w))
		if err != nil {
			return wrap(text, w)
		}
		m.md, m.mdWidth = r, w
	}
	out, err := m.md.Render(text)
	if err != nil {
		return wrap(text, w)
	}
	lines := strings.Split(strings.Trim(out, "\n"), "\n")
	for i, l := range lines {
		lines[i] = trailingPad.ReplaceAllString(l, "\x1b[m")
	}
	return lines
}

// trailingPad is the run of styled spaces glamour pads every line with.
var trailingPad = regexp.MustCompile(`(?:\x1b\[[0-9;]*m| )+$`)

// chatStyle is glamour's dark style without the document margins, so the answer fills the transcript.
func chatStyle() glamouransi.StyleConfig {
	style := glamourstyles.DarkStyleConfig
	none := uint(0)
	style.Document.Margin = &none
	style.Document.BlockPrefix, style.Document.BlockSuffix = "", ""
	return style
}
