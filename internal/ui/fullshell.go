package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AmadeusDE/Misa/internal/term"
)

func (m *Model) enterFullShell() {
	m.screen = screenShell
	m.inputMode = true
	m.cmdInput.Focus()
	m.updateFullShellVP()
	m.resize()
}

func (m *Model) exitFullShell() {
	if m.terminal != nil {
		m.terminal.Close()
		m.terminal = nil
	}
	m.screen = screenMenu
	m.inputMode = false
	m.cmdInput.Blur()
	m.resize()
}

func (m *Model) updateFullShellVP() {
	wasAtBottom := m.fullShellVP.AtBottom()
	content := strings.Join(m.shellBuffer, "\n")
	m.fullShellVP.SetContent(content)
	if wasAtBottom {
		m.fullShellVP.GotoBottom()
	}
}

func (m *Model) updateFullShellVPWithTerm() {
	wasAtBottom := m.fullShellVP.AtBottom()
	var parts []string
	if len(m.shellBuffer) > 0 {
		parts = append(parts, strings.Join(m.shellBuffer, "\n"))
	}
	if m.terminal != nil {
		showCur := m.cursorBlink && m.terminal.CursorVisible()
		termLines := m.terminal.LinesWithCursor(showCur, nil)
		if len(termLines) > 0 {
			parts = append(parts, strings.Join(termLines, "\n"))
		}
	}
	content := strings.Join(parts, "\n")
	m.fullShellVP.SetContent(content)
	if wasAtBottom {
		m.fullShellVP.GotoBottom()
	}
}

// copyRows leaves room for copy mode's hint line.
func (m *Model) copyRows() int {
	if m.height > 2 {
		return m.height - 2
	}
	return m.height
}

// fullShellTermTop is the view row of the running terminal's first screen
// row while it shares the viewport with earlier output.
func (m *Model) fullShellTermTop() int {
	top := m.terminal.ScrollbackLen() - m.fullShellVP.YOffset()
	if len(m.shellBuffer) > 0 {
		top += strings.Count(strings.Join(m.shellBuffer, "\n"), "\n") + 1
	}
	return top
}

// fullShellPlainLines mirrors updateFullShellVPWithTerm's content, unstyled.
func (m *Model) fullShellPlainLines() []string {
	var lines []string
	if len(m.shellBuffer) > 0 {
		lines = strings.Split(ansi.Strip(strings.Join(m.shellBuffer, "\n")), "\n")
	}
	if m.terminal != nil {
		lines = append(lines, m.terminal.PlainLines()...)
	}
	return lines
}

func (m *Model) updateFullShell(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.session != nil {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.PasteMsg, mouseMsg, tea.FocusMsg, tea.BlurMsg:
			return m.updateAgentInput(msg)
		}
		return m, nil
	}
	if m.copyMode.active {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.updateCopyMode(keyMsg)
		}
		return m, nil
	}

	if m.terminal != nil {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if msg.String() == "ctrl+y" {
				m.copyMode = newCopyState(m.terminal, m.fullShellPlainLines(), m.copyRows(), m.width)
				return m, nil
			}
			if msg.String() == "ctrl+v" {
				pasteToTerm(m.terminal)
				return m, nil
			}
			if m.selKey(msg, m.terminal) {
				return m, nil
			}
			m.sel.clear()
			sendKey(m.terminal, msg)
			return m, nil
		case tea.PasteMsg:
			m.sel.clear()
			sendPaste(m.terminal, msg.Content)
			return m, nil
		case mouseMsg:
			if m.terminal.InAltScreen() {
				if !forwardMouse(m.terminal, msg, msg.X, msg.Y) && !scrollAltScreen(m.terminal, msg) {
					_, cmd := m.selMouse(msg, m.terminal)
					return m, cmd
				}
				return m, nil
			}
			if forwardMouse(m.terminal, msg, msg.X, msg.Y-m.fullShellTermTop()) {
				return m, nil
			}
			if handled, cmd := m.handleVPSelMouse(msg, m.fullShellVP.YOffset(), m.fullShellPlainLines(), 0); handled {
				return m, cmd
			}
			if isWheelUp(msg) {
				m.fullShellVP.ScrollUp(3)
				return m, nil
			}
			if isWheelDown(msg) {
				m.fullShellVP.ScrollDown(3)
				return m, nil
			}
			return m, nil
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case mouseMsg:
		if handled, cmd := m.handleVPSelMouse(msg, m.fullShellVP.YOffset(), m.fullShellPlainLines(), 0); handled {
			return m, cmd
		}
		if isWheelUp(msg) {
			m.fullShellVP.ScrollUp(3)
			return m, nil
		}
		if isWheelDown(msg) {
			m.fullShellVP.ScrollDown(3)
			return m, nil
		}
		return m, nil
	case tea.KeyPressMsg:
		m.sel.clear()
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "ctrl+y":
			m.copyMode = newCopyState(m.terminal, m.fullShellPlainLines(), m.copyRows(), m.width)
			return m, nil

		case "ctrl+d":
			m.exitFullShell()
			return m, nil

		case "ctrl+f":
			if m.thefuckFix != "" {
				fix := m.thefuckFix
				m.thefuckFix = ""
				m.shell.lastCmd = fix
				m.shellBuffer = append(m.shellBuffer, m.renderPrompt()+fix)
				if handled, cmd := m.handleBuiltinCmd(fix); handled {
					m.cmdInput.SetValue("")
					m.updateFullShellVP()
					return m, cmd
				}
				m.cmdInput.SetValue("")
				m.updateFullShellVP()
				return m, m.execShellCmd(fix)
			}
			return m, nil

		case "ctrl+v":
			text := m.registerText()
			if text == "" {
				text = readClipboard()
			}
			if text != "" {
				insertAtCursor(&m.cmdInput, text)
			}
			return m, nil

		case "shift+up", "pgup":
			m.fullShellVP.ScrollUp(3)
			return m, nil
		case "shift+down", "pgdown":
			m.fullShellVP.ScrollDown(3)
			return m, nil
		case "end":
			m.fullShellVP.GotoBottom()
			return m, nil

		case "up":
			val := m.shell.historyUp(m.cmdInput.Value())
			m.cmdInput.SetValue(val)
			m.cmdInput.SetCursor(len(val))
			return m, nil
		case "down":
			val := m.shell.historyDown()
			m.cmdInput.SetValue(val)
			m.cmdInput.SetCursor(len(val))
			return m, nil

		case "right":
			input := m.cmdInput.Value()
			if m.cmdInput.Position() >= utf8.RuneCountInString(input) && m.shell.suggestion != "" {
				accepted := m.shell.acceptSuggestion(input)
				if accepted != input {
					m.cmdInput.SetValue(accepted)
					m.cmdInput.SetCursor(len(accepted))
					return m, nil
				}
			}

		case "tab":
			input := m.cmdInput.Value()
			if strings.TrimSpace(input) == "" {
				return m, nil
			}
			completed := m.shell.tabComplete(input, m.dir)
			if completed != input {
				m.cmdInput.SetValue(completed)
				m.cmdInput.SetCursor(len(completed))
			}
			return m, nil

		case "enter":
			val := strings.TrimSpace(m.cmdInput.Value())
			if val == "" {
				return m, nil
			}

			m.shell.addHistory(val)
			m.shell.resetHistory()
			m.shell.suggestion = ""
			m.thefuckFix = ""
			m.shellBuffer = append(m.shellBuffer, m.renderPrompt()+val)

			expanded := m.expandAlias(val)
			m.shell.lastCmd = expanded
			fields := strings.Fields(expanded)
			if len(fields) == 0 {
				m.cmdInput.SetValue("")
				m.updateFullShellVP()
				return m, nil
			}

			switch compoundAware(fields[0], expanded) {
			case "clear":
				m.shellBuffer = nil
				m.updateFullShellVP()
				m.updateShellView()
				m.cmdInput.SetValue("")
				return m, nil
			case "exit", "misa":
				m.exitFullShell()
				return m, nil
			}

			if native, nativeCmd := stripNativePrefix(expanded); native {
				m.cmdInput.SetValue("")
				m.shellBuffer = append(m.shellBuffer, m.renderPrompt()+"! "+nativeCmd)
				m.updateFullShellVP()
				return m, m.execNative(nativeCmd)
			}

			if handled, cmd := m.handleBuiltinCmd(expanded); handled {
				m.cmdInput.SetValue("")
				m.updateFullShellVP()
				return m, cmd
			}

			m.cmdInput.SetValue("")
			m.updateFullShellVP()
			return m, m.execShellCmd(expanded)
		}
	}

	var cmd tea.Cmd
	prev := m.cmdInput.Value()
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	if m.cmdInput.Value() != prev {
		m.shell.complete(m.cmdInput.Value(), m.dir)
		m.shell.resetHistory()
	}
	return m, cmd
}

func (m *Model) fullShellContent() string {
	if m.copyMode.active {
		return m.copyMode.render(m.uiColors.ghost)
	}

	if m.terminal != nil && m.terminal.InAltScreen() {
		var sel *term.Highlight
		if m.sel.hasRange {
			sel = &term.Highlight{
				StartRow: m.sel.startRow, StartCol: m.sel.startCol,
				EndRow: m.sel.endRow, EndCol: m.sel.endCol,
			}
		}
		return m.terminal.RenderFull(m.cursorBlink && m.terminal.CursorVisible(), sel)
	}

	if m.session != nil {
		return m.session.a.View().Content
	}
	if m.terminal != nil {
		return m.sel.highlightRows(m.fullShellVP.View(), m.fullShellVP.YOffset())
	}

	prompt := m.renderPrompt()
	inputView := renderInput(m.cmdInput)
	ghost := ""
	if m.shell.suggestion != "" {
		input := strings.TrimSpace(m.cmdInput.Value())
		if input != "" && strings.HasPrefix(m.shell.suggestion, input) {
			ghost = lipgloss.NewStyle().Foreground(m.uiColors.ghost).Render(m.shell.suggestion[len(input):])
		}
	}

	hintText := "misa: open TUI  ctrl+d: back  ctrl+w: split  ctrl+f: fix  ctrl+y: copy"
	if !m.fullShellVP.AtBottom() {
		hintText = "▼ new output below — End: jump to bottom  " + hintText
	}
	hint := lipgloss.NewStyle().Foreground(m.uiColors.ghost).Render(hintText)

	return m.sel.highlightRows(m.fullShellVP.View(), m.fullShellVP.YOffset()) +
		"\n" + fmt.Sprintf("%s%s%s", prompt, inputView, ghost) +
		"\n" + hint
}
