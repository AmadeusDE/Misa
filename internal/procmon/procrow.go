package procmon

import "strconv"

// procRow is one native process table entry: what enumeration reports about a
// process before any per-process detail is read. On Windows it comes from a
// Toolhelp snapshot, which succeeds even for processes whose details are
// protected (PID 0, the System process, other users' processes).
type procRow struct {
	PID      int
	PPID     int
	Name     string
	Threads  int
	BasePrio int32 // priority class base priority; 0 when unknown
}

// procDetail is the per-process information read after enumeration, from
// GetProcessTimes and the process memory counters.
type procDetail struct {
	Kernel100ns uint64        // kernel CPU time, 100 ns units
	User100ns   uint64        // user CPU time, 100 ns units
	Identity    uint64        // process creation time; identifies the instance
	WorkingSet  uint64        // resident bytes
	Commit      uint64        // committed bytes, the closest Windows VIRT equivalent
	Image       string        // full executable path, empty when it cannot be read
	Class       PriorityClass // exact priority class, when the platform reports one
	HasClass    bool
}

// process turns an enumeration row into a Process whose platform-dependent
// fields are all marked missing, so no view shows a fabricated zero for them.
func (r procRow) process() Process {
	name := r.Name
	if name == "" {
		name = "[" + strconv.Itoa(r.PID) + "]"
	}
	p := Process{
		PID:     r.PID,
		PPID:    r.PPID,
		Comm:    name,
		Cmdline: name,
		Threads: r.Threads,
		Missing: MissingUser | MissingState | MissingCPUTime | MissingMemory |
			MissingVirt | MissingShared | MissingMemPct,
	}
	if c, ok := PriorityClassOfBase(r.BasePrio); ok {
		p.Priority = int64(c.Base())
		p.Nice = int64(c.Nice())
	} else {
		p.Missing |= MissingPriority
	}
	return p
}

// withDetail applies the detail read to a process and clears the fields it
// supplies.
func (p Process) withDetail(d procDetail) Process {
	p.UTime = hundredNsToTicks(d.User100ns)
	p.STime = hundredNsToTicks(d.Kernel100ns)
	p.Identity = d.Identity
	p.VSize = d.Commit
	p.RSS = int64((d.WorkingSet + PageSize - 1) / PageSize)
	p.Missing &^= MissingCPUTime | MissingMemory | MissingVirt
	if d.Image != "" {
		p.Cmdline = d.Image
	}
	// The class read with the process handle is authoritative; the base
	// priority from enumeration is the fallback for protected processes.
	if d.HasClass {
		p.Priority = int64(d.Class.Base())
		p.Nice = int64(d.Class.Nice())
		p.Missing &^= MissingPriority
	}
	return p
}

// hundredNsToTicks converts 100-nanosecond units, the unit Windows reports CPU
// times in, to the ticks every view formats, so CPU time has one unit on every
// platform.
func hundredNsToTicks(hns uint64) uint64 {
	return hns * ClkTck / 10_000_000
}

// sameProcess reports whether two snapshots describe the same process
// instance. PIDs are recycled quickly on Windows, and the CPU time of the
// previous owner of a PID must not be subtracted from the new process's.
func sameProcess(a, b Process) bool {
	if a.PID != b.PID {
		return false
	}
	return a.Identity == 0 || b.Identity == 0 || a.Identity == b.Identity
}
