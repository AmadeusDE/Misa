package procmon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReadUptime(t *testing.T) {
	up := ReadUptime()
	if up <= 0 {
		t.Errorf("uptime should be positive, got %f", up)
	}
}

func TestReadLoadAvg(t *testing.T) {
	la := ReadLoadAvg()
	if la[0] < 0 || la[1] < 0 || la[2] < 0 {
		t.Errorf("load averages should be non-negative: %v", la)
	}
}

func TestReadCPUTimes(t *testing.T) {
	all, cores, ok := ReadCPUTimes()
	if !ok {
		t.Fatal("host /proc/stat should be readable here")
	}
	if all.Total() == 0 {
		t.Error("total CPU time should be non-zero")
	}
	if len(cores) == 0 {
		t.Error("should have at least one CPU core")
	}
}

func TestReadMemInfo(t *testing.T) {
	mem := ReadMemInfo()
	if mem.Total == 0 {
		t.Error("total memory should be non-zero")
	}
	if mem.Used() > mem.Total {
		t.Errorf("used memory %d should not exceed total %d", mem.Used(), mem.Total)
	}
}

func TestReadAllProcs(t *testing.T) {
	procs := ReadAllProcs()
	if len(procs) == 0 {
		t.Error("should have at least one process")
	}
	foundSelf := false
	for _, p := range procs {
		if p.Comm != "" && p.PID > 0 {
			foundSelf = true
			break
		}
	}
	if !foundSelf {
		t.Error("should find at least one valid process")
	}
}

func TestCountStates(t *testing.T) {
	procs := ReadAllProcs()
	total, running, sleeping, stopped, zombie := CountStates(procs)
	if total != len(procs) {
		t.Errorf("total %d should equal len(procs) %d", total, len(procs))
	}
	sum := running + sleeping + stopped + zombie
	if sum > total {
		t.Errorf("state counts (%d) should not exceed total (%d)", sum, total)
	}
}

func TestSortProcs(t *testing.T) {
	procs := []Process{
		{PID: 1, CPUPct: 10, MemPct: 5},
		{PID: 2, CPUPct: 50, MemPct: 2},
		{PID: 3, CPUPct: 1, MemPct: 80},
	}
	SortProcs(procs, SortCPU, false)
	if procs[0].PID != 2 {
		t.Errorf("expected PID 2 first when sorting by CPU descending, got %d", procs[0].PID)
	}

	SortProcs(procs, SortMem, false)
	if procs[0].PID != 3 {
		t.Errorf("expected PID 3 first when sorting by MEM descending, got %d", procs[0].PID)
	}

	SortProcs(procs, SortPID, false)
	if procs[0].PID != 1 {
		t.Errorf("expected PID 1 first when sorting by PID, got %d", procs[0].PID)
	}
}

func TestBuildTree(t *testing.T) {
	procs := []Process{
		{PID: 1, PPID: 0, Comm: "init"},
		{PID: 10, PPID: 1, Comm: "shell"},
		{PID: 100, PPID: 10, Comm: "app"},
	}
	tree := BuildTree(procs)
	if len(tree) != 3 {
		t.Fatalf("expected 3 procs in tree, got %d", len(tree))
	}
	if tree[0].PID != 1 {
		t.Errorf("root should be PID 1, got %d", tree[0].PID)
	}
}

func TestFmtBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1K"},
		{1536, "2K"},
		{1048576, "1.0M"},
		{1073741824, "1.00G"},
	}
	for _, tt := range tests {
		got := FmtBytes(tt.input)
		if got != tt.expected {
			t.Errorf("FmtBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFmtUptime(t *testing.T) {
	tests := []struct {
		secs     float64
		expected string
	}{
		{3600, "1:00"},
		{7200, "2:00"},
		{86400, "1 day(s), 0:00"},
		{90061, "1 day(s), 1:01"},
	}
	for _, tt := range tests {
		got := FmtUptime(tt.secs)
		if got != tt.expected {
			t.Errorf("FmtUptime(%f) = %q, want %q", tt.secs, got, tt.expected)
		}
	}
}

func TestFmtCPUTime(t *testing.T) {
	got := FmtCPUTime(6150)
	if got != "1:01.50" {
		t.Errorf("FmtCPUTime(6150) = %q, want %q", got, "1:01.50")
	}
}

func TestBar(t *testing.T) {
	b := Bar(10, 50, '|')
	if len(b) != 10 {
		t.Errorf("bar length should be 10, got %d", len(b))
	}
}

func TestCalcCPUDelta(t *testing.T) {
	prev := CPUTimes{User: 100, System: 50, Idle: 850}
	cur := CPUTimes{User: 200, System: 100, Idle: 1700}
	us, sy, _, id, _, _, _, _ := CalcCPUDelta(prev, cur)
	if us < 9 || us > 11 {
		t.Errorf("user%% should be ~10, got %f", us)
	}
	if sy < 4 || sy > 6 {
		t.Errorf("sys%% should be ~5, got %f", sy)
	}
	if id < 84 || id > 86 {
		t.Errorf("idle%% should be ~85, got %f", id)
	}
}

func TestReadNetDev(t *testing.T) {
	devs := ReadNetDev()
	// May be empty in containers, but shouldn't panic
	_ = devs
}

func TestReadDiskStats(t *testing.T) {
	disks := ReadDiskStats()
	_ = disks
}

func TestTakeFullSnapshot(t *testing.T) {
	snap := TakeFullSnapshot()
	if snap.Uptime <= 0 {
		t.Error("snapshot uptime should be positive")
	}
	if snap.Mem.Total == 0 {
		t.Error("snapshot memory total should be non-zero")
	}
	if len(snap.Procs) == 0 {
		t.Error("snapshot should have processes")
	}
}

func TestLookupUser(t *testing.T) {
	name := LookupUser(0)
	if name != "root" && name != "0" {
		t.Errorf("UID 0 should be root, got %q", name)
	}
	name2 := LookupUser(0)
	if name2 != name {
		t.Error("cached lookup should return same value")
	}
}

func TestTruncStr(t *testing.T) {
	if TruncStr("hello", 3) != "hel" {
		t.Error("should truncate to 3")
	}
	if TruncStr("hi", 10) != "hi" {
		t.Error("should not truncate short strings")
	}
	ansi := "\033[31mred\033[0m"
	got := TruncStr(ansi, 3)
	if got != ansi {
		t.Errorf("should preserve ANSI: got %q, want %q", got, ansi)
	}
	got2 := TruncStr("\033[31mhello\033[0m", 3)
	if got2 != "\033[31mhel" {
		t.Errorf("should truncate visible chars with ANSI: got %q", got2)
	}
	bars := "████░░"
	got3 := TruncStr(bars, 4)
	if got3 != "████" {
		t.Errorf("should truncate multi-byte runes correctly: got %q, want %q", got3, "████")
	}
}

func fakeStat(pid int, comm string) string {
	return strconv.Itoa(pid) + " (" + comm + ") S 1 1 1 0 -1 0 0 0 0 0 0 0 0 20 0 1 0 100 200 3 0\n"
}

func TestFakeProcFSUnavailableAndMissingPIDs(t *testing.T) {
	dir := t.TempDir()
	old := procRoot
	procRoot = dir
	t.Cleanup(func() { procRoot = old })

	if _, _, ok := ReadCPUTimes(); ok {
		t.Fatal("missing stat was readable")
	}
	snap := TakeSnapshot()
	if snap.CPUAvailable || len(snap.Procs) != 0 {
		t.Fatalf("empty procfs: available=%v procs=%d", snap.CPUAvailable, len(snap.Procs))
	}

	stat := filepath.Join(dir, "stat")
	body := "cpu  10 20 30 40 1 2 3 4\ncpu0 5 10 15 20 0 0 0 0\n"
	if err := os.WriteFile(stat, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	all, cores, ok := ReadCPUTimes()
	if !ok || all.User != 10 || len(cores) != 1 || cores[0].User != 5 {
		t.Fatalf("parsed stat ok=%v all=%+v cores=%+v", ok, all, cores)
	}

	if os.Geteuid() != 0 {
		if err := os.Chmod(stat, 0); err != nil {
			t.Fatal(err)
		}
		if _, _, ok := ReadCPUTimes(); ok {
			t.Fatal("EACCES on stat was treated as available")
		}
		if err := os.Chmod(stat, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Mkdir(filepath.Join(dir, "20"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20", "stat"), []byte(fakeStat(20, "alive")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "21"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "not-a-pid"), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		if err := os.Mkdir(filepath.Join(dir, "22"), 0o755); err != nil {
			t.Fatal(err)
		}
		denied := filepath.Join(dir, "22", "stat")
		if err := os.WriteFile(denied, []byte(fakeStat(22, "hidden")), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(denied, 0); err != nil {
			t.Fatal(err)
		}
	}

	procs := ReadAllProcs()
	if len(procs) != 1 || procs[0].PID != 20 || procs[0].Comm != "alive" {
		t.Fatalf("procs = %+v, want only pid 20", procs)
	}
	if _, ok := readProcess(99); ok {
		t.Fatal("missing pid was readable")
	}
}

func TestViewsShowDashWhenCPUUnavailable(t *testing.T) {
	top := &TopViewer{width: 80, height: 16, sortField: SortCPU}
	if !strings.Contains(top.View(), "—") {
		t.Fatalf("top hid unavailable CPU: %q", top.View())
	}
	if strings.Contains(top.View(), " id,") {
		t.Fatalf("top rendered an idle breakdown: %q", top.View())
	}

	htop := &HtopViewer{width: 80, height: 16}
	if !strings.Contains(htop.View(), "—") {
		t.Fatalf("htop hid unavailable CPU: %q", htop.View())
	}

	btop := &BtopViewer{width: 80, height: 24}
	if !strings.Contains(strings.Join(btop.renderCPUBox(80, 8), "\n"), "—") {
		t.Fatal("btop hid unavailable CPU")
	}

	tm := &TMViewer{width: 100, height: 20, tab: tabProcesses}
	if !strings.Contains(tm.View(), "—") {
		t.Fatalf("task manager hid unavailable CPU: %q", tm.View())
	}
	tm.tab = tabPerformance
	if !strings.Contains(tm.View(), "—") {
		t.Fatalf("performance tab hid unavailable CPU: %q", tm.View())
	}
}
