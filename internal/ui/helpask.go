package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/help"
)

// aiAsker answers a question about a help page. It stays nil without AI support (-tags nough).
var aiAsker func(command, question, page string) (string, error)

const helpFooter = "  esc: back  scroll: up/down  a: ask AI"

// helpAnswerMsg is the AI's answer to a question about the page in the help viewer.
type helpAnswerMsg struct {
	command, question, answer string
	err                       error
}

// helpAskState is "Ask AI" on the TLDR/man page in the viewer: the page, the conversation so far and the prompt.
type helpAskState struct {
	command, page string
	rendered      string // the page as shown, before any answers
	transcript    string
	input         textinput.Model
	prompting     bool
	busy          bool
}

func (m *Model) showHelpPage(command, page string) {
	m.helpAsk = helpAskState{command: command, page: page, rendered: help.RenderMarkdown(page) + "\n\n" + settingHelpStyle.Render(helpFooter)}
	m.helpView.SetContent(m.helpAsk.rendered)
	m.screen = screenHelp
	m.resize()
	m.helpView.GotoTop()
}

func (m *Model) helpPromptView() string {
	if !m.helpAsk.prompting {
		return ""
	}
	return "\n" + settingHelpStyle.Render("Ask AI: ") + renderInput(m.helpAsk.input)
}

func (m *Model) helpPromptRows() int {
	if m.helpAsk.prompting {
		return 1
	}
	return 0
}

// setHelpPrompting shows or hides the prompt line, which takes a row from the viewer.
func (m *Model) setHelpPrompting(on bool) {
	if m.helpAsk.prompting == on {
		return
	}
	m.helpAsk.prompting = on
	if on {
		m.helpView.SetHeight(m.helpView.Height() - 1)
	} else {
		m.helpView.SetHeight(m.helpView.Height() + 1)
	}
}

func (m *Model) startHelpAsk() tea.Cmd {
	st := &m.helpAsk
	if st.command == "" || st.busy || st.prompting {
		return nil
	}
	if aiAsker == nil {
		m.appendHelpAnswer(settingHelpStyle.Render("built without AI support (-tags nough)"))
		return nil
	}
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "ask about " + st.command + "..."
	ti.CharLimit = 512
	ti.SetWidth(max(m.helpView.Width()-10, 10))
	st.input = ti
	m.setHelpPrompting(true)
	return st.input.Focus()
}

func (m *Model) updateHelpPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	st := &m.helpAsk
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.setHelpPrompting(false)
			return m, nil
		case "enter":
			question := strings.TrimSpace(st.input.Value())
			m.setHelpPrompting(false)
			if question == "" {
				return m, nil
			}
			return m, m.askHelpAI(question)
		}
	}
	var cmd tea.Cmd
	st.input, cmd = st.input.Update(msg)
	return m, cmd
}

func (m *Model) askHelpAI(question string) tea.Cmd {
	st := &m.helpAsk
	st.busy = true
	m.appendHelpAnswer(settingHelpStyle.Render("Q: "+question) + "\n" + settingHelpStyle.Render("asking AI..."))
	command, page, ask := st.command, st.page, aiAsker
	return func() tea.Msg {
		answer, err := ask(command, question, page)
		return helpAnswerMsg{command: command, question: question, answer: answer, err: err}
	}
}

// appendHelpAnswer adds to the transcript under the page and scrolls to the start of the addition.
func (m *Model) appendHelpAnswer(block string) {
	st := &m.helpAsk
	before := st.rendered + st.transcript
	st.transcript += "\n\n" + block
	m.helpView.SetContent(st.rendered + st.transcript)
	m.helpView.SetYOffset(strings.Count(before, "\n") + 2)
}

// receiveHelpAnswer replaces "asking AI..." with the answer, unless another page has been opened since.
func (m *Model) receiveHelpAnswer(msg helpAnswerMsg) {
	st := &m.helpAsk
	if st.command != msg.command || !st.busy {
		return
	}
	st.busy = false
	waiting := "\n" + settingHelpStyle.Render("asking AI...")
	body := help.RenderMarkdown(msg.answer)
	if msg.err != nil {
		body = settingHelpStyle.Render("AI: " + msg.err.Error())
	}
	st.transcript = strings.TrimSuffix(st.transcript, waiting) + "\n" + body
	m.helpView.SetContent(st.rendered + st.transcript)
}
