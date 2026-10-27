package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
	"github.com/AmadeusDE/Misa/internal/webdevtui"
	"github.com/charmbracelet/x/ansi"
)

type screen int

const (
	screenPicker screen = iota
	screenChat
	screenApprovals
	screenRoutines
	screenRepos
	screenCost
	screenStatus
	screenDevices
)

var screenNames = map[screen]string{screenPicker: "Bots", screenChat: "Chat", screenApprovals: "Approvals", screenRoutines: "Routines", screenRepos: "Repos",
	screenCost: "Cost", screenStatus: "Status", screenDevices: "Devices"}

// Model is the root of the ughbot UI.
type Model struct {
	ctx       context.Context
	be        Backend
	screen    screen
	w, h      int
	help      bool
	helpOff   int
	nextChat  int
	picker    pickerModel
	chat      *chatModel
	approvals approvalsModel
	routines  routinesModel
	repos     reposModel
	cost      costModel
	status    statusModel
	devices   *devicesScreen
	form      *botForm
	confirm   string // bot awaiting a delete confirmation
	note      string
}

// New builds the UI; a non-empty bot opens its chat directly.
func New(ctx context.Context, be Backend, bot string) *Model {
	m := &Model{ctx: ctx, be: be, w: 80, h: 24}
	m.devices = newDevicesScreen(ctx, be, m.w, m.bodyHeight())
	if bot != "" {
		m.openChat(bot)
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.fetchBots(), m.fetchApprovals(), m.pollApprovals()}
	if m.chat != nil {
		cmds = append(cmds, m.chat.init())
	}
	return tea.Batch(cmds...)
}

func (m *Model) fetchBots() tea.Cmd {
	return func() tea.Msg {
		bots, err := m.be.ListBots(m.ctx)
		return botsMsg{bots, err}
	}
}

func (m *Model) fetchStatus() tea.Cmd {
	return func() tea.Msg {
		st, err := m.be.Status(m.ctx)
		return statusMsg{st, err}
	}
}

func (m *Model) fetchCost() tea.Cmd {
	bot := m.cost.filter
	return func() tea.Msg {
		rows, err := m.be.Cost(m.ctx, api.CostQuery{Bot: bot})
		return costMsg{bot, rows, err}
	}
}

func (m *Model) bodyHeight() int { return max(m.h-2, 1) }

func (m *Model) openChat(bot string) tea.Cmd {
	m.screen = screenChat
	if m.chat != nil {
		if m.chat.bot == bot {
			return nil
		}
		m.chat.cancel()
	}
	m.nextChat++
	m.chat = newChat(m.ctx, m.be, m.nextChat, bot, m.w, m.bodyHeight())
	return m.chat.init()
}

func (m *Model) screens() []screen {
	out := []screen{screenPicker}
	if m.chat != nil {
		out = append(out, screenChat)
	}
	if _, ok := m.approvalBackend(); ok {
		out = append(out, screenApprovals)
	}
	if _, ok := m.routineBackend(); ok {
		out = append(out, screenRoutines)
	}
	if _, ok := m.repoBackend(); ok {
		out = append(out, screenRepos)
	}
	return append(out, screenCost, screenStatus, screenDevices)
}

func (m *Model) goTo(s screen) tea.Cmd {
	m.screen = s
	switch s {
	case screenPicker:
		return m.fetchBots()
	case screenApprovals:
		return m.fetchApprovals()
	case screenRoutines:
		return tea.Batch(m.fetchRoutines(), m.fetchBots())
	case screenRepos:
		return m.fetchRepos()
	case screenCost:
		return m.fetchCost()
	case screenStatus:
		return m.fetchStatus()
	case screenDevices:
		return m.devices.model.Reload()
	}
	return nil
}

// goToNumber jumps to the nth screen of the header (1-based).
func (m *Model) goToNumber(n int) tea.Cmd {
	order := m.screens()
	if n < 1 || n > len(order) {
		return nil
	}
	return m.goTo(order[n-1])
}

func (m *Model) step(delta int) tea.Cmd {
	order := m.screens()
	at := 0
	for i, s := range order {
		if s == m.screen {
			at = i
		}
	}
	return m.goTo(order[(at+delta+len(order))%len(order)])
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if webdevtui.Owns(msg) {
		return m, m.devices.update(msg)
	}
	if id, ok := msgID(msg); ok {
		if m.chat == nil || m.chat.id != id {
			return m, nil
		}
		return m, m.chat.update(msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.devices.model.Resize(m.w, m.bodyHeight())
		if m.form != nil {
			m.form.resize(m.w, m.bodyHeight())
		}
		if m.routines.form != nil {
			m.routines.form.resize(m.w, m.bodyHeight())
		}
		if m.chat != nil {
			m.chat.resize(m.w, m.bodyHeight())
		}
	case botsMsg:
		m.picker.load(msg)
	case botDefMsg:
		if msg.err != nil {
			m.note = "cannot load bot: " + msg.err.Error()
			return m, nil
		}
		m.form = newBotForm(&msg.def, m.w, m.bodyHeight())
	case botSavedMsg:
		return m, m.botSaved(msg)
	case approvalsMsg:
		m.approvals.load(msg)
	case decidedMsg:
		return m, m.decided(msg)
	case routinesMsg:
		m.routines.load(msg)
	case routineDoneMsg:
		return m, m.routineDone(msg)
	case reposMsg:
		m.repos.load(msg)
	case repoDoneMsg:
		return m, m.repoDone(msg)
	case approvalTick:
		return m, tea.Batch(m.fetchApprovals(), m.pollApprovals())
	case statusMsg:
		m.status.load(msg)
	case costMsg:
		m.cost.load(msg)
	case tea.KeyPressMsg:
		return m.key(msg)
	case tea.PasteMsg:
		if m.routines.form != nil {
			return m.routineFormKey(msg)
		}
		if m.form != nil {
			m.form.update(msg)
			return m, nil
		}
		if m.screen == screenChat && m.chat != nil {
			return m, m.chat.passthrough(msg)
		}
	default:
		if m.screen == screenChat && m.chat != nil {
			return m, m.chat.passthrough(msg)
		}
	}
	return m, nil
}

func (m *Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.help {
		if !m.helpKey(msg.String()) {
			m.help, m.helpOff = false, 0
		}
		return m, nil
	}
	if m.form != nil {
		return m.formKey(msg)
	}
	if m.routines.form != nil {
		return m.routineFormKey(msg)
	}
	if m.confirm != "" {
		return m.confirmKey(msg)
	}
	chatting := m.screen == screenChat && m.chat != nil
	capturing := m.screen == screenDevices && m.devices.model.Capturing()
	switch msg.String() {
	case "ctrl+c":
		return m, m.quit()
	case "f1":
		m.help = true
		return m, nil
	case "ctrl+o":
		if !capturing {
			return m, m.step(1)
		}
	}
	if chatting {
		switch msg.String() {
		case "ctrl+d":
			return m, m.quit()
		case "esc":
			return m, m.goTo(screenPicker)
		}
		return m, m.chat.key(msg)
	}
	if capturing {
		return m, m.devices.update(msg)
	}
	if !(m.screen == screenPicker && m.picker.filtering) && !(m.screen == screenApprovals && m.approvals.confirm != "") && !(m.screen == screenRoutines && m.routines.confirm) && !(m.screen == screenRepos && m.repos.confirm) {
		if cmd, handled := m.globalKey(msg); handled {
			return m, cmd
		}
	}
	switch m.screen {
	case screenPicker:
		if cmd, handled := m.botAdminKey(msg); handled {
			return m, cmd
		}
		if bot := m.picker.key(msg); bot != "" {
			return m, m.openChat(bot)
		}
	case screenApprovals:
		return m, m.approvalsKey(msg)
	case screenRoutines:
		return m, m.routinesKey(msg)
	case screenRepos:
		return m, m.reposKey(msg)
	case screenCost:
		if m.cost.key(msg, m.botNames()) {
			return m, m.fetchCost()
		}
	case screenStatus:
		if msg.String() == "r" {
			return m, m.fetchStatus()
		}
	case screenDevices:
		if msg.String() == "esc" {
			return m, m.goTo(screenPicker)
		}
		return m, m.devices.update(msg)
	}
	return m, nil
}

// globalKey handles the navigation keys shared by every screen but the chat input.
func (m *Model) globalKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch s := msg.String(); s {
	case "q":
		return m.quit(), true
	case "?":
		m.help = true
		return nil, true
	case "tab", "]":
		return m.step(1), true
	case "shift+tab", "[":
		return m.step(-1), true
	default:
		if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
			return m.goToNumber(int(s[0] - '0')), true
		}
	}
	return nil, false
}

func (m *Model) botNames() []string {
	var names []string
	for _, b := range m.picker.bots {
		names = append(names, b.Name)
	}
	return names
}

func (m *Model) quit() tea.Cmd {
	if m.chat != nil {
		m.chat.cancel()
	}
	return tea.Quit
}

func (m *Model) header() string {
	var tabs []string
	for _, s := range m.screens() {
		name := screenNames[s]
		if s == screenChat {
			name += " " + m.chat.bot
		}
		if s == screenApprovals && m.approvals.pending > 0 {
			name += fmt.Sprintf(" %d", m.approvals.pending)
		}
		if s == m.screen {
			tabs = append(tabs, tabActive.Render(name))
		} else {
			tabs = append(tabs, tabStyle.Render(name))
		}
	}
	return ansi.Truncate(titleStyle.Render("ughbot")+" "+strings.Join(tabs, ""), m.w, "…")
}

func (m *Model) footer() string {
	nav := fmt.Sprintf("tab/[ ] screens  1-%d jump  ? help  q quit", len(m.screens()))
	hint := nav
	if m.form != nil {
		return hintStyle.Render(clipWidth(m.form.hint(), m.w))
	}
	if m.confirm != "" {
		return warnStyle.Render("delete bot " + m.confirm + "? y/N")
	}
	switch m.screen {
	case screenChat:
		return hintStyle.Render(fit(m.w, "enter send  alt+enter newline  pgup/pgdn scroll  esc bots  ctrl+o next screen  f1 help  ctrl+c quit",
			"enter send  alt+enter newline  esc bots  ctrl+o next  f1 help"))
	case screenPicker:
		hint = "enter open  n new  p preset  e edit  x delete  / filter  " + nav
	case screenApprovals:
		if m.approvals.confirm != "" {
			return warnStyle.Render(clipWidth(m.approvals.hint(), m.w))
		}
		hint = m.approvals.hint() + "  " + nav
	case screenRoutines:
		if m.routines.form != nil || m.routines.confirm {
			return warnStyle.Render(clipWidth(m.routines.hint(), m.w))
		}
		hint = m.routines.hint() + "  " + nav
	case screenRepos:
		if m.repos.confirm {
			return warnStyle.Render(clipWidth(m.repos.hint(), m.w))
		}
		hint = m.repos.hint() + "  " + nav
	case screenCost:
		hint = "left/right bot  r refresh  " + nav
	case screenDevices:
		hint = strings.TrimSpace(m.devices.hint() + "  " + nav)
	}
	return hintStyle.Render(fit(m.w, hint, strings.TrimSuffix(hint, "  "+nav)+"  ? help"))
}

// fit returns the first candidate that fits in w columns, else the last one cut to w.
func fit(w int, candidates ...string) string {
	for _, c := range candidates {
		if lipgloss.Width(c) <= w {
			return c
		}
	}
	return clipWidth(candidates[len(candidates)-1], w)
}

func clipWidth(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return string([]rune(s)[:max(w, 0)])
}

func (m *Model) View() tea.View {
	var body string
	bh := m.bodyHeight()
	switch {
	case m.help:
		body, _ = helpView(m.helpOff, bh)
	case m.form != nil:
		body = m.form.view()
	case m.screen == screenChat && m.chat != nil:
		body = m.chat.view()
	case m.screen == screenPicker:
		body = m.picker.view(m.w, bh)
		if m.note != "" {
			body += "\n\n" + warnStyle.Render(m.note)
		}
	case m.screen == screenDevices:
		body = m.devices.view()
	case m.screen == screenApprovals:
		body = m.approvals.view(m.w, bh)
	case m.screen == screenRoutines:
		body = m.routines.view(m.w, bh)
	case m.screen == screenRepos:
		body = m.repos.view(m.w, bh)
	case m.screen == screenCost:
		body = m.cost.view(m.w, bh)
	default:
		body = m.status.view(m.w, bh)
	}
	body = lipgloss.NewStyle().Width(m.w).Height(bh).MaxWidth(m.w).MaxHeight(bh).Render(body)
	v := tea.NewView(m.header() + "\n" + body + "\n" + m.footer())
	v.AltScreen = true
	return v
}
