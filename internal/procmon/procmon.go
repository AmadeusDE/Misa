package procmon

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
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

// procRoot is the procfs root. Tests point it at a fake tree.
var procRoot = "/proc"

const unavailable = "—"

func procFile(elem ...string) string {
	return filepath.Join(append([]string{procRoot}, elem...)...)
}

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
	Time         time.Time
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

var uidCache = map[uint32]string{}

func LookupUser(uid uint32) string {
	if name, ok := uidCache[uid]; ok {
		return name
	}
	name := strconv.Itoa(int(uid))
	if u, err := user.LookupId(name); err == nil {
		name = u.Username
	}
	uidCache[uid] = name
	return name
}

func ReadUptime() float64 {
	data, err := os.ReadFile(procFile("uptime"))
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func ReadLoadAvg() [3]float64 {
	var la [3]float64
	data, err := os.ReadFile(procFile("loadavg"))
	if err != nil {
		return la
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return la
	}
	la[0], _ = strconv.ParseFloat(fields[0], 64)
	la[1], _ = strconv.ParseFloat(fields[1], 64)
	la[2], _ = strconv.ParseFloat(fields[2], 64)
	return la
}

func parseCPULine(fields []string) (CPUTimes, bool) {
	if len(fields) < 5 {
		return CPUTimes{}, false
	}
	var ct CPUTimes
	ct.User, _ = strconv.ParseUint(fields[1], 10, 64)
	ct.Nice, _ = strconv.ParseUint(fields[2], 10, 64)
	ct.System, _ = strconv.ParseUint(fields[3], 10, 64)
	ct.Idle, _ = strconv.ParseUint(fields[4], 10, 64)
	if len(fields) > 5 {
		ct.IOWait, _ = strconv.ParseUint(fields[5], 10, 64)
	}
	if len(fields) > 6 {
		ct.IRQ, _ = strconv.ParseUint(fields[6], 10, 64)
	}
	if len(fields) > 7 {
		ct.SoftIRQ, _ = strconv.ParseUint(fields[7], 10, 64)
	}
	if len(fields) > 8 {
		ct.Steal, _ = strconv.ParseUint(fields[8], 10, 64)
	}
	return ct, true
}

// ReadCPUTimes reports whether /proc/stat could be read. EACCES and a missing
// file are unavailable (ok == false): callers show "—" and do not fail.
func ReadCPUTimes() (CPUTimes, []CPUTimes, bool) {
	f, err := os.Open(procFile("stat"))
	if err != nil {
		return CPUTimes{}, nil, false
	}
	defer f.Close()

	var all CPUTimes
	var cores []CPUTimes
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "cpu" {
			all, _ = parseCPULine(fields)
		} else if strings.HasPrefix(fields[0], "cpu") {
			if ct, ok := parseCPULine(fields); ok {
				cores = append(cores, ct)
			}
		}
	}
	return all, cores, true
}

func ReadMemInfo() MemInfo {
	var mi MemInfo
	f, err := os.Open(procFile("meminfo"))
	if err != nil {
		return mi
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(parts[1])
		valStr = strings.TrimSuffix(valStr, " kB")
		val, _ := strconv.ParseUint(strings.TrimSpace(valStr), 10, 64)
		val *= 1024

		switch key {
		case "MemTotal":
			mi.Total = val
		case "MemFree":
			mi.Free = val
		case "MemAvailable":
			mi.Available = val
		case "Buffers":
			mi.Buffers = val
		case "Cached":
			mi.Cached = val
		case "SwapTotal":
			mi.SwapTotal = val
		case "SwapFree":
			mi.SwapFree = val
		case "Shmem":
			mi.Shmem = val
		case "SReclaimable":
			mi.SReclaimable = val
		}
	}
	return mi
}

func readProcess(pid int) (Process, bool) {
	dir := procFile(strconv.Itoa(pid))

	statData, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return Process{}, false
	}

	s := string(statData)
	openP := strings.IndexByte(s, '(')
	closeP := strings.LastIndexByte(s, ')')
	if openP < 0 || closeP < 0 || closeP <= openP {
		return Process{}, false
	}

	var p Process
	p.PID = pid
	p.Comm = s[openP+1 : closeP]

	rest := strings.Fields(s[closeP+2:])
	if len(rest) < 22 {
		return Process{}, false
	}

	p.State = rest[0][0]
	p.PPID, _ = strconv.Atoi(rest[1])
	p.TTY, _ = strconv.Atoi(rest[4])
	p.UTime, _ = strconv.ParseUint(rest[11], 10, 64)
	p.STime, _ = strconv.ParseUint(rest[12], 10, 64)
	p.Priority, _ = strconv.ParseInt(rest[15], 10, 64)
	p.Nice, _ = strconv.ParseInt(rest[16], 10, 64)
	p.Threads, _ = strconv.Atoi(rest[17])
	p.StartTime, _ = strconv.ParseUint(rest[19], 10, 64)
	p.VSize, _ = strconv.ParseUint(rest[20], 10, 64)
	rss, _ := strconv.ParseInt(rest[21], 10, 64)
	p.RSS = rss

	if statmData, err := os.ReadFile(filepath.Join(dir, "statm")); err == nil {
		fields := strings.Fields(string(statmData))
		if len(fields) >= 3 {
			shr, _ := strconv.ParseInt(fields[2], 10, 64)
			p.SHR = shr
		}
	}

	if statusData, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
		for _, line := range strings.Split(string(statusData), "\n") {
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					uid, _ := strconv.ParseUint(fields[1], 10, 32)
					p.UID = uint32(uid)
				}
				break
			}
		}
	}

	if data, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
		p.Cmdline = strings.ReplaceAll(strings.TrimRight(string(data), "\x00"), "\x00", " ")
	}
	if p.Cmdline == "" {
		p.Cmdline = "[" + p.Comm + "]"
	}

	p.User = LookupUser(p.UID)
	return p, true
}

func ReadAllProcs() []Process {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	var procs []Process
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if p, ok := readProcess(pid); ok {
			procs = append(procs, p)
		}
	}
	return procs
}

func ReadNetDev() []NetDev {
	f, err := os.Open(procFile("net", "dev"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var devs []NetDev
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		rxp, _ := strconv.ParseUint(fields[1], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		txp, _ := strconv.ParseUint(fields[9], 10, 64)
		devs = append(devs, NetDev{Name: name, RxBytes: rx, TxBytes: tx, RxPkts: rxp, TxPkts: txp})
	}
	return devs
}

func ReadDiskStats() []DiskStat {
	f, err := os.Open(procFile("diskstats"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var disks []DiskStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "zram") {
			continue
		}
		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
		disks = append(disks, DiskStat{
			Name:       name,
			ReadBytes:  readSectors * 512,
			WriteBytes: writeSectors * 512,
		})
	}
	return disks
}

func enrichDiskMounts(ioDisks []DiskStat) []DiskStat {
	f, err := os.Open(procFile("mounts"))
	if err != nil {
		return ioDisks
	}
	defer f.Close()

	ioByName := map[string]*DiskStat{}
	for i := range ioDisks {
		ioByName[ioDisks[i].Name] = &ioDisks[i]
	}

	seen := map[string]bool{}
	var result []DiskStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		dev := fields[0]
		mount := fields[1]
		if !strings.HasPrefix(dev, "/dev/") {
			continue
		}
		devName := filepath.Base(dev)
		if seen[devName] {
			continue
		}
		var stat syscall.Statfs_t
		if syscall.Statfs(mount, &stat) != nil {
			continue
		}
		total, free := getDiskSpace(&stat)
		if total == 0 {
			continue
		}
		seen[devName] = true
		d := DiskStat{
			Name:  devName,
			Mount: mount,
			Total: total,
			Used:  total - free,
		}
		if io, ok := ioByName[devName]; ok {
			d.ReadBytes = io.ReadBytes
			d.WriteBytes = io.WriteBytes
		}
		result = append(result, d)
	}
	if len(result) == 0 {
		return ioDisks
	}
	return result
}

func TakeSnapshot() Snapshot {
	cpuAll, cpus, cpuOK := ReadCPUTimes()
	return Snapshot{
		Time:         time.Now(),
		Uptime:       ReadUptime(),
		LoadAvg:      ReadLoadAvg(),
		CPUAll:       cpuAll,
		CPUs:         cpus,
		CPUAvailable: cpuOK,
		Mem:          ReadMemInfo(),
		Procs:        ReadAllProcs(),
		Net:          ReadNetDev(),
		Disks:        enrichDiskMounts(ReadDiskStats()),
	}
}

func TakeFullSnapshot() Snapshot {
	cpuAll, cpus, cpuOK := ReadCPUTimes()
	return Snapshot{
		Time:         time.Now(),
		Uptime:       ReadUptime(),
		LoadAvg:      ReadLoadAvg(),
		CPUAll:       cpuAll,
		CPUs:         cpus,
		CPUAvailable: cpuOK,
		Mem:          ReadMemInfo(),
		Procs:        ReadAllProcs(),
		Net:          ReadNetDev(),
		Disks:        enrichDiskMounts(ReadDiskStats()),
	}
}

func CalcCPUPercent(prev, cur []Process, elapsed float64) {
	m := make(map[int]*Process, len(prev))
	for i := range prev {
		m[prev[i].PID] = &prev[i]
	}
	for i := range cur {
		if p, ok := m[cur[i].PID]; ok {
			dt := float64(cur[i].CPUTime()-p.CPUTime()) / float64(ClkTck)
			if elapsed > 0 {
				cur[i].CPUPct = dt / elapsed * 100.0
			}
		}
	}
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
