package procmon

// PriorityClass is a Windows priority class. Unix priorities are continuous
// and use nice values directly, so this type is only used where a platform
// reports coarse classes (see docs/plans/windows.md, I04).
type PriorityClass int

const (
	PriorityIdle PriorityClass = iota
	PriorityBelowNormal
	PriorityNormal
	PriorityAboveNormal
	PriorityHigh
	PriorityRealtime
)

// priorityClassInfo maps a class to what the views show for it: the class's
// base priority (the value Windows process enumeration reports for PR) and a
// nice equivalent, so the NI column keeps its Unix range and can be stepped
// with the same keys. The base priorities are the documented Win32 values.
var priorityClassInfo = []struct {
	class PriorityClass
	nice  int
	base  int32
	name  string
}{
	{PriorityIdle, 19, 4, "Idle"},
	{PriorityBelowNormal, 10, 6, "Below normal"},
	{PriorityNormal, 0, 8, "Normal"},
	{PriorityAboveNormal, -5, 10, "Above normal"},
	{PriorityHigh, -10, 13, "High"},
	{PriorityRealtime, -20, 24, "Realtime"},
}

func (c PriorityClass) info() (nice int, base int32, name string, ok bool) {
	for _, i := range priorityClassInfo {
		if i.class == c {
			return i.nice, i.base, i.name, true
		}
	}
	return 0, 0, "", false
}

// Nice is the nice equivalent the NI column shows for the class.
func (c PriorityClass) Nice() int {
	nice, _, _, _ := c.info()
	return nice
}

// Base is the base priority Windows reports for the class.
func (c PriorityClass) Base() int32 {
	_, base, _, _ := c.info()
	return base
}

// String is the class name, as Windows spells it.
func (c PriorityClass) String() string {
	_, _, name, _ := c.info()
	return name
}

// PriorityClassOfBase maps a process's base priority (what Toolhelp reports as
// PriClassBase) to its class. The second result is false for a base priority
// that is not a class base.
func PriorityClassOfBase(base int32) (PriorityClass, bool) {
	for _, i := range priorityClassInfo {
		if i.base == base {
			return i.class, true
		}
	}
	return PriorityNormal, false
}

// PriorityLevel is one priority the UI can set. class is the platform class
// behind the level; it is only meaningful where the platform has classes.
type PriorityLevel struct {
	Nice  int
	Name  string
	class PriorityClass
}

// windowsPriorityLevels are the classes the process viewers offer, ordered
// from highest priority (lowest nice) to lowest. Realtime is deliberately
// absent: changing to it needs a privilege and letting any user's process run
// above the system's own threads is not a UI-level operation.
var windowsPriorityLevels = []PriorityLevel{
	{Nice: -10, Name: PriorityHigh.String(), class: PriorityHigh},
	{Nice: -5, Name: PriorityAboveNormal.String(), class: PriorityAboveNormal},
	{Nice: 0, Name: PriorityNormal.String(), class: PriorityNormal},
	{Nice: 10, Name: PriorityBelowNormal.String(), class: PriorityBelowNormal},
	{Nice: 19, Name: PriorityIdle.String(), class: PriorityIdle},
}

// PriorityLevelOf returns the level a nice value belongs to: the first level
// at or below that priority.
func PriorityLevelOf(levels []PriorityLevel, nice int) (PriorityLevel, bool) {
	if len(levels) == 0 {
		return PriorityLevel{}, false
	}
	for _, l := range levels {
		if nice <= l.Nice {
			return l, true
		}
	}
	return levels[len(levels)-1], true
}

// PriorityStep is the priority to set for one step up (delta < 0) or down
// (delta > 0) the priority scale. Without discrete levels it is a clamped nice
// value, which keeps the Unix behaviour of F7/F8 exactly as it was; with
// levels it moves to the neighbouring level.
func PriorityStep(levels []PriorityLevel, current, delta int) int {
	if delta == 0 {
		return current
	}
	if len(levels) == 0 {
		return clampNice(current + delta)
	}
	level, ok := PriorityLevelOf(levels, current)
	if !ok {
		return clampNice(current + delta)
	}
	idx := 0
	for i, l := range levels {
		if l == level {
			idx = i
			break
		}
	}
	if delta < 0 {
		idx--
	} else {
		idx++
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(levels) {
		idx = len(levels) - 1
	}
	return levels[idx].Nice
}

// clampNice keeps a nice value in the POSIX range every view and action uses.
func clampNice(nice int) int {
	if nice < -20 {
		return -20
	}
	if nice > 19 {
		return 19
	}
	return nice
}
