package procmon

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type htopMode int

const (
	htopNormal htopMode = iota
	htopHelp
	htopKill
	htopSearch
	htopFilter
)

var (
	sBarUser   = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	sBarSys    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934"))
	sBarNice   = lipgloss.NewStyle().Foreground(lipgloss.Color("#458588"))
	sBarWait   = lipgloss.NewStyle().Foreground(lipgloss.Color("#689d6a"))
	sBarMemUse = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	sBarMemBuf = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f"))
	sBarEmpty  = lipgloss.NewStyle().Foreground(lipgloss.Color("#504945"))
	sLabel     = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	sValue     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	sBold      = lipgloss.NewStyle().Bold(true)
	sReverse   = lipgloss.NewStyle().Reverse(true)
	sFnKey     = lipgloss.NewStyle().Bold(true).Reverse(true)
	sStateR    = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	sStateZ    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934"))
	sStateD    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fe8019"))
	sStateT    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f"))
	sSelected  = lipgloss.NewStyle().Reverse(true)
	sKillSel   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#fabd2f"))
	sStatusMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f"))

	sPID     = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	sUser    = lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598"))
	sUserR   = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true)
	sPriNi   = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	sMem     = lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598"))
	sTime    = lipgloss.NewStyle().Foreground(lipgloss.Color("#d3869b"))
	sComm    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	sCommK   = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	sTree    = lipgloss.NewStyle().Foreground(lipgloss.Color("#665c54"))
	sCPULow  = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	sCPUMed  = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f"))
	sCPUHigh = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true)
)

type HtopViewer struct {
	width, height int
	prev, cur     Snapshot
	sortField     SortField
	scrollOffset  int
	selectedIdx   int
	treeView      bool
	mode          htopMode
	killIdx       int
	killScroll    int
	searchBuf     string
	filterBuf     string
	statusMsg     string
	statusTimer   int
	Quit          bool
}

func NewHtop() *HtopViewer {
	h := &HtopViewer{sortField: SortCPU, killIdx: platformActions.defaultSignal()}
	h.prev = TakeFullSnapshot()
	time.Sleep(300 * time.Millisecond)
	h.cur = TakeFullSnapshot()
	h.calcRates()
	return h
}

func (h *HtopViewer) IsQuit() bool { return h.Quit }

func (h *HtopViewer) Resize(w, ht int) {
	h.width = w
	h.height = ht
}

type htopTickMsg struct{}

func htopTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return htopTickMsg{} })
}

func (h *HtopViewer) Init() tea.Cmd { return htopTick() }

func (h *HtopViewer) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.Resize(msg.Width, msg.Height)
	case htopTickMsg:
		h.refresh()
		if h.statusTimer > 0 {
			h.statusTimer--
			if h.statusTimer == 0 {
				h.statusMsg = ""
			}
		}
		return htopTick()
	case tea.KeyPressMsg:
		return h.handleKey(msg)
	}
	return nil
}

func (h *HtopViewer) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()

	switch h.mode {
	case htopHelp:
		h.mode = htopNormal
		return nil

	case htopKill:
		return h.handleKillKey(key)

	case htopSearch:
		return h.handleSearchKey(typedKey(msg))

	case htopFilter:
		return h.handleFilterKey(typedKey(msg))
	}

	switch key {
	case "q", "f10":
		h.Quit = true
	case "esc":
		if h.filterBuf != "" {
			h.filterBuf = ""
		} else {
			h.Quit = true
		}
	case "f1", "?":
		h.mode = htopHelp
	case "f3", "/":
		h.mode = htopSearch
		h.searchBuf = ""
	case "f4", "\\":
		h.mode = htopFilter
	case "f5", "t":
		h.treeView = !h.treeView
		h.scrollOffset = 0
	case "f6", "<", ">":
		h.cycleSort()
	case "f7":
		h.adjustNice(-1)
	case "f8":
		h.adjustNice(1)
	case "f9", "k":
		h.mode = htopKill
		h.killIdx = platformActions.defaultSignal()
		h.killScroll = 0
	case "P":
		h.sortField = SortCPU
	case "M":
		h.sortField = SortMem
	case "T":
		h.sortField = SortTime
	case "N":
		h.sortField = SortPID
	case "up":
		if h.selectedIdx > 0 {
			h.selectedIdx--
		}
	case "down", "j":
		h.selectedIdx++
	case "pgup":
		h.selectedIdx -= h.procRows()
		if h.selectedIdx < 0 {
			h.selectedIdx = 0
		}
	case "pgdown":
		h.selectedIdx += h.procRows()
	case "home", "g":
		h.selectedIdx = 0
	case "end", "G":
		h.selectedIdx = len(h.cur.Procs) - 1
	}
	return nil
}

func (h *HtopViewer) handleKillKey(key string) tea.Cmd {
	switch key {
	case "esc":
		h.mode = htopNormal
	case "up", "k":
		if h.killIdx > 0 {
			h.killIdx--
		}
	case "down", "j":
		if h.killIdx < len(platformActions.Signals)-1 {
			h.killIdx++
		}
	case "pgup":
		h.killIdx -= 10
		if h.killIdx < 0 {
			h.killIdx = 0
		}
	case "pgdown":
		h.killIdx += 10
		if h.killIdx >= len(platformActions.Signals) {
			h.killIdx = len(platformActions.Signals) - 1
		}
	case "enter":
		h.sendSignal()
		h.mode = htopNormal
	}
	return nil
}

func (h *HtopViewer) handleSearchKey(key string) tea.Cmd {
	switch key {
	case "esc":
		h.mode = htopNormal
		h.searchBuf = ""
	case "enter":
		h.mode = htopNormal
	case "backspace", "ctrl+h":
		if len(h.searchBuf) > 0 {
			h.searchBuf = h.searchBuf[:len(h.searchBuf)-1]
		}
	default:
		if len([]rune(key)) == 1 {
			h.searchBuf += key
		}
	}
	if h.searchBuf != "" {
		h.jumpToSearch()
	}
	return nil
}

func (h *HtopViewer) handleFilterKey(key string) tea.Cmd {
	switch key {
	case "esc":
		h.mode = htopNormal
	case "enter":
		h.mode = htopNormal
	case "backspace", "ctrl+h":
		if len(h.filterBuf) > 0 {
			h.filterBuf = h.filterBuf[:len(h.filterBuf)-1]
		} else {
			h.mode = htopNormal
		}
	default:
		if len([]rune(key)) == 1 {
			h.filterBuf += key
		}
	}
	return nil
}

func (h *HtopViewer) jumpToSearch() {
	procs := h.getFilteredProcs()
	needle := strings.ToLower(h.searchBuf)
	for i, p := range procs {
		if strings.Contains(strings.ToLower(p.Comm), needle) ||
			strings.Contains(strings.ToLower(p.Cmdline), needle) {
			h.selectedIdx = i
			return
		}
	}
}

func (h *HtopViewer) sendSignal() {
	procs := h.getFilteredProcs()
	if h.selectedIdx < 0 || h.selectedIdx >= len(procs) {
		return
	}
	sig := platformActions.Signals[h.killIdx]
	if sig.sig == 0 {
		h.statusMsg = "Cancelled"
		h.statusTimer = 2
		return
	}
	pid := procs[h.selectedIdx].PID
	err := platformActions.Send(pid, sig)
	if err != nil {
		h.statusMsg = fmt.Sprintf("Kill %d with %s: %v", pid, sig.name, err)
	} else {
		h.statusMsg = fmt.Sprintf("Sent %s to PID %d", sig.name, pid)
	}
	h.statusTimer = 3
}

func (h *HtopViewer) adjustNice(delta int) {
	procs := h.getFilteredProcs()
	if h.selectedIdx < 0 || h.selectedIdx >= len(procs) {
		return
	}
	p := procs[h.selectedIdx]
	newNice := int(p.Nice) + delta
	if newNice < -20 {
		newNice = -20
	}
	if newNice > 19 {
		newNice = 19
	}
	err := platformActions.Renice(p.PID, newNice)
	if err != nil {
		h.statusMsg = fmt.Sprintf("renice PID %d: %v", p.PID, err)
	} else {
		h.statusMsg = fmt.Sprintf("Set PID %d nice to %d", p.PID, newNice)
	}
	h.statusTimer = 3
}

func (h *HtopViewer) cycleSort() {
	switch h.sortField {
	case SortCPU:
		h.sortField = SortMem
	case SortMem:
		h.sortField = SortPID
	case SortPID:
		h.sortField = SortTime
	case SortTime:
		h.sortField = SortCommand
	default:
		h.sortField = SortCPU
	}
}

func (h *HtopViewer) refresh() {
	h.prev = h.cur
	h.cur = TakeFullSnapshot()
	h.calcRates()
}

func (h *HtopViewer) calcRates() {
	elapsed := h.cur.Time.Sub(h.prev.Time).Seconds()
	CalcCPUPercent(h.prev.Procs, h.cur.Procs, elapsed)
	CalcMemPercent(h.cur.Procs, h.cur.Mem.Total)
}

func (h *HtopViewer) procRows() int {
	headerRows := h.headerHeight() + 2
	footerRows := 2
	r := h.height - headerRows - footerRows
	if r < 1 {
		r = 1
	}
	return r
}

func (h *HtopViewer) headerHeight() int {
	cpuRows := len(h.cur.CPUs)
	if cpuRows == 0 && !h.cur.CPUAvailable {
		cpuRows = 1
	}
	if h.width >= 80 && cpuRows > 1 {
		cpuRows = (cpuRows + 1) / 2
	}
	return cpuRows + 2
}

func (h *HtopViewer) getFilteredProcs() []Process {
	procs := make([]Process, len(h.cur.Procs))
	copy(procs, h.cur.Procs)

	if h.filterBuf != "" {
		needle := strings.ToLower(h.filterBuf)
		var filtered []Process
		for _, p := range procs {
			if strings.Contains(strings.ToLower(p.Comm), needle) ||
				strings.Contains(strings.ToLower(p.Cmdline), needle) {
				filtered = append(filtered, p)
			}
		}
		procs = filtered
	}

	if h.treeView {
		SortProcs(procs, SortPID, false)
		procs = BuildTree(procs)
	} else {
		SortProcs(procs, h.sortField, false)
	}
	return procs
}

func htopCPUBar(prev, cur CPUTimes, barWidth int) string {
	us, sy, ni, _, wa, _, _, _ := CalcCPUDelta(prev, cur)
	total := us + sy + ni + wa

	var b strings.Builder
	b.WriteString("[")
	used := 0
	for _, seg := range []struct {
		pct   float64
		style lipgloss.Style
	}{
		{us, sBarUser},
		{sy, sBarSys},
		{ni, sBarNice},
		{wa, sBarWait},
	} {
		n := int(seg.pct / 100.0 * float64(barWidth))
		if n+used > barWidth {
			n = barWidth - used
		}
		if n > 0 {
			b.WriteString(seg.style.Render(strings.Repeat("|", n)))
			used += n
		}
	}
	if used < barWidth {
		b.WriteString(sBarEmpty.Render(strings.Repeat(" ", barWidth-used)))
	}
	b.WriteString("]")
	b.WriteString(fmt.Sprintf(" %4.1f%%", total))
	return b.String()
}

func htopMemBar(label string, used, buffCache, total uint64, barWidth int) string {
	if total == 0 {
		return label + "[" + strings.Repeat(" ", barWidth) + "]"
	}
	usedPct := float64(used) * 100.0 / float64(total)
	bcPct := float64(buffCache) * 100.0 / float64(total)

	var b strings.Builder
	b.WriteString(label)
	b.WriteString("[")
	filled := 0
	usedN := int(usedPct / 100.0 * float64(barWidth))
	if usedN > barWidth {
		usedN = barWidth
	}
	b.WriteString(sBarMemUse.Render(strings.Repeat("|", usedN)))
	filled += usedN

	bcN := int(bcPct / 100.0 * float64(barWidth))
	if bcN+filled > barWidth {
		bcN = barWidth - filled
	}
	if bcN > 0 {
		b.WriteString(sBarMemBuf.Render(strings.Repeat("|", bcN)))
		filled += bcN
	}

	if filled < barWidth {
		b.WriteString(sBarEmpty.Render(strings.Repeat(" ", barWidth-filled)))
	}
	b.WriteString("]")
	b.WriteString(fmt.Sprintf(" %s/%s", FmtBytes(used+buffCache), FmtBytes(total)))
	return b.String()
}

func (h *HtopViewer) View() string {
	if h.width == 0 || h.height == 0 {
		return "loading..."
	}

	if h.mode == htopHelp {
		return h.helpView()
	}

	var lines []string

	barWidth := h.width - 20
	if barWidth < 10 {
		barWidth = 10
	}

	twoCol := h.width >= 80 && len(h.cur.CPUs) > 1
	if !h.cur.CPUAvailable {
		lines = append(lines, sLabel.Render("CPU ")+unavailable)
	} else if twoCol {
		colW := (h.width - 4) / 2
		cbarW := colW - 12
		if cbarW < 8 {
			cbarW = 8
		}
		for i := 0; i < len(h.cur.CPUs); i += 2 {
			var prevL, prevR CPUTimes
			if i < len(h.prev.CPUs) {
				prevL = h.prev.CPUs[i]
			}
			leftLabel := sLabel.Render(fmt.Sprintf("%2d", i))
			left := leftLabel + htopCPUBar(prevL, h.cur.CPUs[i], cbarW)

			right := ""
			if i+1 < len(h.cur.CPUs) {
				if i+1 < len(h.prev.CPUs) {
					prevR = h.prev.CPUs[i+1]
				}
				rightLabel := sLabel.Render(fmt.Sprintf("%2d", i+1))
				right = rightLabel + htopCPUBar(prevR, h.cur.CPUs[i+1], cbarW)
			}

			leftPad := colW - lipgloss.Width(left)
			if leftPad < 1 {
				leftPad = 1
			}
			row := left + strings.Repeat(" ", leftPad) + right
			lines = append(lines, row)
		}
	} else {
		cbarW := barWidth - 4
		if cbarW < 8 {
			cbarW = 8
		}
		for i, cpu := range h.cur.CPUs {
			var prev CPUTimes
			if i < len(h.prev.CPUs) {
				prev = h.prev.CPUs[i]
			}
			row := sLabel.Render(fmt.Sprintf("%2d", i)) + htopCPUBar(prev, cpu, cbarW)
			lines = append(lines, row)
		}
	}

	mem := h.cur.Mem
	memBarW := barWidth
	if memBarW > h.width-30 {
		memBarW = h.width - 30
	}
	if memBarW < 8 {
		memBarW = 8
	}
	lines = append(lines, htopMemBar(sLabel.Render("Mem"), mem.Used(), mem.BuffCache(), mem.Total, memBarW))
	lines = append(lines, htopMemBar(sLabel.Render("Swp"), mem.SwapUsed(), 0, mem.SwapTotal, memBarW))

	total, running, _, _, _ := CountStates(h.cur.Procs)
	thrCount := 0
	for _, p := range h.cur.Procs {
		thrCount += p.Threads
	}
	taskLine := fmt.Sprintf("Tasks: "+sBold.Render("%d")+", %d thr; "+sBarUser.Render("%d running"),
		total, thrCount, running)
	loadLine := fmt.Sprintf("Load average: "+sBold.Render("%.2f")+" %.2f %.2f   Uptime: %s",
		h.cur.LoadAvg[0], h.cur.LoadAvg[1], h.cur.LoadAvg[2],
		FmtUptime(h.cur.Uptime))
	lines = append(lines, "  "+taskLine+"  "+loadLine)

	lines = append(lines, "")

	if h.mode == htopKill {
		lines = append(lines, h.killView()...)
	} else {
		lines = append(lines, h.processView()...)
	}

	for len(lines) < h.height-1 {
		lines = append(lines, "")
	}

	lines = append(lines, h.footerLine())

	if len(lines) > h.height {
		lines = lines[:h.height]
	}

	return strings.Join(lines, "\n")
}

func (h *HtopViewer) processView() []string {
	var lines []string

	sortLabel := func(f SortField, name string) string {
		if h.sortField == f {
			return sBold.Underline(true).Render(name)
		}
		return name
	}

	hdr := fmt.Sprintf("  %s %-8s %3s  %3s %7s %7s %7s %s %s %s %8s %s",
		sortLabel(SortPID, "PID"), "USER", "PRI", "NI",
		"VIRT", "RES", "SHR", "S",
		sortLabel(SortCPU, "CPU%"),
		sortLabel(SortMem, "MEM%"),
		sortLabel(SortTime, "TIME+"),
		sortLabel(SortCommand, "Command"))
	lines = append(lines, sReverse.Width(h.width).Render(hdr))

	procs := h.getFilteredProcs()

	if h.selectedIdx >= len(procs) {
		h.selectedIdx = len(procs) - 1
	}
	if h.selectedIdx < 0 {
		h.selectedIdx = 0
	}

	rows := h.procRows()
	if h.selectedIdx < h.scrollOffset {
		h.scrollOffset = h.selectedIdx
	}
	if h.selectedIdx >= h.scrollOffset+rows {
		h.scrollOffset = h.selectedIdx - rows + 1
	}

	end := h.scrollOffset + rows
	if end > len(procs) {
		end = len(procs)
	}

	for i := h.scrollOffset; i < end; i++ {
		p := procs[i]
		if i == h.selectedIdx {
			row := formatProcRowPlain(p, h.width)
			lines = append(lines, sSelected.Width(h.width).Render(row))
		} else {
			lines = append(lines, formatProcRowColored(p, h.width))
		}
	}

	return lines
}

func formatProcRowPlain(p Process, width int) string {
	comm := p.Comm
	maxComm := width - 68
	if maxComm < 8 {
		maxComm = 8
	}
	if len(comm) > maxComm {
		comm = comm[:maxComm]
	}
	return fmt.Sprintf("%7d %-8s %3d  %3d %7s %7s %7s %s %4.1f %4.1f %8s %s",
		p.PID, TruncStr(p.User, 8),
		p.Priority, p.Nice,
		FmtBytes(p.VSize), FmtBytes(p.RSSBytes()), FmtBytes(p.SHRBytes()),
		p.StateStr(),
		p.CPUPct, p.MemPct,
		FmtCPUTime(p.CPUTime()),
		comm)
}

func cpuColor(pct float64) lipgloss.Style {
	switch {
	case pct >= 50:
		return sCPUHigh
	case pct >= 15:
		return sCPUMed
	default:
		return sCPULow
	}
}

func formatProcRowColored(p Process, width int) string {
	comm := p.Comm
	maxComm := width - 68
	if maxComm < 8 {
		maxComm = 8
	}
	if len(comm) > maxComm {
		comm = comm[:maxComm]
	}

	userStyle := sUser
	if p.UID == 0 {
		userStyle = sUserR
	}

	var stateStyle lipgloss.Style
	switch p.State {
	case 'R':
		stateStyle = sStateR
	case 'Z':
		stateStyle = sStateZ
	case 'D':
		stateStyle = sStateD
	case 'T', 't':
		stateStyle = sStateT
	default:
		stateStyle = sLabel
	}

	commStyle := sComm
	if strings.HasPrefix(p.Comm, "[") || strings.HasPrefix(p.Comm, "├─[") || strings.HasPrefix(p.Comm, "│") {
		commStyle = sCommK
	}

	treePrefix := ""
	name := comm
	for _, pfx := range []string{"├─", "│ "} {
		if idx := strings.LastIndex(comm, pfx); idx >= 0 {
			treePrefix = sTree.Render(comm[:idx+len(pfx)])
			name = comm[idx+len(pfx):]
			break
		}
	}

	var b strings.Builder
	b.WriteString(sPID.Render(fmt.Sprintf("%7d", p.PID)))
	b.WriteString(" ")
	b.WriteString(userStyle.Render(fmt.Sprintf("%-8s", TruncStr(p.User, 8))))
	b.WriteString(" ")
	b.WriteString(sPriNi.Render(fmt.Sprintf("%3d  %3d", p.Priority, p.Nice)))
	b.WriteString(" ")
	b.WriteString(sMem.Render(fmt.Sprintf("%7s %7s %7s", FmtBytes(p.VSize), FmtBytes(p.RSSBytes()), FmtBytes(p.SHRBytes()))))
	b.WriteString(" ")
	b.WriteString(stateStyle.Render(p.StateStr()))
	b.WriteString(" ")
	b.WriteString(cpuColor(p.CPUPct).Render(fmt.Sprintf("%4.1f", p.CPUPct)))
	b.WriteString(" ")
	b.WriteString(cpuColor(p.MemPct).Render(fmt.Sprintf("%4.1f", p.MemPct)))
	b.WriteString(" ")
	b.WriteString(sTime.Render(fmt.Sprintf("%8s", FmtCPUTime(p.CPUTime()))))
	b.WriteString(" ")
	if treePrefix != "" {
		b.WriteString(treePrefix)
	}
	b.WriteString(commStyle.Render(name))

	return b.String()
}

func (h *HtopViewer) killView() []string {
	var lines []string

	procs := h.getFilteredProcs()
	targetPID := 0
	targetComm := ""
	if h.selectedIdx >= 0 && h.selectedIdx < len(procs) {
		targetPID = procs[h.selectedIdx].PID
		targetComm = procs[h.selectedIdx].Comm
	}

	hdrText := fmt.Sprintf(" Send signal to PID %d (%s):", targetPID, targetComm)
	hdr := fmt.Sprintf("%-*s  %s", 18, "Send signal:", hdrText)
	lines = append(lines, sReverse.Width(h.width).Render(TruncStr(hdr, h.width)))

	rows := h.procRows()

	if h.killIdx < h.killScroll {
		h.killScroll = h.killIdx
	}
	if h.killIdx >= h.killScroll+rows {
		h.killScroll = h.killIdx - rows + 1
	}

	end := h.killScroll + rows
	if end > len(platformActions.Signals) {
		end = len(platformActions.Signals)
	}

	for i := h.killScroll; i < end; i++ {
		sig := platformActions.Signals[i]
		row := fmt.Sprintf(" %2d %-12s", sig.num, sig.name)

		if i == h.killIdx {
			lines = append(lines, sKillSel.Width(h.width).Render(row))
		} else {
			lines = append(lines, row)
		}
	}

	return lines
}

func (h *HtopViewer) footerLine() string {
	if h.mode == htopSearch {
		return sFnKey.Render("Search: ") + h.searchBuf + sValue.Render("_")
	}
	if h.mode == htopFilter {
		return sFnKey.Render("Filter: ") + h.filterBuf + sValue.Render("_")
	}
	if h.mode == htopKill {
		return sFnKey.Render("Enter") + "Send   " + sFnKey.Render("Esc") + "Cancel"
	}
	if h.statusMsg != "" {
		return sStatusMsg.Render(h.statusMsg)
	}

	var b strings.Builder
	for _, item := range []struct{ key, label string }{
		{"F1", "Help"},
		{"F3", "Search"},
		{"F4", "Filter"},
		{"F5", "Tree"},
		{"F6", "SortBy"},
		{"-F7", "Nice"},
		{"+F8", "Nice"},
		{"F9", "Kill"},
		{"F10", "Quit"},
	} {
		b.WriteString(sFnKey.Render(item.key))
		b.WriteString(item.label)
	}

	extra := ""
	if h.treeView {
		extra = sLabel.Render("  [tree]")
	}
	if h.filterBuf != "" {
		extra += sLabel.Render("  filter:" + h.filterBuf)
	}

	return b.String() + extra
}

func (h *HtopViewer) helpView() string {
	title := sBold.Render("htop help")
	help := []string{
		title,
		"",
		"  q / F10     Quit",
		"  F1 / ?      This help",
		"  F3 / /      Search processes",
		"  F4 / \\      Filter processes",
		"  F5 / t      Toggle tree view",
		"  F6 / < >    Cycle sort column",
		"  -F7         Raise priority (lower nice)",
		"  +F8         Lower priority (higher nice)",
		"  F9 / k      Kill selected process",
		"  P           Sort by CPU%",
		"  M           Sort by MEM%",
		"  T           Sort by TIME+",
		"  N           Sort by PID",
		"  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598")).Render("up") + " / " + lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598")).Render("down") + "    Navigate process list",
		"  PgUp/PgDn   Scroll page",
		"  Home/End    Jump to start/end",
		"",
		sLabel.Render("  Press any key to close help"),
	}
	var lines []string
	lines = append(lines, help...)
	for len(lines) < h.height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:h.height], "\n")
}

// Standalone

type htopModel struct{ viewer *HtopViewer }

func (m *htopModel) Init() tea.Cmd { return m.viewer.Init() }

func (m *htopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.viewer.Update(msg)
	if m.viewer.Quit {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *htopModel) View() tea.View { return fullScreen(m.viewer.View()) }

func RunHtop() error {
	v := NewHtop()
	p := tea.NewProgram(&htopModel{viewer: v})
	_, err := p.Run()
	return err
}
