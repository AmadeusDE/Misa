package term

import "sync"

// maxMarkSets bounds the combining-mark table; marks past it are dropped
// rather than letting hostile output grow it without end.
const maxMarkSets = 1 << 16

// maxMarksPerCell caps the marks stacked on one cell. Without it each added
// mark interns a new, longer string, and the table could hold gigabytes.
const maxMarksPerCell = 8

// markTable interns the combining marks drawn over a cell, so a Cell holds
// only an id and stays free of pointers: the garbage collector then never
// scans screens or scrollback. Id 0 is no marks.
var markTable = struct {
	sync.RWMutex
	ids  map[string]uint32
	sets []string
}{ids: map[string]uint32{}, sets: []string{""}}

func internMarks(s string) uint32 {
	if s == "" {
		return 0
	}
	markTable.RLock()
	id, ok := markTable.ids[s]
	markTable.RUnlock()
	if ok {
		return id
	}
	markTable.Lock()
	defer markTable.Unlock()
	if id, ok := markTable.ids[s]; ok {
		return id
	}
	if len(markTable.sets) >= maxMarkSets {
		return 0
	}
	id = uint32(len(markTable.sets))
	markTable.sets = append(markTable.sets, s)
	markTable.ids[s] = id
	return id
}

func marksOf(id uint32) string {
	if id == 0 {
		return ""
	}
	markTable.RLock()
	defer markTable.RUnlock()
	return markTable.sets[id]
}

// Marks are the combining characters drawn over the cell's character.
func (c *Cell) Marks() string { return marksOf(c.marks) }
