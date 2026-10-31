package proc

import (
	"errors"
	"strconv"
	"strings"
)

// Meminfo is one /proc/meminfo snapshot. Every value is in kibibytes, exactly
// as the kernel reports it. A field the kernel does not provide stays zero and
// the corresponding Known flag is false, so a caller can tell "0 kB" from "the
// kernel has no such line".
type Meminfo struct {
	MemTotal, MemFree, MemAvailable uint64
	Buffers, Cached, Shmem          uint64
	SReclaimable                    uint64
	SwapTotal, SwapFree             uint64
	Active, Inactive                uint64
	HighTotal, HighFree             uint64
	LowTotal, LowFree               uint64
	CommitLimit, CommittedAS        uint64

	// AvailableKnown reports whether the MemAvailable line exists. Kernels
	// before 3.14 do not have it; a zero MemAvailable then means unknown, not
	// "no memory available".
	AvailableKnown bool
	// ActiveKnown and InactiveKnown report the Active/Inactive lines, which
	// vmstat -a displays.
	ActiveKnown, InactiveKnown bool
	// SwapKnown reports whether SwapTotal/SwapFree were read (not every kernel
	// or Android device reports them).
	SwapKnown bool
}

// Available returns the memory a new workload can use: MemAvailable when the
// kernel reports it, otherwise the pre-3.14 estimate used elsewhere in Misa
// (free + reclaimable page cache + buffers + shmem), saturating at zero.
func (m Meminfo) Available() uint64 {
	if m.AvailableKnown {
		return m.MemAvailable
	}
	usable := m.MemFree + m.Buffers + m.Cached + m.SReclaimable
	if usable < m.Shmem {
		return 0
	}
	return usable - m.Shmem
}

// ParseMeminfo parses a /proc/meminfo file. Lines without a decimal value
// (HugePages_Total and friends) are ignored; unreadable content reports an
// error rather than an all-zero snapshot.
func ParseMeminfo(data []byte) (Meminfo, error) {
	var m Meminfo
	seen := false
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			m.MemTotal = n
		case "MemFree":
			m.MemFree = n
		case "MemAvailable":
			m.MemAvailable, m.AvailableKnown = n, true
		case "Buffers":
			m.Buffers = n
		case "Cached":
			m.Cached = n
		case "SReclaimable":
			m.SReclaimable = n
		case "Shmem":
			m.Shmem = n
		case "SwapTotal":
			m.SwapTotal, m.SwapKnown = n, true
		case "SwapFree":
			m.SwapFree, m.SwapKnown = n, true
		case "Active":
			m.Active, m.ActiveKnown = n, true
		case "Inactive":
			m.Inactive, m.InactiveKnown = n, true
		case "HighTotal":
			m.HighTotal = n
		case "HighFree":
			m.HighFree = n
		case "LowTotal":
			m.LowTotal = n
		case "LowFree":
			m.LowFree = n
		case "CommitLimit":
			m.CommitLimit = n
		case "Committed_AS":
			m.CommittedAS = n
		default:
			continue
		}
		seen = true
	}
	if !seen {
		return Meminfo{}, errors.New("proc: no memory information")
	}
	return m, nil
}
