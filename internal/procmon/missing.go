package procmon

import "fmt"

// unknown marks a single per-process field the platform did not report, the
// same convention lscore uses for unavailable file metadata. A whole section
// that a platform cannot report uses unavailable ("—") instead. Neither is a
// fabricated zero.
const unknown = "?"

// ProcMissing flags the per-process fields the platform could not read; zero
// means every field is available.
type ProcMissing uint16

const (
	MissingUser ProcMissing = 1 << iota
	MissingState
	MissingCPUTime
	MissingMemory // resident size and memory percentage source
	MissingVirt
	MissingShared
	MissingMemPct
	MissingPriority
)

func (p Process) has(f ProcMissing) bool { return p.Missing&f == 0 }

// HasUser reports whether the owning user is known.
func (p Process) HasUser() bool { return p.has(MissingUser) }

// HasState reports whether the run state is known.
func (p Process) HasState() bool { return p.has(MissingState) }

// HasCPUTime reports whether CPU times (and so %CPU and TIME+) are known.
func (p Process) HasCPUTime() bool { return p.has(MissingCPUTime) }

// HasMemory reports whether the resident size is known.
func (p Process) HasMemory() bool { return p.has(MissingMemory) }

// HasVirt reports whether the virtual size is known.
func (p Process) HasVirt() bool { return p.has(MissingVirt) }

// HasShared reports whether the shared size is known.
func (p Process) HasShared() bool { return p.has(MissingShared) }

// HasMemPct reports whether %MEM is meaningful, which needs the system memory
// total as well as the resident size.
func (p Process) HasMemPct() bool { return p.has(MissingMemPct) }

// HasPriority reports whether the priority and nice-equivalent columns are
// known.
func (p Process) HasPriority() bool { return p.has(MissingPriority) }

// CellText renders s, or the unknown marker when the platform did not report
// the field.
func CellText(s string, ok bool) string {
	if !ok {
		return unknown
	}
	return s
}

// CellTextLeft renders s right-padded to width, or the unknown marker
// right-aligned in it.
func CellTextLeft(s string, ok bool, width int) string {
	if !ok {
		s = unknown
	}
	return fmt.Sprintf("%-*s", width, s)
}

// CellTextRight renders s right-aligned to width, or the unknown marker.
func CellTextRight(s string, ok bool, width int) string {
	if !ok {
		s = unknown
	}
	return fmt.Sprintf("%*s", width, s)
}

// CellInt renders a right-aligned integer column, or the unknown marker.
func CellInt(v int64, ok bool, width int) string {
	if !ok {
		return fmt.Sprintf("%*s", width, unknown)
	}
	return fmt.Sprintf("%*d", width, v)
}

// CellFloat renders a right-aligned float column, or the unknown marker.
func CellFloat(v float64, ok bool, width, prec int) string {
	if !ok {
		return fmt.Sprintf("%*s", width, unknown)
	}
	return fmt.Sprintf("%*.*f", width, prec, v)
}

// CellBytes renders a right-aligned byte size, or the unknown marker.
func CellBytes(v uint64, ok bool, width int) string {
	if !ok {
		return fmt.Sprintf("%*s", width, unknown)
	}
	return fmt.Sprintf("%*s", width, FmtBytes(v))
}
