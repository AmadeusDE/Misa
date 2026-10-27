package procmon

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// BtopViewer is a btop-inspired resource monitor: a full-width CPU panel on
// top, mem/disks/net panels stacked on the left below it, and a process list
// filling the right column.
type BtopViewer struct {
	width, height int
	prev, cur     Snapshot
	sortField     SortField
	scrollOffset  int
	selectedIdx   int
	treeView      bool
	filterBuf     string
	filterMode    bool
	Quit          bool
}

func NewBtop() *BtopViewer {
	b := &BtopViewer{sortField: SortCPU}
	b.prev = TakeFullSnapshot()
	time.Sleep(300 * time.Millisecond)
	b.cur = TakeFullSnapshot()
	b.calcRates()
	return b
}

func (b *BtopViewer) IsQuit() bool { return b.Quit }

func (b *BtopViewer) Resize(w, h int) {
	b.width = w
	b.height = h
}

type btopTickMsg struct{}

func btopTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return btopTickMsg{} })
}

func (b *BtopViewer) Init() tea.Cmd { return btopTick() }

func (b *BtopViewer) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.Resize(msg.Width, msg.Height)
	case btopTickMsg:
		b.refresh()
		return btopTick()
	case tea.KeyPressMsg:
		if b.filterMode {
			b.handleFilterKey(msg)
			return nil
		}
		b.handleKey(msg)
	}
	return nil
}

func (b *BtopViewer) handleFilterKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "esc", "enter":
		b.filterMode = false
	case "ctrl+c":
		b.Quit = true
	case "backspace":
		if len(b.filterBuf) > 0 {
			r := []rune(b.filterBuf)
			b.filterBuf = string(r[:len(r)-1])
		}
	default:
		b.filterBuf += msg.Text
	}
}

func (b *BtopViewer) handleKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "q", "ctrl+c":
		b.Quit = true
	case "esc":
		if b.filterBuf != "" {
			b.filterBuf = ""
		} else {
			b.Quit = true
		}
	case "1":
		b.sortField = SortCPU
	case "2":
		b.sortField = SortMem
	case "3":
		b.sortField = SortPID
	case "t":
		b.treeView = !b.treeView
	case "f":
		b.filterMode = true
	case "K":
		b.killSelected()
	case "up", "k":
		if b.selectedIdx > 0 {
			b.selectedIdx--
		}
	case "down", "j":
		b.selectedIdx++
	case "pgup":
		b.selectedIdx -= 10
		if b.selectedIdx < 0 {
			b.selectedIdx = 0
		}
	case "pgdown":
		b.selectedIdx += 10
	case "home", "g":
		b.selectedIdx = 0
	case "end", "G":
		b.selectedIdx = len(b.filteredProcs()) - 1
	}
}

func (b *BtopViewer) killSelected() {
	procs := b.filteredProcs()
	if b.selectedIdx < 0 || b.selectedIdx >= len(procs) {
		return
	}
	_ = syscall.Kill(procs[b.selectedIdx].PID, syscall.SIGTERM)
}

func (b *BtopViewer) refresh() {
	b.prev = b.cur
	b.cur = TakeFullSnapshot()
	b.calcRates()
}

func (b *BtopViewer) calcRates() {
	elapsed := b.cur.Time.Sub(b.prev.Time).Seconds()
	CalcCPUPercent(b.prev.Procs, b.cur.Procs, elapsed)
	CalcMemPercent(b.cur.Procs, b.cur.Mem.Total)
}

// filteredProcs returns the current process list, filtered by filterBuf (if
// set) and sorted/tree-shaped according to the active view.
func (b *BtopViewer) filteredProcs() []Process {
	procs := make([]Process, len(b.cur.Procs))
	copy(procs, b.cur.Procs)

	if b.filterBuf != "" {
		needle := strings.ToLower(b.filterBuf)
		filtered := procs[:0:0]
		for _, p := range procs {
			if strings.Contains(strings.ToLower(p.Comm), needle) ||
				strings.Contains(strings.ToLower(p.Cmdline), needle) ||
				strings.Contains(strings.ToLower(p.User), needle) {
				filtered = append(filtered, p)
			}
		}
		procs = filtered
	}

	if b.treeView {
		SortProcs(procs, SortPID, false)
		procs = BuildTree(procs)
	} else {
		SortProcs(procs, b.sortField, false)
	}
	return procs
}

// Gruvbox palette, per the design brief.
var (
	btopBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#504945"))
	btopTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#b8bb26"))
	btopLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	btopValueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ebdbb2"))
	btopAccentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598"))
	btopPurpleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#d3869b"))
	btopGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26"))
	btopYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#fabd2f"))
	btopRedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934"))
	btopDimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	btopSelectStyle = lipgloss.NewStyle().Reverse(true)
)

func btopThresholdColor(pct float64) lipgloss.Style {
	switch {
	case pct > 80:
		return btopRedStyle
	case pct > 50:
		return btopYellowStyle
	default:
		return btopGreenStyle
	}
}

func btopBarThreshold(width int, pct float64) string {
	return ThresholdBar(width, pct, btopGreenStyle, btopYellowStyle, btopRedStyle, btopDimStyle)
}

// btopPad pads (or truncates) s to an exact visible width, ANSI-aware.
func btopPad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w > width {
		return TruncStr(s, width)
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// btopBox draws a bordered, titled box exactly width x height cells, filling
// body content (already-styled lines) into the interior, padding/truncating
// as needed.
func btopBox(title string, width, height int, body []string) []string {
	if width < 6 {
		width = 6
	}
	if height < 3 {
		height = 3
	}
	innerW := width - 2
	innerH := height - 2

	maxTitleW := width - 5
	if maxTitleW < 0 {
		maxTitleW = 0
	}
	if lipgloss.Width(title) > maxTitleW {
		title = TruncStr(title, maxTitleW)
	}
	dashes := width - lipgloss.Width(title) - 5
	if dashes < 0 {
		dashes = 0
	}

	top := btopBorderStyle.Render("┌─ ") + btopTitleStyle.Render(title) +
		btopBorderStyle.Render(" "+strings.Repeat("─", dashes)+"┐")
	bottom := btopBorderStyle.Render("└" + strings.Repeat("─", innerW) + "┘")
	side := btopBorderStyle.Render("│")

	lines := make([]string, 0, height)
	lines = append(lines, top)
	for i := 0; i < innerH; i++ {
		content := ""
		if i < len(body) {
			content = body[i]
		}
		lines = append(lines, side+btopPad(content, innerW)+side)
	}
	lines = append(lines, bottom)
	return lines
}

// btopJoinH concatenates same-height box renders left-to-right.
func btopJoinH(boxes ...[]string) []string {
	h := 0
	for _, box := range boxes {
		if len(box) > h {
			h = len(box)
		}
	}
	lines := make([]string, h)
	for i := 0; i < h; i++ {
		var sb strings.Builder
		for _, box := range boxes {
			if i < len(box) {
				sb.WriteString(box[i])
			}
		}
		lines[i] = sb.String()
	}
	return lines
}

func (b *BtopViewer) View() string {
	if b.width <= 0 || b.height <= 0 {
		return "loading..."
	}
	w, h := b.width, b.height

	numCores := len(b.cur.CPUs)
	twoCol := numCores > 4
	coreRows := numCores
	if twoCol {
		coreRows = (numCores + 1) / 2
	}
	cpuInner := 1 + coreRows
	if cpuInner < 4 {
		cpuInner = 4
	}
	cpuBoxH := cpuInner + 2
	maxCPU := h * 2 / 5
	if maxCPU < 6 {
		maxCPU = 6
	}
	if cpuBoxH > maxCPU {
		cpuBoxH = maxCPU
	}
	if cpuBoxH > h-6 {
		cpuBoxH = h - 6
	}
	if cpuBoxH < 4 {
		cpuBoxH = 4
	}

	bottomH := h - cpuBoxH
	if bottomH < 4 {
		bottomH = 4
	}

	leftW := w * 2 / 5
	if leftW < 24 {
		leftW = 24
	}
	if leftW > w-20 {
		leftW = w - 20
	}
	if leftW < 10 {
		leftW = w / 2
	}
	rightW := w - leftW

	netBoxH := bottomH / 3
	if netBoxH < 5 {
		netBoxH = 5
	}
	if netBoxH > bottomH-4 {
		netBoxH = bottomH - 4
	}
	if netBoxH < 3 {
		netBoxH = 3
	}
	memDisksH := bottomH - netBoxH
	if memDisksH < 3 {
		memDisksH = 3
	}

	memW := leftW / 2
	disksW := leftW - memW

	cpuBox := b.renderCPUBox(w, cpuBoxH)
	memBox := b.renderMemBox(memW, memDisksH)
	disksBox := b.renderDiskBox(disksW, memDisksH)
	netBox := b.renderNetBox(leftW, netBoxH)

	memDisksRow := btopJoinH(memBox, disksBox)
	left := append([]string{}, memDisksRow...)
	left = append(left, netBox...)

	procBox := b.renderProcBox(rightW, len(left))

	bottomRow := btopJoinH(left, procBox)

	all := append([]string{}, cpuBox...)
	all = append(all, bottomRow...)

	for len(all) < h {
		all = append(all, "")
	}
	if len(all) > h {
		all = all[:h]
	}
	for i, l := range all {
		all[i] = TruncStr(l, w)
	}
	return strings.Join(all, "\n")
}

func (b *BtopViewer) renderCPUBox(width, height int) []string {
	innerW := width - 2
	if innerW < 10 {
		innerW = 10
	}

	rightW := innerW / 3
	if rightW > 28 {
		rightW = 28
	}
	if rightW < 16 {
		rightW = 16
	}
	leftW := innerW - rightW - 1
	if leftW < 8 {
		leftW = 8
	}

	totalUsage := CalcCPUUsage(b.prev.CPUAll, b.cur.CPUAll)
	barW := leftW - 10
	if barW < 4 {
		barW = 4
	}
	var left []string
	left = append(left, fmt.Sprintf("%s %s %s",
		btopLabelStyle.Render("CPU"),
		btopBarThreshold(barW, totalUsage),
		btopThresholdColor(totalUsage).Render(fmt.Sprintf("%5.1f%%", totalUsage))))

	numCores := len(b.cur.CPUs)
	twoCol := numCores > 4
	corePct := func(i int) float64 {
		var prev CPUTimes
		if i < len(b.prev.CPUs) {
			prev = b.prev.CPUs[i]
		}
		return CalcCPUUsage(prev, b.cur.CPUs[i])
	}

	if twoCol {
		colW := leftW/2 - 10
		if colW < 3 {
			colW = 3
		}
		for i := 0; i < numCores; i += 2 {
			pct0 := corePct(i)
			line := fmt.Sprintf("%s %s%4.0f%%",
				btopLabelStyle.Render(fmt.Sprintf("C%-2d", i)),
				btopBarThreshold(colW, pct0), pct0)
			if i+1 < numCores {
				pct1 := corePct(i + 1)
				line += "  " + fmt.Sprintf("%s %s%4.0f%%",
					btopLabelStyle.Render(fmt.Sprintf("C%-2d", i+1)),
					btopBarThreshold(colW, pct1), pct1)
			}
			left = append(left, line)
		}
	} else {
		cbarW := leftW - 9
		if cbarW < 4 {
			cbarW = 4
		}
		for i := 0; i < numCores; i++ {
			pct := corePct(i)
			left = append(left, fmt.Sprintf("%s %s%4.0f%%",
				btopLabelStyle.Render(fmt.Sprintf("C%-2d", i)),
				btopBarThreshold(cbarW, pct), pct))
		}
	}

	la := b.cur.LoadAvg
	total, running, _, _, _ := CountStates(b.cur.Procs)
	right := []string{
		btopLabelStyle.Render("Load avg ") + btopValueStyle.Render(fmt.Sprintf("%.2f %.2f %.2f", la[0], la[1], la[2])),
		btopLabelStyle.Render("Uptime   ") + btopValueStyle.Render(FmtUptime(b.cur.Uptime)),
		btopLabelStyle.Render("Tasks    ") + btopValueStyle.Render(fmt.Sprintf("%d", total)),
		btopLabelStyle.Render("Running  ") + btopValueStyle.Render(fmt.Sprintf("%d", running)),
	}

	rows := len(left)
	if len(right) > rows {
		rows = len(right)
	}
	body := make([]string, rows)
	for i := 0; i < rows; i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		body[i] = btopPad(l, leftW) + " " + btopPad(r, rightW)
	}
	return btopBox("cpu", width, height, body)
}

func (b *BtopViewer) renderMemBox(width, height int) []string {
	innerW := width - 2
	if innerW < 6 {
		innerW = 6
	}
	mem := b.cur.Mem
	barW := innerW - 7
	if barW < 3 {
		barW = 3
	}

	pct := func(part uint64) float64 {
		if mem.Total == 0 {
			return 0
		}
		return float64(part) * 100 / float64(mem.Total)
	}

	row := func(label string, val uint64, p float64) string {
		return fmt.Sprintf("%s%s%s",
			btopLabelStyle.Render(btopPad(label, 6)),
			btopBarThreshold(barW, p),
			btopValueStyle.Render(fmt.Sprintf("%6s", FmtBytesShort(val))))
	}

	var swapPct float64
	if mem.SwapTotal > 0 {
		swapPct = float64(mem.SwapUsed()) * 100 / float64(mem.SwapTotal)
	}

	body := []string{
		btopLabelStyle.Render(btopPad("Total", 6)) + btopValueStyle.Render(FmtBytesShort(mem.Total)),
		row("Used", mem.Used(), pct(mem.Used())),
		row("Cache", mem.BuffCache(), pct(mem.BuffCache())),
		row("Free", mem.Free, pct(mem.Free)),
		row("Swap", mem.SwapUsed(), swapPct),
	}
	return btopBox("mem", width, height, body)
}

func (b *BtopViewer) renderDiskBox(width, height int) []string {
	innerW := width - 2
	if innerW < 6 {
		innerW = 6
	}
	elapsed := b.cur.Time.Sub(b.prev.Time).Seconds()

	prevMap := make(map[string]DiskStat, len(b.prev.Disks))
	for _, d := range b.prev.Disks {
		prevMap[d.Name] = d
	}

	barW := innerW - 16
	if barW < 4 {
		barW = 4
	}

	var body []string
	for _, d := range b.cur.Disks {
		if d.Mount == "" {
			continue
		}
		var readRate, writeRate uint64
		if p, ok := prevMap[d.Name]; ok && elapsed > 0 {
			readRate = uint64(float64(d.ReadBytes-p.ReadBytes) / elapsed)
			writeRate = uint64(float64(d.WriteBytes-p.WriteBytes) / elapsed)
		}
		mount := d.Mount
		if len(mount) > 10 {
			mount = mount[len(mount)-10:]
		}
		body = append(body, btopAccentStyle.Render(mount)+"  "+
			btopValueStyle.Render(FmtBytesShort(d.Total)))
		if d.Total > 0 {
			pct := float64(d.Used) * 100 / float64(d.Total)
			body = append(body, " "+btopBarThreshold(barW, pct)+
				fmt.Sprintf(" %4.0f%%", pct))
		}
		ioLine := " " + btopLabelStyle.Render("IO:") + " " +
			btopGreenStyle.Render("R "+FmtRate(readRate)) + " " +
			btopYellowStyle.Render("W "+FmtRate(writeRate))
		body = append(body, ioLine)
	}
	if len(body) == 0 {
		for _, d := range b.cur.Disks {
			body = append(body, btopAccentStyle.Render(d.Name))
		}
	}
	return btopBox("disks", width, height, body)
}

func (b *BtopViewer) renderNetBox(width, height int) []string {
	elapsed := b.cur.Time.Sub(b.prev.Time).Seconds()
	rxRate, txRate := CalcNetRates(b.prev.Net, b.cur.Net, elapsed)
	var totalRx, totalTx uint64
	for _, n := range b.cur.Net {
		totalRx += n.RxBytes
		totalTx += n.TxBytes
	}

	body := []string{
		btopGreenStyle.Render("▼ download ") + btopValueStyle.Render(FmtRate(rxRate)),
		btopRedStyle.Render("▲ upload   ") + btopValueStyle.Render(FmtRate(txRate)),
		btopLabelStyle.Render("total ") +
			btopGreenStyle.Render("▼") + btopValueStyle.Render(" "+FmtBytesShort(totalRx)) + "  " +
			btopRedStyle.Render("▲") + btopValueStyle.Render(" "+FmtBytesShort(totalTx)),
	}
	return btopBox("net", width, height, body)
}

const (
	procColPID  = 6
	procColUser = 9
	procColPct  = 6
	procColTime = 8
)

func (b *BtopViewer) renderProcBox(width, height int) []string {
	innerW := width - 2
	if innerW < 10 {
		innerW = 10
	}
	innerH := height - 2
	listH := innerH - 2
	if listH < 1 {
		listH = 1
	}

	procs := b.filteredProcs()
	if b.selectedIdx >= len(procs) {
		b.selectedIdx = len(procs) - 1
	}
	if b.selectedIdx < 0 {
		b.selectedIdx = 0
	}
	if b.selectedIdx < b.scrollOffset {
		b.scrollOffset = b.selectedIdx
	}
	if b.selectedIdx >= b.scrollOffset+listH {
		b.scrollOffset = b.selectedIdx - listH + 1
	}
	if b.scrollOffset < 0 {
		b.scrollOffset = 0
	}
	end := b.scrollOffset + listH
	if end > len(procs) {
		end = len(procs)
	}

	sortLabel := func(f SortField, label string) string {
		if !b.treeView && b.sortField == f {
			return btopTitleStyle.Render(fmt.Sprintf("%*s", procColPct, label))
		}
		return btopLabelStyle.Render(fmt.Sprintf("%*s", procColPct, label))
	}

	hdr := fmt.Sprintf("%*s %-*s %s %s %*s  %s",
		procColPID, "PID",
		procColUser, "USER",
		sortLabel(SortCPU, "CPU%"),
		sortLabel(SortMem, "MEM%"),
		procColTime, "TIME+",
		"COMMAND")

	body := []string{btopPad(hdr, innerW)}

	switch {
	case b.filterMode:
		body = append(body, btopAccentStyle.Render("filter: ")+b.filterBuf+btopValueStyle.Render("_"))
	case b.filterBuf != "":
		body = append(body, btopAccentStyle.Render("filter: ")+btopValueStyle.Render(b.filterBuf)+btopLabelStyle.Render("  (esc clears)"))
	default:
		body = append(body, btopLabelStyle.Render("[1]cpu [2]mem [3]pid [t]ree [f]ilter [K]ill [q]uit"))
	}

	for i := b.scrollOffset; i < end; i++ {
		p := procs[i]

		pidField := fmt.Sprintf("%*d", procColPID, p.PID)
		userField := fmt.Sprintf("%-*s", procColUser, TruncStr(p.User, procColUser-1))
		cpuField := fmt.Sprintf("%*.1f", procColPct, p.CPUPct)
		memField := fmt.Sprintf("%*.1f", procColPct, p.MemPct)
		timeField := fmt.Sprintf("%*s", procColTime, FmtCPUTime(p.CPUTime()))
		cmdField := p.Comm

		if i == b.selectedIdx {
			plain := pidField + " " + userField + " " + cpuField + " " + memField + " " + timeField + "  " + cmdField
			body = append(body, btopSelectStyle.Render(btopPad(plain, innerW)))
			continue
		}

		row := btopGreenStyle.Render(pidField) + " " +
			btopAccentStyle.Render(userField) + " " +
			btopThresholdColor(p.CPUPct).Render(cpuField) + " " +
			btopThresholdColor(p.MemPct).Render(memField) + " " +
			btopDimStyle.Render(timeField) + "  " +
			btopValueStyle.Render(cmdField)
		body = append(body, btopPad(row, innerW))
	}

	return btopBox("proc", width, height, body)
}

// Standalone

type btopModel struct{ viewer *BtopViewer }

func (m *btopModel) Init() tea.Cmd { return m.viewer.Init() }

func (m *btopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.viewer.Update(msg)
	if m.viewer.Quit {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *btopModel) View() tea.View { return fullScreen(m.viewer.View()) }

func RunBtop() error {
	v := NewBtop()
	p := tea.NewProgram(&btopModel{viewer: v})
	_, err := p.Run()
	return err
}
