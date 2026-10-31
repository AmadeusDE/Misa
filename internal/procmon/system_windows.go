//go:build windows

package procmon

import "time"

// The system-wide counters the Unix backend reads from procfs come from the
// native system information provider tracked as I03. Until it lands they are
// unavailable on Windows: Snapshot.SystemAvailable stays false and the views
// render "—" for uptime, load, memory and per-core CPU instead of showing
// zeros. Nothing here reads /proc or invents a value.

func ReadUptime() float64 { return 0 }

func ReadLoadAvg() [3]float64 { return [3]float64{} }

func ReadCPUTimes() (CPUTimes, []CPUTimes, bool) { return CPUTimes{}, nil, false }

func ReadMemInfo() MemInfo { return MemInfo{} }

func ReadNetDev() []NetDev { return nil }

func ReadDiskStats() []DiskStat { return nil }

func TakeSnapshot() Snapshot { return takeSnapshot() }

func TakeFullSnapshot() Snapshot { return takeSnapshot() }

// takeSnapshot collects the platform's view; the system-wide sections stay
// zero and unavailable (see the package comment above).
func takeSnapshot() Snapshot {
	return Snapshot{Time: time.Now(), Procs: ReadAllProcs()}
}
