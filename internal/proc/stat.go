package proc

import (
	"errors"
	"strconv"
	"strings"
)

// CPUStat is one row of /proc/stat CPU times in USER_HZ ticks (ClockTicks).
type CPUStat struct {
	User, Nice, System, Idle, Iowait, Irq, Softirq, Steal, Guest, GuestNice uint64
}

// Total returns the summed CPU time, which is the denominator for the vmstat
// percentages. Guest time is already part of User/System (the kernel adds it),
// so the guest fields are deliberately left out.
func (c CPUStat) Total() uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.Iowait + c.Irq + c.Softirq + c.Steal
}

// Sub returns c minus o, per field.
func (c CPUStat) Sub(o CPUStat) CPUStat {
	return CPUStat{
		User:      c.User - o.User,
		Nice:      c.Nice - o.Nice,
		System:    c.System - o.System,
		Idle:      c.Idle - o.Idle,
		Iowait:    c.Iowait - o.Iowait,
		Irq:       c.Irq - o.Irq,
		Softirq:   c.Softirq - o.Softirq,
		Steal:     c.Steal - o.Steal,
		Guest:     c.Guest - o.Guest,
		GuestNice: c.GuestNice - o.GuestNice,
	}
}

// Stat is the kernel-wide part of /proc/stat. The per-CPU rows are not read:
// no builtin needs them yet.
type Stat struct {
	CPU          CPUStat
	Intr         uint64 // interrupts since boot
	Ctxt         uint64 // context switches since boot
	ProcsRunning uint64
	ProcsBlocked uint64
	// BootTime is the btime line: seconds since the epoch at boot, or 0 when
	// the kernel does not report it.
	BootTime int64
}

// ParseStat parses /proc/stat. The aggregate "cpu" row is required; every other
// line is best effort, so a kernel without btime or procs_running still yields
// a usable snapshot.
func ParseStat(data []byte) (Stat, error) {
	var (
		st      Stat
		cpuSeen bool
	)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "cpu":
			cols := fields[1:]
			if len(cols) < 4 {
				return Stat{}, errors.New("proc: bad cpu line")
			}
			vals := make([]uint64, len(cols))
			for i, col := range cols {
				v, err := strconv.ParseUint(col, 10, 64)
				if err != nil {
					return Stat{}, errors.New("proc: bad cpu line")
				}
				vals[i] = v
			}
			at := func(i int) uint64 {
				if i < len(vals) {
					return vals[i]
				}
				return 0
			}
			// user nice system idle iowait irq softirq steal guest guest_nice
			st.CPU = CPUStat{
				User: at(0), Nice: at(1), System: at(2), Idle: at(3), Iowait: at(4),
				Irq: at(5), Softirq: at(6), Steal: at(7), Guest: at(8), GuestNice: at(9),
			}
			cpuSeen = true
		case "intr":
			st.Intr = atou(fields[1])
		case "ctxt":
			st.Ctxt = atou(fields[1])
		case "btime":
			st.BootTime = int64(atou(fields[1]))
		case "procs_running":
			st.ProcsRunning = atou(fields[1])
		case "procs_blocked":
			st.ProcsBlocked = atou(fields[1])
		}
	}
	if !cpuSeen {
		return Stat{}, errors.New("proc: no cpu statistics")
	}
	return st, nil
}
