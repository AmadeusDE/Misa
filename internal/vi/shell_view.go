package vi

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// handleShellEvent gates editing while a job or its output is active.
func (e *Editor) handleShellEvent(msg tea.Msg) bool {
	if result, ok := msg.(shellResultMsg); ok {
		e.finishShell(result)
		return true
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		e.Resize(size.Width, size.Height)
		return true
	}
	if e.shell != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok && typedKey(key) == "ctrl+c" {
			e.shell.cancel()
		}
		return true
	}
	if e.shellOutput != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			e.shellOutputKey(key)
		}
		return true
	}
	return false
}

// shellOutput is a modal, scrollable view, not terminal emulation.
type shellOutput struct {
	text string
	top  int
}

func (e *Editor) openShellFilter(first, last int) {
	e.mode = ModeCommand
	e.cmdBuf = fmt.Sprintf("%d,%d!", first+1, last+1)
}

// Strip terminal control sequences and all remaining controls before styling;
// untrusted subprocess output must never be emitted as terminal commands.
func shellSafeText(s string) string {
	s = ansi.Strip(s)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func (e *Editor) shellOutputLines() []string {
	text := shellSafeText(e.shellOutput.text)
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		// Expand tabs so their display width does not depend on terminal state.
		line = strings.ReplaceAll(line, "\t", "    ")
		lines = append(lines, strings.Split(ansi.Wrap(line, max(1, e.width), ""), "\n")...)
	}
	return lines
}

func (e *Editor) shellOutputKey(msg tea.KeyPressMsg) {
	lines := e.shellOutputLines()
	page := max(1, e.viewRows()-1)
	switch typedKey(msg) {
	case "enter":
		e.shellOutput = nil
		return
	case "up", "k":
		e.shellOutput.top--
	case "down", "j":
		e.shellOutput.top++
	case "pgup", "ctrl+b":
		e.shellOutput.top -= page
	case "pgdown", "ctrl+f", " ":
		e.shellOutput.top += page
	case "home":
		e.shellOutput.top = 0
	case "end":
		e.shellOutput.top = len(lines) - page
	}
	e.shellOutput.top = max(0, min(e.shellOutput.top, max(0, len(lines)-page)))
}

func (e *Editor) renderShellOutput() string {
	lines := e.shellOutputLines()
	page := max(1, e.viewRows()-1)
	e.shellOutput.top = max(0, min(e.shellOutput.top, max(0, len(lines)-page)))
	var b strings.Builder
	for i := 0; i < page && e.viewRows() > 1; i++ {
		if n := e.shellOutput.top + i; n < len(lines) {
			b.WriteString(lines[n])
		}
		b.WriteByte('\n')
	}
	b.WriteString(lipgloss.NewStyle().Reverse(true).Render(ansi.Truncate("Enter: dismiss  ↑/↓: scroll", max(1, e.width), "")))
	return b.String()
}
