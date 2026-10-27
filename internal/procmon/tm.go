package procmon

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type tmTab int

const (
	tabProcesses tmTab = iota
	tabPerformance
	tabHtop
	tabBtop
)

const tmTabCount = 4

type TMViewer struct {
	width, height int
	prev, cur     Snapshot
	tab           tmTab
	sortField     SortField
	scrollOffset  int
	selectedIdx   int
	treeView      bool
	htop          *HtopViewer
	btop          *BtopViewer
	Quit          bool
}

func NewTM() *TMViewer {
	t := &TMViewer{sortField: SortCPU}
	t.prev = TakeFullSnapshot()
	time.Sleep(300 * time.Millisecond)
	t.cur = TakeFullSnapshot()
	t.calcRates()
	t.htop = NewHtop()
	t.btop = NewBtop()
	return t
}

func (t *TMViewer) IsQuit() bool { return t.Quit }

func (t *TMViewer) Resize(w, h int) {
	t.width = w
	t.height = h
	contentH := h - 2
	if contentH < 1 {
		contentH = 1
	}
	t.htop.Resize(w, contentH)
	t.btop.Resize(w, contentH)
}

type tmTickMsg struct{}

func tmTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tmTickMsg{} })
}

func (t *TMViewer) Init() tea.Cmd { return tmTick() }

func (t *TMViewer) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.Resize(msg.Width, msg.Height)
	case tmTickMsg:
		t.refresh()
		t.htop.refresh()
		t.btop.refresh()
		return tmTick()
	case htopTickMsg, btopTickMsg:
		return nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "tab" {
			t.switchTab((t.tab + 1) % tmTabCount)
			return nil
		}
		if key == "shift+tab" {
			t.switchTab((t.tab + tmTabCount - 1) % tmTabCount)
			return nil
		}

		switch t.tab {
		case tabHtop:
			if key == "q" {
				t.Quit = true
				return nil
			}
			cmd := t.htop.Update(msg)
			if t.htop.Quit {
				t.htop.Quit = false
				t.switchTab(tabProcesses)
				return nil
			}
			return cmd
		case tabBtop:
			if key == "q" {
				t.Quit = true
				return nil
			}
			cmd := t.btop.Update(msg)
			if t.btop.Quit {
				t.btop.Quit = false
				t.switchTab(tabProcesses)
				return nil
			}
			return cmd
		default:
			return t.handleOwnKey(key)
		}
	}
	return nil
}

func (t *TMViewer) switchTab(tab tmTab) {
	t.tab = tab
	t.scrollOffset = 0
	t.selectedIdx = 0
}

func (t *TMViewer) handleOwnKey(key string) tea.Cmd {
	switch key {
	case "q", "esc":
		t.Quit = true
	case "1":
		t.switchTab(tabProcesses)
	case "2":
		t.switchTab(tabPerformance)
	case "3":
		t.switchTab(tabHtop)
	case "4":
		t.switchTab(tabBtop)
	case "c":
		t.sortField = SortCPU
	case "m":
		t.sortField = SortMem
	case "n":
		t.sortField = SortPID
	case "p":
		t.sortField = SortTime
	case "t":
		t.treeView = !t.treeView
	case "up", "k":
		if t.selectedIdx > 0 {
			t.selectedIdx--
		}
	case "down", "j":
		t.selectedIdx++
	case "pgup":
		t.selectedIdx -= 10
		if t.selectedIdx < 0 {
			t.selectedIdx = 0
		}
	case "pgdown":
		t.selectedIdx += 10
	case "home", "g":
		t.selectedIdx = 0
	case "end", "G":
		t.selectedIdx = len(t.cur.Procs) - 1
	}
	return nil
}

func (t *TMViewer) refresh() {
	t.prev = t.cur
	t.cur = TakeFullSnapshot()
	t.calcRates()
}

func (t *TMViewer) calcRates() {
	elapsed := t.cur.Time.Sub(t.prev.Time).Seconds()
	CalcCPUPercent(t.prev.Procs, t.cur.Procs, elapsed)
	CalcMemPercent(t.cur.Procs, t.cur.Mem.Total)
}

var (
	tmTabStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374")).Padding(0, 1)
	tmTabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ebdbb2")).
				Background(lipgloss.Color("#504945")).Padding(0, 1)
	tmSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#b8bb26"))
	tmDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	tmValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	tmAccentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598"))
	tmBarLow       = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	tmBarMed       = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f"))
	tmBarHigh      = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934"))
)

func (t *TMViewer) View() string {
	if t.width == 0 || t.height == 0 {
		return "loading..."
	}

	var lines []string

	tabs := []struct {
		label string
		tab   tmTab
	}{
		{"Processes", tabProcesses},
		{"Performance", tabPerformance},
		{"htop", tabHtop},
		{"btop", tabBtop},
	}

	var tabBar strings.Builder
	tabBar.WriteString("  ")
	for _, tb := range tabs {
		if tb.tab == t.tab {
			tabBar.WriteString(tmTabActiveStyle.Render(tb.label))
		} else {
			tabBar.WriteString(tmTabStyle.Render(tb.label))
		}
		tabBar.WriteString(" ")
	}
	tabBar.WriteString(tmDimStyle.Render("  [tab] switch  [q] quit"))
	lines = append(lines, TruncStr(tabBar.String(), t.width))
	lines = append(lines, strings.Repeat("─", t.width))

	switch t.tab {
	case tabProcesses:
		lines = append(lines, t.processView()...)
	case tabPerformance:
		lines = append(lines, t.performanceView()...)
	case tabHtop:
		lines = append(lines, strings.Split(t.htop.View(), "\n")...)
	case tabBtop:
		lines = append(lines, strings.Split(t.btop.View(), "\n")...)
	}

	for len(lines) < t.height {
		lines = append(lines, "")
	}
	if len(lines) > t.height {
		lines = lines[:t.height]
	}

	return strings.Join(lines, "\n")
}

func tmBar(width int, pct float64) string {
	if width <= 0 {
		return ""
	}
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	var style lipgloss.Style
	switch {
	case pct > 80:
		style = tmBarHigh
	case pct > 50:
		style = tmBarMed
	default:
		style = tmBarLow
	}
	return style.Render(strings.Repeat("█", filled)) +
		tmDimStyle.Render(strings.Repeat("░", width-filled))
}

type procCategory struct {
	name  string
	procs []Process
}

func categorizeProcs(procs []Process) []procCategory {
	var apps, background, system []Process
	for _, p := range procs {
		switch {
		case p.UID == 0 || p.PID <= 2 || strings.HasPrefix(p.Comm, "["):
			system = append(system, p)
		case p.HasTTY():
			apps = append(apps, p)
		default:
			background = append(background, p)
		}
	}
	var cats []procCategory
	if len(apps) > 0 {
		cats = append(cats, procCategory{"Apps", apps})
	}
	if len(background) > 0 {
		cats = append(cats, procCategory{"Background processes", background})
	}
	if len(system) > 0 {
		cats = append(cats, procCategory{"System processes", system})
	}
	return cats
}

func (t *TMViewer) processView() []string {
	procs := make([]Process, len(t.cur.Procs))
	copy(procs, t.cur.Procs)
	SortProcs(procs, t.sortField, false)

	cats := categorizeProcs(procs)

	cpuUsage := CalcCPUUsage(t.prev.CPUAll, t.cur.CPUAll)
	cpuText := fmt.Sprintf("%4.1f%%", cpuUsage)
	if !t.cur.CPUAvailable {
		cpuUsage = 0
		cpuText = unavailable
	}
	mem := t.cur.Mem
	memPct := 0.0
	if mem.Total > 0 {
		memPct = float64(mem.Used()) * 100 / float64(mem.Total)
	}

	elapsed := t.cur.Time.Sub(t.prev.Time).Seconds()
	diskRead, diskWrite := CalcDiskRates(t.prev.Disks, t.cur.Disks, elapsed)
	rxRate, txRate := CalcNetRates(t.prev.Net, t.cur.Net, elapsed)

	barW := 12

	var lines []string
	summaryLine := fmt.Sprintf("  CPU %s %s    Mem %s %4.1f%%    Disk R:%s W:%s    Net ▼%s ▲%s",
		tmBar(barW, cpuUsage), cpuText,
		tmBar(barW, memPct), memPct,
		FmtRate(diskRead), FmtRate(diskWrite),
		FmtRate(rxRate), FmtRate(txRate))
	lines = append(lines, TruncStr(summaryLine, t.width))
	lines = append(lines, "")

	sortInd := func(f SortField, label string) string {
		if t.sortField == f {
			return lipgloss.NewStyle().Bold(true).Underline(true).Render(label)
		}
		return label
	}

	hdr := fmt.Sprintf("  %-20s %s %s  %7s  %s",
		"Name",
		sortInd(SortCPU, " CPU%"),
		sortInd(SortMem, " MEM%"),
		"Memory",
		sortInd(SortPID, "PID"))
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#928374")).Render(
		TruncStr(hdr, t.width)))

	procH := t.height - 7
	if procH < 3 {
		procH = 3
	}

	var flatList []Process
	var sectionIdxs []int
	var sectionNames []string
	for _, cat := range cats {
		sectionIdxs = append(sectionIdxs, len(flatList))
		sectionNames = append(sectionNames, cat.name)
		flatList = append(flatList, cat.procs...)
	}

	if t.selectedIdx >= len(flatList) {
		t.selectedIdx = len(flatList) - 1
	}
	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}

	if t.selectedIdx < t.scrollOffset {
		t.scrollOffset = t.selectedIdx
	}
	if t.selectedIdx >= t.scrollOffset+procH {
		t.scrollOffset = t.selectedIdx - procH + 1
	}

	selectedStyle := lipgloss.NewStyle().Reverse(true)
	globalIdx := 0
	sectionPtr := 0

	for globalIdx < len(flatList) && len(lines)-5 < procH {
		if sectionPtr < len(sectionIdxs) && globalIdx == sectionIdxs[sectionPtr] {
			if globalIdx >= t.scrollOffset {
				lines = append(lines, tmSectionStyle.Render(
					fmt.Sprintf("  ▸ %s (%d)", sectionNames[sectionPtr],
						func() int {
							end := len(flatList)
							if sectionPtr+1 < len(sectionIdxs) {
								end = sectionIdxs[sectionPtr+1]
							}
							return end - sectionIdxs[sectionPtr]
						}())))
			}
			sectionPtr++
		}

		if globalIdx >= t.scrollOffset {
			p := flatList[globalIdx]
			row := fmt.Sprintf("  %-20s %5.1f %5.1f  %7s  %7d",
				TruncStr(p.Comm, 20),
				p.CPUPct, p.MemPct,
				FmtBytes(p.RSSBytes()),
				p.PID)
			row = TruncStr(row, t.width)
			if globalIdx == t.selectedIdx {
				row = selectedStyle.Width(t.width).Render(row)
			}
			lines = append(lines, row)
		}
		globalIdx++
	}

	total, running, sleeping, stopped, zombie := CountStates(t.cur.Procs)
	footer := tmDimStyle.Render(fmt.Sprintf(
		"  %d processes  %d running  %d sleeping  %d stopped  %d zombie   [c]pu [m]em [n]pid [t]ree",
		total, running, sleeping, stopped, zombie))
	lines = append(lines, "")
	lines = append(lines, TruncStr(footer, t.width))

	return lines
}

func (t *TMViewer) performanceView() []string {
	var lines []string

	elapsed := t.cur.Time.Sub(t.prev.Time).Seconds()
	cpuUsage := CalcCPUUsage(t.prev.CPUAll, t.cur.CPUAll)
	mem := t.cur.Mem

	barW := t.width - 30
	if barW < 10 {
		barW = 10
	}

	cpuText := fmt.Sprintf("%4.1f%%", cpuUsage)
	if !t.cur.CPUAvailable {
		cpuUsage = 0
		cpuText = unavailable
	}
	lines = append(lines, tmSectionStyle.Render("  CPU"))
	lines = append(lines, fmt.Sprintf("  Utilization  %s  %s", tmBar(barW, cpuUsage), cpuText))
	lines = append(lines, fmt.Sprintf("  Cores: %d    Load: %.2f %.2f %.2f    Uptime: %s",
		len(t.cur.CPUs), t.cur.LoadAvg[0], t.cur.LoadAvg[1], t.cur.LoadAvg[2],
		FmtUptime(t.cur.Uptime)))

	twoCol := len(t.cur.CPUs) > 4
	coreBarW := barW / 2
	if !twoCol {
		coreBarW = barW - 8
	}
	if coreBarW < 6 {
		coreBarW = 6
	}

	if twoCol {
		for i := 0; i < len(t.cur.CPUs); i += 2 {
			var prev0, prev1 CPUTimes
			if i < len(t.prev.CPUs) {
				prev0 = t.prev.CPUs[i]
			}
			pct0 := CalcCPUUsage(prev0, t.cur.CPUs[i])
			left := fmt.Sprintf("  %2d %s%4.0f%%", i, tmBar(coreBarW, pct0), pct0)

			right := ""
			if i+1 < len(t.cur.CPUs) {
				if i+1 < len(t.prev.CPUs) {
					prev1 = t.prev.CPUs[i+1]
				}
				pct1 := CalcCPUUsage(prev1, t.cur.CPUs[i+1])
				right = fmt.Sprintf("  %2d %s%4.0f%%", i+1, tmBar(coreBarW, pct1), pct1)
			}
			lines = append(lines, left+right)
		}
	} else {
		for i, cpu := range t.cur.CPUs {
			var prev CPUTimes
			if i < len(t.prev.CPUs) {
				prev = t.prev.CPUs[i]
			}
			pct := CalcCPUUsage(prev, cpu)
			lines = append(lines, fmt.Sprintf("  %2d %s %4.1f%%", i, tmBar(coreBarW, pct), pct))
		}
	}

	lines = append(lines, "")
	lines = append(lines, tmSectionStyle.Render("  MEMORY"))
	memPct := 0.0
	if mem.Total > 0 {
		memPct = float64(mem.Used()) * 100 / float64(mem.Total)
	}
	lines = append(lines, fmt.Sprintf("  In Use   %s  %s / %s (%4.1f%%)",
		tmBar(barW, memPct), FmtBytesShort(mem.Used()), FmtBytesShort(mem.Total), memPct))
	lines = append(lines, fmt.Sprintf("  Available: %s    Cached: %s    Buffers: %s",
		FmtBytesShort(mem.Available), FmtBytesShort(mem.Cached), FmtBytesShort(mem.Buffers)))

	swapPct := 0.0
	if mem.SwapTotal > 0 {
		swapPct = float64(mem.SwapUsed()) * 100 / float64(mem.SwapTotal)
	}
	lines = append(lines, fmt.Sprintf("  Swap     %s  %s / %s",
		tmBar(barW, swapPct), FmtBytesShort(mem.SwapUsed()), FmtBytesShort(mem.SwapTotal)))

	lines = append(lines, "")
	lines = append(lines, tmSectionStyle.Render("  DISK"))
	readRate, writeRate := CalcDiskRates(t.prev.Disks, t.cur.Disks, elapsed)
	lines = append(lines, fmt.Sprintf("  Read:  %s    Write: %s", FmtRate(readRate), FmtRate(writeRate)))
	for _, d := range t.cur.Disks[:min(3, len(t.cur.Disks))] {
		lines = append(lines, fmt.Sprintf("    %s  total: %s read, %s written",
			tmAccentStyle.Render(d.Name), FmtBytesShort(d.ReadBytes), FmtBytesShort(d.WriteBytes)))
	}

	lines = append(lines, "")
	lines = append(lines, tmSectionStyle.Render("  NETWORK"))
	rxRate, txRate := CalcNetRates(t.prev.Net, t.cur.Net, elapsed)
	lines = append(lines, fmt.Sprintf("  Receive: %s    Send: %s", FmtRate(rxRate), FmtRate(txRate)))
	for _, n := range t.cur.Net {
		lines = append(lines, fmt.Sprintf("    %s  ▼ %s  ▲ %s",
			tmAccentStyle.Render(n.Name), FmtBytesShort(n.RxBytes), FmtBytesShort(n.TxBytes)))
	}

	return lines
}

// Standalone

type tmModel struct{ viewer *TMViewer }

func (m *tmModel) Init() tea.Cmd { return m.viewer.Init() }

func (m *tmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.viewer.Update(msg)
	if m.viewer.Quit {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *tmModel) View() tea.View { return fullScreen(m.viewer.View()) }

func RunTM() error {
	v := NewTM()
	p := tea.NewProgram(&tmModel{viewer: v})
	_, err := p.Run()
	return err
}
