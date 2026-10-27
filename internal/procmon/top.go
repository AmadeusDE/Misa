package procmon

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type TopViewer struct {
	width, height int
	prev, cur     Snapshot
	sortField     SortField
	scrollOffset  int
	filterUser    string
	Quit          bool
}

func NewTop() *TopViewer {
	t := &TopViewer{sortField: SortCPU}
	t.prev = TakeFullSnapshot()
	time.Sleep(300 * time.Millisecond)
	t.cur = TakeFullSnapshot()
	t.calcRates()
	return t
}

func (t *TopViewer) IsQuit() bool { return t.Quit }

func (t *TopViewer) Resize(w, h int) {
	t.width = w
	t.height = h
}

type topTickMsg struct{}

func topTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return topTickMsg{} })
}

func (t *TopViewer) Init() tea.Cmd { return topTick() }

func (t *TopViewer) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.Resize(msg.Width, msg.Height)
	case topTickMsg:
		t.refresh()
		return topTick()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			t.Quit = true
		case "P":
			t.sortField = SortCPU
			t.scrollOffset = 0
		case "M":
			t.sortField = SortMem
			t.scrollOffset = 0
		case "N":
			t.sortField = SortPID
			t.scrollOffset = 0
		case "T":
			t.sortField = SortTime
			t.scrollOffset = 0
		case "up", "k":
			if t.scrollOffset > 0 {
				t.scrollOffset--
			}
		case "down", "j":
			t.scrollOffset++
		case "pgup":
			t.scrollOffset -= t.procRows()
			if t.scrollOffset < 0 {
				t.scrollOffset = 0
			}
		case "pgdown":
			t.scrollOffset += t.procRows()
		case "home", "g":
			t.scrollOffset = 0
		}
	}
	return nil
}

func (t *TopViewer) refresh() {
	t.prev = t.cur
	t.cur = TakeFullSnapshot()
	t.calcRates()
}

func (t *TopViewer) calcRates() {
	elapsed := t.cur.Time.Sub(t.prev.Time).Seconds()
	CalcCPUPercent(t.prev.Procs, t.cur.Procs, elapsed)
	CalcMemPercent(t.cur.Procs, t.cur.Mem.Total)
}

func (t *TopViewer) procRows() int {
	h := t.height - 7
	if h < 1 {
		h = 1
	}
	return h
}

var (
	topHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fabd2f"))
	topLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598"))
	topValStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
)

func (t *TopViewer) View() string {
	if t.width == 0 || t.height == 0 {
		return "loading..."
	}

	procs := make([]Process, len(t.cur.Procs))
	copy(procs, t.cur.Procs)

	if t.filterUser != "" {
		var filtered []Process
		for _, p := range procs {
			if p.User == t.filterUser {
				filtered = append(filtered, p)
			}
		}
		procs = filtered
	}

	SortProcs(procs, t.sortField, false)
	total, running, sleeping, stopped, zombie := CountStates(procs)

	us, sy, ni, id, wa, hi, si, st := CalcCPUDelta(t.prev.CPUAll, t.cur.CPUAll)
	mem := t.cur.Mem

	var lines []string

	now := time.Now().Format("15:04:05")
	line1 := fmt.Sprintf("%s up %s, load average: %.2f, %.2f, %.2f",
		topHeaderStyle.Render("top - "+now),
		FmtUptime(t.cur.Uptime), t.cur.LoadAvg[0], t.cur.LoadAvg[1], t.cur.LoadAvg[2])
	lines = append(lines, TruncStr(line1, t.width))

	line2 := fmt.Sprintf("%s %d total, %d running, %d sleeping, %d stopped, %d zombie",
		topLabelStyle.Render("Tasks:"), total, running, sleeping, stopped, zombie)
	lines = append(lines, TruncStr(line2, t.width))

	line3 := fmt.Sprintf("%s %4.1f us, %4.1f sy, %4.1f ni, %4.1f id, %4.1f wa, %4.1f hi, %4.1f si, %4.1f st",
		topLabelStyle.Render("%CPU(s):"), us, sy, ni, id, wa, hi, si, st)
	lines = append(lines, TruncStr(line3, t.width))

	line4 := fmt.Sprintf("%s %9s total, %9s free, %9s used, %9s buff/cache",
		topLabelStyle.Render("MiB Mem:"),
		FmtBytes(mem.Total), FmtBytes(mem.Free), FmtBytes(mem.Used()), FmtBytes(mem.BuffCache()))
	lines = append(lines, TruncStr(line4, t.width))

	line5 := fmt.Sprintf("%s %9s total, %9s free, %9s used. %9s avail",
		topLabelStyle.Render("MiB Swap:"),
		FmtBytes(mem.SwapTotal), FmtBytes(mem.SwapFree), FmtBytes(mem.SwapUsed()), FmtBytes(mem.Available))
	lines = append(lines, TruncStr(line5, t.width))

	lines = append(lines, "")

	sortMarker := func(f SortField) string {
		if t.sortField == f {
			return topHeaderStyle.Render("*")
		}
		return " "
	}

	hdr := fmt.Sprintf(" %s%7s %-9s %3s %4s %7s %7s %7s %s %s%5s %s%5s %9s %s",
		sortMarker(SortPID), "PID", "USER", "PR", "NI",
		"VIRT", "RES", "SHR", "S",
		sortMarker(SortCPU), "%CPU",
		sortMarker(SortMem), "%MEM",
		"TIME+", "COMMAND")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Reverse(true).Width(t.width).Render(
		TruncStr(hdr, t.width)))

	rows := t.procRows()
	if t.scrollOffset >= len(procs) {
		t.scrollOffset = len(procs) - 1
	}
	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}

	end := t.scrollOffset + rows
	if end > len(procs) {
		end = len(procs)
	}

	for _, p := range procs[t.scrollOffset:end] {
		row := fmt.Sprintf("  %7d %-9s %3d %4d %7s %7s %7s %s %5.1f %5.1f %9s %s",
			p.PID, TruncStr(p.User, 9),
			p.Priority, p.Nice,
			FmtBytes(p.VSize), FmtBytes(p.RSSBytes()), FmtBytes(p.SHRBytes()),
			p.StateStr(),
			p.CPUPct, p.MemPct,
			FmtCPUTime(p.CPUTime()),
			p.Comm)
		lines = append(lines, TruncStr(row, t.width))
	}

	for len(lines) < t.height {
		lines = append(lines, "")
	}

	return strings.Join(lines[:t.height], "\n")
}

// Standalone runner

type topModel struct{ viewer *TopViewer }

func (m *topModel) Init() tea.Cmd { return m.viewer.Init() }

func (m *topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.viewer.Update(msg)
	if m.viewer.Quit {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *topModel) View() tea.View { return fullScreen(m.viewer.View()) }

func RunTop() error {
	v := NewTop()
	p := tea.NewProgram(&topModel{viewer: v})
	_, err := p.Run()
	return err
}
