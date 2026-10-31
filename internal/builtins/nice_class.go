package builtins

// Windows priority classes (CreateProcess flags) and the nice value each one
// stands for. The mapping lives in an untagged file so every platform tests
// it; only the Windows files use it.
const (
	classIdle        uint32 = 0x00000040
	classBelowNormal uint32 = 0x00004000
	classNormal      uint32 = 0x00000020
	classAboveNormal uint32 = 0x00008000
	classHigh        uint32 = 0x00000080
)

// priorityClassForNice picks the class for a resulting nice value: 15 and up
// is Idle, 1 to 14 Below normal, 0 Normal, -1 to -9 Above normal, -10 and
// below High. Real-time is never chosen.
func priorityClassForNice(nice int) uint32 {
	switch {
	case nice >= 15:
		return classIdle
	case nice >= 1:
		return classBelowNormal
	case nice == 0:
		return classNormal
	case nice > -10:
		return classAboveNormal
	default:
		return classHigh
	}
}

// niceForPriorityClass is the inverse used to report the current value. A
// class outside the mapping (real-time) reports as High.
func niceForPriorityClass(class uint32) int {
	switch class {
	case classIdle:
		return 19
	case classBelowNormal:
		return 10
	case classAboveNormal:
		return -5
	case classNormal:
		return 0
	default:
		return -10
	}
}
