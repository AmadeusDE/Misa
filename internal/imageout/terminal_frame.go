package imageout

import (
	"fmt"
	"sort"
	"strings"
)

// TerminalFrame constructs one synchronized standalone terminal update. Rows are
// zero-based and contain trusted, already styled text. Graphics deletes precede
// text and graphics transmissions follow it.
func TerminalFrame(rows map[int]string, clear bool, before, after []byte) []byte {
	if !clear && len(rows) == 0 && len(before) == 0 && len(after) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("\x1b[?2026h")
	b.Write(before)
	if clear {
		b.WriteString("\x1b[2J")
	}
	keys := make([]int, 0, len(rows))
	for n := range rows {
		keys = append(keys, n)
	}
	sort.Ints(keys)
	for _, n := range keys {
		fmt.Fprintf(&b, "\x1b[%d;1H", n+1)
		b.WriteString(rows[n])
		b.WriteString("\x1b[K")
	}
	b.Write(after)
	b.WriteString("\x1b[?2026l")
	return []byte(b.String())
}
