package webdevtui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const eventLimit = 200

type view int

const (
	viewDevices view = iota
	viewSessions
	viewEvents
)

var viewNames = []string{"Devices", "Sessions", "Events"}

type pendingKind int

const (
	pendingNone pendingKind = iota
	pendingRevoke
	pendingKill
)

type (
	loadedMsg struct {
		devices  []Device
		sessions []Session
		events   []Event
		notices  []Notice
		err      error
	}
	doneMsg struct {
		note string
		err  error
	}
)

// Owns reports whether msg is one of this package's results, so a host can
// route it here even after it has switched screens.
func Owns(msg tea.Msg) bool {
	switch msg.(type) {
	case loadedMsg, doneMsg:
		return true
	}
	return false
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	headStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	hintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	selStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
)

// Model is the device management screen. Hosts forward key presses and
// Owns(msg) results to Update and call Capturing to know when the screen
// needs every key (rename text, confirmation).
type Model struct {
	admin    Admin
	view     view
	cursor   [3]int
	devices  []Device
	sessions []Session
	events   []Event
	notices  []Notice
	loaded   bool
	err      error
	actErr   error
	note     string
	pending  pendingKind
	target   string
	targetLb string
	rename   textinput.Model
	renaming bool
	w, h     int
	quit     bool
	now      func() time.Time
}

func New(a Admin) *Model {
	in := textinput.New()
	in.Prompt = "new name: "
	in.CharLimit = 64
	return &Model{admin: a, rename: in, w: 80, h: 22, now: time.Now}
}

func (m *Model) Init() tea.Cmd { return m.reload() }

// Reload fetches everything again.
func (m *Model) Reload() tea.Cmd { return m.reload() }

func (m *Model) Resize(w, h int) { m.w, m.h = w, h }

// Err is the last load error, nil when the data loaded.
func (m *Model) Err() error { return m.err }

// Quit is set when the user asked to leave (q or esc outside a prompt).
func (m *Model) Quit() bool { return m.quit }

// Capturing is true while a prompt needs every key.
func (m *Model) Capturing() bool { return m.renaming || m.pending != pendingNone }

func (m *Model) reload() tea.Cmd {
	a := m.admin
	return func() tea.Msg {
		var r loadedMsg
		if r.devices, r.err = a.Devices(); r.err != nil {
			return r
		}
		if r.sessions, r.err = a.Sessions(); r.err != nil {
			return r
		}
		if r.events, r.err = a.Events(eventLimit); r.err != nil {
			return r
		}
		r.notices, r.err = a.Notices()
		return r
	}
}

func (m *Model) act(note string, fn func() error) tea.Cmd {
	return tea.Sequence(func() tea.Msg { return doneMsg{note, fn()} }, m.reload())
}

func (m *Model) rowCount() int {
	switch m.view {
	case viewDevices:
		return len(m.devices)
	case viewSessions:
		return len(m.sessions)
	}
	return len(m.events)
}

func (m *Model) clamp() {
	for v := range m.cursor {
		n := [3]int{len(m.devices), len(m.sessions), len(m.events)}[v]
		m.cursor[v] = min(max(m.cursor[v], 0), max(n-1, 0))
	}
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case loadedMsg:
		m.loaded, m.err = true, msg.err
		if msg.err == nil {
			m.devices, m.sessions, m.events, m.notices = msg.devices, msg.sessions, reverse(msg.events), msg.notices
		}
		m.clamp()
	case doneMsg:
		m.note, m.actErr = msg.note, msg.err
	case tea.KeyPressMsg:
		return m.key(msg)
	default:
		if m.renaming {
			var cmd tea.Cmd
			m.rename, cmd = m.rename.Update(msg)
			return cmd
		}
	}
	return nil
}

func reverse(ev []Event) []Event {
	out := make([]Event, len(ev))
	for i, e := range ev {
		out[len(ev)-1-i] = e
	}
	return out
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case m.renaming:
		return m.renameKey(msg)
	case m.pending != pendingNone:
		return m.confirmKey(msg)
	}
	m.note, m.actErr = "", nil
	switch msg.String() {
	case "up", "k":
		if m.view == viewSessions && msg.String() == "k" {
			return m.askKill()
		}
		m.cursor[m.view]--
	case "down", "j":
		m.cursor[m.view]++
	case "pgup":
		m.cursor[m.view] -= max(m.h-6, 1)
	case "pgdown":
		m.cursor[m.view] += max(m.h-6, 1)
	case "home", "g":
		m.cursor[m.view] = 0
	case "end", "G":
		m.cursor[m.view] = m.rowCount() - 1
	case "left", "h":
		m.view = (m.view + 2) % 3
	case "right", "l":
		m.view = (m.view + 1) % 3
	case "r":
		return m.startRename()
	case "x":
		if m.view == viewSessions {
			return m.askKill()
		}
		return m.askRevoke()
	case "d":
		return m.dismiss()
	case "R":
		return m.reload()
	case "q", "esc":
		m.quit = true
	}
	m.clamp()
	return nil
}

func (m *Model) selectedDevice() (Device, bool) {
	if m.view != viewDevices || m.cursor[viewDevices] >= len(m.devices) {
		return Device{}, false
	}
	return m.devices[m.cursor[viewDevices]], true
}

func (m *Model) startRename() tea.Cmd {
	d, ok := m.selectedDevice()
	if !ok {
		return nil
	}
	m.renaming, m.target = true, d.ID
	m.rename.SetValue(d.Name)
	m.rename.CursorEnd()
	return m.rename.Focus()
}

func (m *Model) renameKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.renaming = false
		m.rename.Blur()
		return nil
	case "enter":
		name, id := strings.TrimSpace(m.rename.Value()), m.target
		m.renaming = false
		m.rename.Blur()
		if name == "" {
			return nil
		}
		return m.act("renamed to "+name, func() error { return m.admin.Rename(id, name) })
	}
	var cmd tea.Cmd
	m.rename, cmd = m.rename.Update(msg)
	return cmd
}

func (m *Model) askRevoke() tea.Cmd {
	d, ok := m.selectedDevice()
	if !ok {
		return nil
	}
	if d.Revoked {
		m.note = d.Name + " is already revoked"
		return nil
	}
	m.pending, m.target, m.targetLb = pendingRevoke, d.ID, d.Name
	return nil
}

func (m *Model) askKill() tea.Cmd {
	i := m.cursor[viewSessions]
	if i >= len(m.sessions) {
		return nil
	}
	s := m.sessions[i]
	m.pending, m.target, m.targetLb = pendingKill, s.ID, s.DeviceName+" "+s.ID
	return nil
}

func (m *Model) confirmKey(msg tea.KeyPressMsg) tea.Cmd {
	kind, id, label := m.pending, m.target, m.targetLb
	m.pending = pendingNone
	if msg.String() != "y" {
		m.note = "cancelled"
		return nil
	}
	if kind == pendingRevoke {
		return m.act("revoked "+label, func() error { return m.admin.Revoke(id) })
	}
	return m.act("ended session "+label, func() error { return m.admin.KillSession(id) })
}

func (m *Model) dismiss() tea.Cmd {
	if len(m.notices) == 0 {
		return nil
	}
	id := m.notices[0].ID
	return m.act("notice dismissed", func() error { return m.admin.DismissNotice(id) })
}

// Hint is the key legend for the current state, for the host's footer.
func (m *Model) Hint() string {
	switch {
	case m.renaming:
		return "enter save  esc cancel"
	case m.pending != pendingNone:
		return "y confirm  any other key cancels"
	}
	hint := "up/down move  left/right view  R refresh"
	switch m.view {
	case viewDevices:
		hint += "  r rename  x revoke"
	case viewSessions:
		hint += "  k/x end session"
	}
	if len(m.notices) > 0 {
		hint += "  d dismiss notice"
	}
	return hint
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(m.tabs() + "\n")
	if len(m.notices) > 0 {
		extra := ""
		if len(m.notices) > 1 {
			extra = fmt.Sprintf(" (+%d more)", len(m.notices)-1)
		}
		b.WriteString(warnStyle.Render(clip("! "+m.notices[0].Text+extra+"  [d] dismiss", m.w)) + "\n")
	}
	switch {
	case m.actErr != nil:
		b.WriteString(errStyle.Render(clip(m.actErr.Error(), m.w)) + "\n")
	case m.err != nil:
		b.WriteString(errStyle.Render(clip(m.err.Error(), m.w)) + "\n")
	case m.note != "":
		b.WriteString(okStyle.Render(clip(m.note, m.w)) + "\n")
	}
	switch {
	case !m.loaded && m.err == nil:
		b.WriteString(dimStyle.Render("loading..."))
	default:
		b.WriteString(m.body())
	}
	b.WriteString("\n" + m.prompt())
	return lipgloss.NewStyle().MaxWidth(max(m.w, 1)).Render(strings.TrimRight(b.String(), "\n"))
}

func (m *Model) tabs() string {
	var parts []string
	for i, n := range viewNames {
		if view(i) == m.view {
			parts = append(parts, selStyle.Render("["+n+"]"))
		} else {
			parts = append(parts, dimStyle.Render(" "+n+" "))
		}
	}
	return titleStyle.Render("Web devices") + "  " + strings.Join(parts, "")
}

func (m *Model) prompt() string {
	switch {
	case m.renaming:
		return m.rename.View()
	case m.pending == pendingRevoke:
		return warnStyle.Render(clip(fmt.Sprintf("Revoke %s and end its sessions? y/N", m.targetLb), m.w))
	case m.pending == pendingKill:
		return warnStyle.Render(clip(fmt.Sprintf("End session %s? y/N", m.targetLb), m.w))
	}
	return ""
}

func (m *Model) visibleRows() int { return max(m.h-5, 1) }

func (m *Model) window(n int) (start, end int) {
	rows := m.visibleRows()
	start = min(max(m.cursor[m.view]-rows+1, 0), max(n-rows, 0))
	return start, min(start+rows, n)
}

func (m *Model) body() string {
	var lines []string
	switch m.view {
	case viewDevices:
		lines = m.deviceLines()
	case viewSessions:
		lines = m.sessionLines()
	default:
		lines = m.eventLines()
	}
	return strings.Join(lines, "\n")
}

func (m *Model) row(i int, text string) string {
	if i == m.cursor[m.view] {
		return selStyle.Render("> " + clip(text, m.w-2))
	}
	return "  " + clip(text, m.w-2)
}

func (m *Model) deviceLines() []string {
	if len(m.devices) == 0 {
		return []string{dimStyle.Render("no devices enrolled; enroll one at /enroll in the browser")}
	}
	lines := []string{headStyle.Render(clip(fmt.Sprintf("  %-18s %-14s %-16s %-16s %s", "NAME", "ID", "CREATED", "LAST SEEN", "STATE"), m.w))}
	s, e := m.window(len(m.devices))
	for i := s; i < e; i++ {
		d := m.devices[i]
		state := "active"
		if d.Revoked {
			state = "revoked"
		}
		lines = append(lines, m.row(i, fmt.Sprintf("%-18s %-14s %-16s %-16s %s",
			clip(d.Name, 18), clip(d.ID, 14), stamp(d.Created, "never"), stamp(d.LastSeen, "never"), state)))
	}
	return lines
}

func (m *Model) sessionLines() []string {
	if len(m.sessions) == 0 {
		return []string{dimStyle.Render("no active sessions")}
	}
	lines := []string{headStyle.Render(clip(fmt.Sprintf("  %-14s %-18s %-16s %-16s %s", "SESSION", "DEVICE", "IP", "CREATED", "EXPIRES"), m.w))}
	s, e := m.window(len(m.sessions))
	for i := s; i < e; i++ {
		x := m.sessions[i]
		lines = append(lines, m.row(i, fmt.Sprintf("%-14s %-18s %-16s %-16s %s",
			clip(x.ID, 14), clip(x.DeviceName, 18), clip(x.IP, 16), stamp(x.Created, "-"), stamp(x.Expires, "-"))))
	}
	return lines
}

func (m *Model) eventLines() []string {
	if len(m.events) == 0 {
		return []string{dimStyle.Render("no events yet")}
	}
	lines := []string{headStyle.Render(clip(fmt.Sprintf("  %-16s %-12s %-18s %-16s %s", "WHEN", "KIND", "DEVICE", "IP", "DETAIL"), m.w))}
	s, e := m.window(len(m.events))
	for i := s; i < e; i++ {
		x := m.events[i]
		lines = append(lines, m.row(i, fmt.Sprintf("%-16s %-12s %-18s %-16s %s",
			stamp(x.At, "-"), clip(x.Kind, 12), clip(x.Name, 18), clip(x.IP, 16), printable(x.Detail))))
	}
	return lines
}

func stamp(t time.Time, zero string) string {
	if t.IsZero() {
		return zero
	}
	return t.Local().Format("2006-01-02 15:04")
}

// clip shortens s to w runes, with control characters (escape sequences
// from a client-supplied name) shown as "?".
func clip(s string, w int) string {
	s = strings.Join(strings.Fields(printable(s)), " ")
	r := []rune(s)
	if w <= 0 {
		return ""
	}
	if len(r) > w {
		return string(r[:max(w-1, 0)]) + "…"
	}
	return s
}

func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != ' ' {
			return '?'
		}
		return r
	}, strings.ToValidUTF8(s, "?"))
}

// Program wraps Model as a full-screen tea.Model for standalone use.
type Program struct{ *Model }

func NewProgram(a Admin) Program { return Program{New(a)} }

func (p Program) Init() tea.Cmd { return p.Model.Init() }

func (p Program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.Resize(msg.Width, msg.Height-1)
		return p, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return p, tea.Quit
		}
	}
	cmd := p.Model.Update(msg)
	if p.Quit() {
		return p, tea.Quit
	}
	return p, cmd
}

func (p Program) View() tea.View {
	v := tea.NewView(p.Model.View() + "\n" + hintStyle.Render(clip(p.Hint()+"  q quit", p.w)))
	v.AltScreen = true
	return v
}
