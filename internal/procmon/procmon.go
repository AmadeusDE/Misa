package procmon

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Viewer interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View() string
	Resize(w, h int)
	IsQuit() bool
}

var (
	PageSize = uint64(os.Getpagesize())
	ClkTck   = uint64(100)
)

// unavailable renders a whole section the platform does not report; a single
// unknown field renders as unknown ("?") instead. Neither is a fabricated zero.
const unavailable = "—"

type CPUTimes struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
}

func (c CPUTimes) Total() uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.IOWait + c.IRQ + c.SoftIRQ + c.Steal
}

func (c CPUTimes) Active() uint64 {
	return c.Total() - c.Idle - c.IOWait
}

type MemInfo struct {
	Total        uint64
	Free         uint64
	Available    uint64
	Buffers      uint64
	Cached       uint64
	SwapTotal    uint64
	SwapFree     uint64
	Shmem        uint64
	SReclaimable uint64
}

func (m MemInfo) Used() uint64 {
	used := m.Total - m.Free - m.Buffers - m.Cached - m.SReclaimable + m.Shmem
	if used > m.Total {
		return m.Total - m.Free
	}
	return used
}

func (m MemInfo) SwapUsed() uint64 {
	return m.SwapTotal - m.SwapFree
}

func (m MemInfo) BuffCache() uint64 {
	return m.Buffers + m.Cached + m.SReclaimable - m.Shmem
}

type Process struct {
	PID     int
	Comm    string
	Cmdline string
	State   byte
	PPID    int
	UID     uint32
	User    string
	TTY     int

	UTime     uint64
	STime     uint64
	Priority  int64
	Nice      int64
	Threads   int
	StartTime uint64

	VSize uint64 // bytes
	RSS   int64  // pages
	SHR   int64  // pages (from statm)

	CPUPct float64
	MemPct float64

	// Identity identifies the process instance across snapshots (start time in
	// ticks since boot on Unix, creation time on Windows). It is zero when the
	// platform does not report one; CPU deltas then fall back to the PID.
	Identity uint64

	// Missing flags the fields the platform could not read. See ProcMissing.
	Missing ProcMissing
}

func (p Process) RSSBytes() uint64 { return uint64(p.RSS) * PageSize }
func (p Process) SHRBytes() uint64 { return uint64(p.SHR) * PageSize }
func (p Process) StateStr() string { return string(p.State) }
func (p Process) CPUTime() uint64  { return p.UTime + p.STime }
func (p Process) HasTTY() bool     { return p.TTY != 0 }

type NetDev struct {
	Name    string
	RxBytes uint64
	TxBytes uint64
	RxPkts  uint64
	TxPkts  uint64
}

type DiskStat struct {
	Name       string
	ReadBytes  uint64
	WriteBytes uint64
	Mount      string
	Total      uint64
	Used       uint64
}

type Snapshot struct {
	Time time.Time

	// SystemAvailable reports whether the platform supplied the system-wide
	// counters below (uptime, load average, memory, per-core CPU times). It is
	// false on Windows until the native system information provider (I03)
	// lands; the views then render those sections as unavailable instead of
	// showing zeros. CPUAvailable reports /proc/stat-class CPU counters only.
	SystemAvailable bool

	Uptime       float64
	LoadAvg      [3]float64
	CPUAll       CPUTimes
	CPUs         []CPUTimes
	CPUAvailable bool
	Mem          MemInfo
	Procs        []Process
	Net          []NetDev
	Disks        []DiskStat
}

// CalcCPUPercent fills each process's CPUPct from the CPU time it used since
// the previous snapshot.
//
// The percentage is per core (Irix mode, like htop and top): 100 % is one
// logical CPU busy, so a process using every core of an N-core machine reports
// N*100 %, and a thread-heavy process can exceed 100 % on a single core. It is
// deliberately not divided by the logical CPU count. Windows process CPU times
// already sum every thread, so the same formula gives the same scale there and
// all skins stay consistent. A process that vanished, was replaced (recycled
// PID) or cannot report CPU time keeps CPUPct zero.
func CalcCPUPercent(prev, cur []Process, elapsed float64) {
	m := make(map[int]*Process, len(prev))
	for i := range prev {
		m[prev[i].PID] = &prev[i]
	}
	for i := range cur {
		p, ok := m[cur[i].PID]
		if !ok || !cur[i].HasCPUTime() || !sameProcess(*p, cur[i]) {
			continue
		}
		if cur[i].CPUTime() < p.CPUTime() {
			continue
		}
		dt := float64(cur[i].CPUTime()-p.CPUTime()) / float64(ClkTck)
		if elapsed > 0 {
			cur[i].CPUPct = dt / elapsed * 100.0
		}
	}
}

// StatesKnown reports whether the process table reports process states, so the
// task counts are meaningful.
func StatesKnown(procs []Process) bool {
	if len(procs) == 0 {
		return false
	}
	for _, p := range procs {
		if !p.HasState() {
			return false
		}
	}
	return true
}

// UptimeStr renders the uptime, or the unavailable marker on a platform that
// does not report system-wide counters.
func (s Snapshot) UptimeStr() string {
	if !s.SystemAvailable {
		return unavailable
	}
	return FmtUptime(s.Uptime)
}

// LoadStr renders the three load averages, or the unavailable marker.
func (s Snapshot) LoadStr() string {
	if !s.SystemAvailable {
		return unavailable
	}
	return fmt.Sprintf("%.2f %.2f %.2f", s.LoadAvg[0], s.LoadAvg[1], s.LoadAvg[2])
}

func CalcMemPercent(procs []Process, totalMem uint64) {
	if totalMem == 0 {
		return
	}
	for i := range procs {
		procs[i].MemPct = float64(procs[i].RSSBytes()) * 100.0 / float64(totalMem)
	}
}

func CalcCPUDelta(prev, cur CPUTimes) (user, sys, nice, idle, iowait, irq, softirq, steal float64) {
	dt := float64(cur.Total() - prev.Total())
	if dt == 0 {
		return 0, 0, 0, 100, 0, 0, 0, 0
	}
	user = float64(cur.User-prev.User) / dt * 100
	sys = float64(cur.System-prev.System) / dt * 100
	nice = float64(cur.Nice-prev.Nice) / dt * 100
	idle = float64(cur.Idle-prev.Idle) / dt * 100
	iowait = float64(cur.IOWait-prev.IOWait) / dt * 100
	irq = float64(cur.IRQ-prev.IRQ) / dt * 100
	softirq = float64(cur.SoftIRQ-prev.SoftIRQ) / dt * 100
	steal = float64(cur.Steal-prev.Steal) / dt * 100
	return
}

func CalcCPUUsage(prev, cur CPUTimes) float64 {
	dt := float64(cur.Total() - prev.Total())
	if dt == 0 {
		return 0
	}
	return float64(cur.Active()-prev.Active()) / dt * 100
}

func CalcNetRates(prev, cur []NetDev, elapsed float64) (rxRate, txRate uint64) {
	pm := make(map[string]*NetDev, len(prev))
	for i := range prev {
		pm[prev[i].Name] = &prev[i]
	}
	for _, c := range cur {
		if p, ok := pm[c.Name]; ok && elapsed > 0 {
			rxRate += uint64(float64(c.RxBytes-p.RxBytes) / elapsed)
			txRate += uint64(float64(c.TxBytes-p.TxBytes) / elapsed)
		}
	}
	return
}

func CalcDiskRates(prev, cur []DiskStat, elapsed float64) (readRate, writeRate uint64) {
	pm := make(map[string]*DiskStat, len(prev))
	for i := range prev {
		pm[prev[i].Name] = &prev[i]
	}
	for _, c := range cur {
		if p, ok := pm[c.Name]; ok && elapsed > 0 {
			readRate += uint64(float64(c.ReadBytes-p.ReadBytes) / elapsed)
			writeRate += uint64(float64(c.WriteBytes-p.WriteBytes) / elapsed)
		}
	}
	return
}

// Sorting

type SortField int

const (
	SortCPU SortField = iota
	SortMem
	SortPID
	SortUser
	SortTime
	SortVirt
	SortRes
	SortShr
	SortNice
	SortState
	SortCommand
)

func SortProcs(procs []Process, field SortField, ascending bool) {
	sort.SliceStable(procs, func(i, j int) bool {
		var less bool
		switch field {
		case SortCPU:
			less = procs[i].CPUPct > procs[j].CPUPct
		case SortMem:
			less = procs[i].MemPct > procs[j].MemPct
		case SortPID:
			less = procs[i].PID < procs[j].PID
		case SortUser:
			less = procs[i].User < procs[j].User
		case SortTime:
			less = procs[i].CPUTime() > procs[j].CPUTime()
		case SortVirt:
			less = procs[i].VSize > procs[j].VSize
		case SortRes:
			less = procs[i].RSSBytes() > procs[j].RSSBytes()
		case SortShr:
			less = procs[i].SHRBytes() > procs[j].SHRBytes()
		case SortNice:
			less = procs[i].Nice < procs[j].Nice
		case SortState:
			less = procs[i].State < procs[j].State
		case SortCommand:
			less = procs[i].Comm < procs[j].Comm
		default:
			less = procs[i].CPUPct > procs[j].CPUPct
		}
		if ascending {
			return !less
		}
		return less
	})
}

func BuildTree(procs []Process) []Process {
	byPPID := make(map[int][]int)
	byPID := make(map[int]int)
	for i, p := range procs {
		byPPID[p.PPID] = append(byPPID[p.PPID], i)
		byPID[p.PID] = i
	}

	visited := make(map[int]bool, len(procs))
	var result []Process

	var walk func(pid, depth int)
	walk = func(pid, depth int) {
		idx, ok := byPID[pid]
		if !ok || visited[pid] {
			return
		}
		visited[pid] = true
		p := procs[idx]
		prefix := ""
		if depth > 0 {
			prefix = strings.Repeat("│ ", depth-1) + "├─"
		}
		p.Comm = prefix + p.Comm
		result = append(result, p)
		for _, ci := range byPPID[pid] {
			walk(procs[ci].PID, depth+1)
		}
	}

	roots := make([]int, 0)
	for _, p := range procs {
		if _, hasParent := byPID[p.PPID]; !hasParent {
			roots = append(roots, p.PID)
		}
	}
	sort.Ints(roots)
	for _, pid := range roots {
		walk(pid, 0)
	}

	for i, p := range procs {
		if !visited[p.PID] {
			result = append(result, procs[i])
		}
	}
	return result
}

// Format helpers

func FmtUptime(secs float64) string {
	d := int(secs) / 86400
	h := (int(secs) % 86400) / 3600
	m := (int(secs) % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%d day(s), %d:%02d", d, h, m)
	}
	return fmt.Sprintf("%d:%02d", h, m)
}

func FmtBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.0fK", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.2fG", float64(b)/(1024*1024*1024))
}

func FmtBytesShort(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.0fK", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(b)/(1024*1024))
	}
	if b < 1024*1024*1024*1024 {
		return fmt.Sprintf("%.1fG", float64(b)/(1024*1024*1024))
	}
	return fmt.Sprintf("%.1fT", float64(b)/(1024*1024*1024*1024))
}

func FmtRate(bytesPerSec uint64) string {
	return FmtBytesShort(bytesPerSec) + "/s"
}

func FmtCPUTime(ticks uint64) string {
	secs := ticks / ClkTck
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	hundredths := (ticks % ClkTck) * 100 / ClkTck
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d.%02d", m, s, hundredths)
}

func CountStates(procs []Process) (total, running, sleeping, stopped, zombie int) {
	total = len(procs)
	for _, p := range procs {
		switch p.State {
		case 'R':
			running++
		case 'S', 'D', 'I':
			sleeping++
		case 'T', 't':
			stopped++
		case 'Z':
			zombie++
		}
	}
	return
}

func Bar(width int, pct float64, ch rune) string {
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
	return strings.Repeat(string(ch), filled) + strings.Repeat(" ", width-filled)
}

type BarSegment struct {
	Pct   float64
	Style lipgloss.Style
}

func StyledBar(width int, segments []BarSegment, bgStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, seg := range segments {
		n := int(seg.Pct / 100.0 * float64(width))
		if n+used > width {
			n = width - used
		}
		if n > 0 {
			b.WriteString(seg.Style.Render(strings.Repeat("|", n)))
			used += n
		}
	}
	if used < width {
		b.WriteString(bgStyle.Render(strings.Repeat(" ", width-used)))
	}
	return b.String()
}

func ThresholdBar(width int, pct float64, low, med, high lipgloss.Style, bgStyle lipgloss.Style) string {
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
		style = high
	case pct > 50:
		style = med
	default:
		style = low
	}
	return style.Render(strings.Repeat("█", filled)) +
		bgStyle.Render(strings.Repeat("░", width-filled))
}

func TruncStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	visible := 0
	i := 0
	for i < len(s) {
		if s[i] == '\033' {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';' || s[j] == '?') {
					j++
				}
				if j < len(s) {
					j++
				}
			}
			i = j
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		visible++
		if visible > maxLen {
			return s[:i]
		}
		i += size
	}
	return s
}
