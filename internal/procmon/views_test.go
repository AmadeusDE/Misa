package procmon

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// plain drops the styling so a view can be asserted on as text.
func plain(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// unixProc is a process exactly as the Unix backend reports one: every field
// available. Its formatted rows anchor the Unix output, which the Windows work
// must not change.
var unixProc = Process{
	PID: 1234, PPID: 1, Comm: "bash", Cmdline: "/bin/bash -l", State: 'S',
	UID: 1000, User: "alice", TTY: 1,
	UTime: 500, STime: 250, Priority: 20, Nice: 0, Threads: 1, StartTime: 42,
	VSize: 10 << 20, RSS: 100, SHR: 20,
	CPUPct: 1.5, MemPct: 2.5, Identity: 42,
}

// windowsProc is a process as the Windows backend reports it once its detail
// was read: enumeration and detail fields available, state, user, shared memory
// and the memory percentage honestly missing.
func windowsProc() Process {
	p := procRow{PID: 4321, PPID: 1, Name: "explorer.exe", Threads: 42, BasePrio: 8}.process()
	p.Missing |= MissingMemPct
	p = p.withDetail(procDetail{
		Kernel100ns: 20_000_000, User100ns: 5_000_000, Identity: 9,
		WorkingSet: 100 * PageSize, Commit: 10 << 20, Image: `C:\Windows\explorer.exe`,
	})
	p.CPUPct = 1.5
	return p
}

func TestHtopRowKeepsUnixLayout(t *testing.T) {
	want := fmt.Sprintf("%7d %-8s %3d  %3d %7s %7s %7s %s %4.1f %4.1f %8s %s",
		unixProc.PID, "alice", 20, 0,
		FmtBytes(10<<20), FmtBytes(100*PageSize), FmtBytes(20*PageSize),
		"S", 1.5, 2.5, FmtCPUTime(750), "bash")
	if got := formatProcRowPlain(unixProc, 200); got != want {
		t.Errorf("unix row changed\n got %q\nwant %q", got, want)
	}
}

func TestTopRowKeepsUnixLayout(t *testing.T) {
	top := &TopViewer{width: 200, height: 20, sortField: SortPID}
	top.cur = Snapshot{SystemAvailable: true, Time: time.Now(), Uptime: 3600,
		CPUAvailable: true, Mem: MemInfo{Total: 1 << 30, Free: 1 << 29},
		Procs: []Process{unixProc}}
	top.prev = top.cur

	want := fmt.Sprintf("  %7d %-9s %3d %4d %7s %7s %7s %s %5.1f %5.1f %9s %s",
		unixProc.PID, "alice", 20, 0,
		FmtBytes(10<<20), FmtBytes(100*PageSize), FmtBytes(20*PageSize),
		"S", 1.5, 2.5, FmtCPUTime(750), "bash")
	view := plain(top.View())
	if !strings.Contains(view, want) {
		t.Errorf("top row changed, want %q in\n%s", want, view)
	}
	if !strings.Contains(view, "1:00") {
		t.Error("top dropped the uptime")
	}
	if !strings.Contains(view, "1 total, 0 running, 1 sleeping, 0 stopped, 0 zombie") {
		t.Error("top dropped the task counts")
	}
}

func TestWindowsRowMarksUnknownColumns(t *testing.T) {
	bare := procRow{PID: 4321, PPID: 1, Name: "explorer.exe", Threads: 42, BasePrio: 8}.process()
	// user, priority? no (base priority is known), virt, res, shared, state,
	// CPU%, MEM% and TIME+ are unknown before the detail read.
	if got := strings.Count(formatProcRowPlain(bare, 200), "?"); got != 8 {
		t.Errorf("bare row has %d unknown columns, want 8: %q", got, formatProcRowPlain(bare, 200))
	}

	read := windowsProc()
	// user, shared memory, state and %MEM stay unavailable on Windows.
	if got := strings.Count(formatProcRowPlain(read, 200), "?"); got != 4 {
		t.Errorf("detail row has %d unknown columns, want 4: %q", got, formatProcRowPlain(read, 200))
	}
	if !strings.Contains(formatProcRowPlain(read, 200), "  10.0M") {
		t.Errorf("detail row lost its virtual size: %q", formatProcRowPlain(read, 200))
	}
}

func TestViewsRenderWindowsSnapshotWithoutFakingSystemSections(t *testing.T) {
	snap := Snapshot{SystemAvailable: false, Time: time.Now(), Procs: []Process{windowsProc()}}

	top := &TopViewer{width: 120, height: 20, sortField: SortCPU}
	top.cur, top.prev = snap, snap
	topView := plain(top.View())
	for _, want := range []string{"up —, load average: —", "MiB Mem: —", "MiB Swap: —", "Tasks: 1 total"} {
		if !strings.Contains(topView, want) {
			t.Errorf("top view is missing %q:\n%s", want, topView)
		}
	}
	if strings.Contains(topView, "0B total") || strings.Contains(topView, "0 running") {
		t.Errorf("top view invented system or state values:\n%s", topView)
	}

	htop := &HtopViewer{width: 120, height: 20}
	htop.cur, htop.prev = snap, snap
	htopView := plain(htop.View())
	for _, want := range []string{"Mem—", "Swp—", "Load average: —", "Uptime: —", "Tasks: 1, 42 thr"} {
		if !strings.Contains(htopView, want) {
			t.Errorf("htop view is missing %q:\n%s", want, htopView)
		}
	}

	btop := &BtopViewer{width: 120, height: 30}
	btop.cur, btop.prev = snap, snap
	btopView := plain(btop.View())
	for _, want := range []string{"Load avg " + unavailable, "Uptime   " + unavailable, "Total " + unavailable} {
		if !strings.Contains(btopView, want) {
			t.Errorf("btop view is missing %q:\n%s", want, btopView)
		}
	}
	if strings.Contains(btopView, "Running") {
		t.Errorf("btop view invented a running count:\n%s", btopView)
	}

	tm := &TMViewer{width: 120, height: 24, tab: tabPerformance}
	tm.cur, tm.prev = snap, snap
	tmView := plain(tm.View())
	for _, want := range []string{"Load: " + unavailable, "In Use   " + unavailable, "Swap     " + unavailable} {
		if !strings.Contains(tmView, want) {
			t.Errorf("task manager view is missing %q:\n%s", want, tmView)
		}
	}
}

func TestWindowsRowsOfferClassPrioritiesInTheKillMenu(t *testing.T) {
	if len(windowsSignals) != 2 {
		t.Fatalf("windows signal menu = %+v, want Cancel and Terminate only", windowsSignals)
	}
	if len(windowsPriorityLevels) != 5 {
		t.Fatalf("windows priority levels = %+v", windowsPriorityLevels)
	}
}
