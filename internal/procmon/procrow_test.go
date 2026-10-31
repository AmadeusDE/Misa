package procmon

import (
	"testing"
	"time"
)

func TestProcRowKeepsEnumerationFieldsAndMarksTheRest(t *testing.T) {
	p := procRow{PID: 4321, PPID: 1, Name: "explorer.exe", Threads: 42, BasePrio: 8}.process()

	if p.PID != 4321 || p.PPID != 1 || p.Comm != "explorer.exe" || p.Threads != 42 {
		t.Fatalf("enumeration fields lost: %+v", p)
	}
	if p.Cmdline != "explorer.exe" {
		t.Errorf("cmdline = %q, want the executable name until the path is read", p.Cmdline)
	}
	if p.HasUser() || p.HasState() || p.HasCPUTime() || p.HasMemory() || p.HasVirt() || p.HasShared() || p.HasMemPct() {
		t.Errorf("a bare row must not claim fields it has not read: missing = %b", p.Missing)
	}
	if !p.HasPriority() || p.Nice != 0 || p.Priority != 8 {
		t.Errorf("priority from base 8 = (%d, %d), want normal (8, 0)", p.Priority, p.Nice)
	}
	if p.CPUPct != 0 || p.MemPct != 0 || p.RSS != 0 {
		t.Error("a bare row must not invent CPU or memory values")
	}
}

func TestProcRowUnknownNameAndPriority(t *testing.T) {
	p := procRow{PID: 0}.process()
	if p.Comm != "[0]" {
		t.Errorf("unnamed process = %q, want a bracketed pid like the Unix convention", p.Comm)
	}
	if p.HasPriority() {
		t.Error("base priority 0 is not a class base priority")
	}
}

func TestProcDetailFillsFieldsAndClearsFlags(t *testing.T) {
	p := procRow{PID: 7, Name: "app.exe", BasePrio: 4}.process()
	p.Missing |= MissingMemPct
	p = p.withDetail(procDetail{
		Kernel100ns: 20_000_000, // 2 s of kernel time
		User100ns:   5_000_000,  // 0.5 s of user time
		Identity:    123456,
		WorkingSet:  3 * PageSize,
		Commit:      1 << 20,
		Image:       `C:\Apps\app.exe`,
	})

	if p.UTime != 50 || p.STime != 200 {
		t.Errorf("cpu ticks = user %d kernel %d, want 50 and 200 at ClkTck %d", p.UTime, p.STime, ClkTck)
	}
	if p.Identity != 123456 {
		t.Errorf("identity = %d", p.Identity)
	}
	if p.RSS != 3 || p.VSize != 1<<20 {
		t.Errorf("rss = %d pages, vsize = %d, want 3 and %d", p.RSS, p.VSize, 1<<20)
	}
	if p.Cmdline != `C:\Apps\app.exe` {
		t.Errorf("cmdline = %q, want the image path", p.Cmdline)
	}
	if !p.HasCPUTime() || !p.HasMemory() || !p.HasVirt() {
		t.Errorf("detail did not clear its flags: %b", p.Missing)
	}
	if p.HasShared() || p.HasState() || p.HasUser() || p.HasMemPct() || !p.HasPriority() {
		t.Errorf("unrelated fields changed: %b", p.Missing)
	}
	if p.Nice != 19 {
		t.Errorf("idle class nice = %d, want 19", p.Nice)
	}
}

func TestProcDetailPriorityClassIsAuthoritative(t *testing.T) {
	// The class read with the process handle wins over the enumerated base.
	p := procRow{PID: 9, Name: "app.exe", BasePrio: 8}.process()
	p = p.withDetail(procDetail{Class: PriorityHigh, HasClass: true})
	if p.Nice != -10 || p.Priority != 13 {
		t.Errorf("class priority = (%d, %d), want high (13, -10)", p.Priority, p.Nice)
	}

	// A row whose base priority is not a class base gets its priority back.
	q := procRow{PID: 9, Name: "app.exe", BasePrio: 99}.process()
	if q.HasPriority() {
		t.Fatal("base priority 99 must not be treated as known")
	}
	q = q.withDetail(procDetail{Class: PriorityIdle, HasClass: true})
	if !q.HasPriority() || q.Nice != 19 || q.Priority != 4 {
		t.Errorf("class priority = (%d, %d) available=%v, want idle (4, 19)", q.Priority, q.Nice, q.HasPriority())
	}
}

func TestProcDetailWithoutImageKeepsTheName(t *testing.T) {
	p := procRow{PID: 7, Name: "app.exe"}.process().withDetail(procDetail{WorkingSet: 1})
	if p.Cmdline != "app.exe" {
		t.Errorf("cmdline = %q, want the enumeration name", p.Cmdline)
	}
}

func TestHundredNsToTicks(t *testing.T) {
	tests := []struct{ hns, want uint64 }{
		{0, 0},
		{10_000_000, 100},            // one second
		{1_000_000, 10},              // one tick
		{99_999, 0},                  // below one tick
		{360_000_000_000, 3_600_000}, // ten hours
	}
	for _, tt := range tests {
		if got := hundredNsToTicks(tt.hns); got != tt.want {
			t.Errorf("hundredNsToTicks(%d) = %d, want %d", tt.hns, got, tt.want)
		}
	}
}

func TestSameProcessDetectsRecycledPID(t *testing.T) {
	if !sameProcess(Process{PID: 5}, Process{PID: 5}) {
		t.Error("same pid without identities must match")
	}
	if !sameProcess(Process{PID: 5, Identity: 9}, Process{PID: 5}) {
		t.Error("a snapshot without an identity must match")
	}
	if !sameProcess(Process{PID: 5, Identity: 9}, Process{PID: 5, Identity: 9}) {
		t.Error("equal identities must match")
	}
	if sameProcess(Process{PID: 5, Identity: 9}, Process{PID: 5, Identity: 10}) {
		t.Error("a different identity is a recycled pid, not the same process")
	}
	if sameProcess(Process{PID: 5}, Process{PID: 6}) {
		t.Error("different pids never match")
	}
}

func TestCalcCPUPercentIsPerCore(t *testing.T) {
	prev := []Process{{PID: 1, UTime: 100, Identity: 1}}
	cur := []Process{{PID: 1, UTime: 300, Identity: 1}}
	// 200 ticks = 2 s of CPU over a 2 s window: one core fully busy.
	CalcCPUPercent(prev, cur, 2)
	if got := cur[0].CPUPct; got < 99.9 || got > 100.1 {
		t.Errorf("cpu%% = %f, want 100 (Irix per-core scaling)", got)
	}
}

func TestCalcCPUPercentIgnoresGoneRecycledAndUnreadableProcs(t *testing.T) {
	base := []Process{{PID: 1, UTime: 100, Identity: 1}, {PID: 2, UTime: 500, Identity: 2}}

	t.Run("gone", func(t *testing.T) {
		cur := []Process{{PID: 2, UTime: 600, Identity: 2}}
		CalcCPUPercent(base, cur, 1)
		if cur[0].CPUPct == 0 {
			t.Error("a surviving process should have a percentage")
		}
	})

	t.Run("recycled pid", func(t *testing.T) {
		cur := []Process{{PID: 1, UTime: 700, Identity: 99}}
		CalcCPUPercent(base, cur, 1)
		if cur[0].CPUPct != 0 {
			t.Errorf("recycled pid cpu%% = %f, want 0", cur[0].CPUPct)
		}
	})

	t.Run("counter went backwards", func(t *testing.T) {
		cur := []Process{{PID: 1, UTime: 50, Identity: 1}}
		CalcCPUPercent(base, cur, 1)
		if cur[0].CPUPct != 0 {
			t.Errorf("backwards counter cpu%% = %f, want 0", cur[0].CPUPct)
		}
	})

	t.Run("no cpu time", func(t *testing.T) {
		cur := []Process{{PID: 1, UTime: 900, Identity: 1, Missing: MissingCPUTime}}
		CalcCPUPercent(base, cur, 1)
		if cur[0].CPUPct != 0 {
			t.Errorf("unreadable cpu time cpu%% = %f, want 0", cur[0].CPUPct)
		}
	})

	t.Run("no elapsed time", func(t *testing.T) {
		cur := []Process{{PID: 1, UTime: 900, Identity: 1}}
		CalcCPUPercent(base, cur, 0)
		if cur[0].CPUPct != 0 {
			t.Errorf("zero window cpu%% = %f, want 0", cur[0].CPUPct)
		}
	})
}

func TestTakeSnapshotIsStamped(t *testing.T) {
	// Portable guard for the snapshot contract every skin relies on: a fresh
	// snapshot is stamped, so views can compute a delta window.
	snap := TakeSnapshot()
	if snap.Time.IsZero() {
		t.Error("snapshot has no timestamp")
	}
	if snap.Time.After(time.Now().Add(time.Second)) {
		t.Error("snapshot timestamp is in the future")
	}
}
