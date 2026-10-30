package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AmadeusDE/Misa/internal/thefuck"
)

// aiFixer asks the help model to correct a failed command. It stays nil without AI support (-tags nough), and
// aiFixReady then also stays nil, so the AI is never offered.
var (
	aiFixer    func(cmd, output string, exit int) (fix, why string, err error)
	aiFixReady func() bool
)

// aiFixMsg is the AI's answer to ctrl+f on a failed command no thefuck rule knew.
type aiFixMsg struct {
	cmd, fix, why string
	err           error
}

// aiFixState is the failed command the AI may be asked about; a new command or an answer clears it.
type aiFixState struct {
	cmd, output string
	exit        int
	busy        bool
}

var (
	fixStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f")).Bold(true)
	fixHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
)

func (m *Model) clearFix() {
	m.thefuckFix = ""
	m.aiFix = aiFixState{}
}

// offerFix shows the pattern-based fix for the failed command, or when no rule matches and a help model is
// chosen, tells the user ctrl+f can ask the AI. It never asks by itself.
func (m *Model) offerFix(output string, exit int) {
	m.clearFix()
	cmd := m.shell.lastCmd
	if exit == 0 || cmd == "" {
		return
	}
	if fix := thefuck.Match(cmd, output, exit); fix != "" {
		m.thefuckFix = fix
		m.shellBuffer = append(m.shellBuffer, fixStyle.Render("fix → ")+fix+fixStyle.Render("  [ctrl+f]"))
		return
	}
	if aiFixer != nil && aiFixReady != nil && aiFixReady() {
		m.aiFix = aiFixState{cmd: cmd, output: output, exit: exit}
		m.shellBuffer = append(m.shellBuffer, fixHintStyle.Render("no known fix  [ctrl+f: ask AI]"))
	}
}

// askAIFix is ctrl+f with no pattern fix pending: it asks the AI about the last failed command.
func (m *Model) askAIFix() tea.Cmd {
	st := m.aiFix
	if st.cmd == "" || st.busy {
		return nil
	}
	m.aiFix.busy = true
	m.shellBuffer = append(m.shellBuffer, fixHintStyle.Render("asking AI..."))
	ask := aiFixer
	return func() tea.Msg {
		fix, why, err := ask(st.cmd, st.output, st.exit)
		return aiFixMsg{cmd: st.cmd, fix: fix, why: why, err: err}
	}
}

// receiveAIFix shows the AI's suggestion like a pattern fix: ctrl+f runs it.
func (m *Model) receiveAIFix(msg aiFixMsg) {
	if m.aiFix.cmd != msg.cmd {
		return // another command has run since
	}
	m.aiFix = aiFixState{}
	switch {
	case msg.err != nil:
		m.shellBuffer = append(m.shellBuffer, fixHintStyle.Render("AI fix failed: "+msg.err.Error()))
	case msg.fix == msg.cmd:
		m.shellBuffer = append(m.shellBuffer, fixHintStyle.Render("AI: the command looks right"))
	default:
		m.thefuckFix = msg.fix
		m.shellBuffer = append(m.shellBuffer, fixStyle.Render("fix → ")+msg.fix+fixStyle.Render("  [ctrl+f]"))
	}
	if msg.err == nil && msg.why != "" {
		for _, line := range strings.Split(msg.why, "\n") {
			m.shellBuffer = append(m.shellBuffer, fixHintStyle.Render(line))
		}
	}
	if m.screen == screenShell {
		m.updateFullShellVP()
	} else {
		m.updateShellView()
	}
}
