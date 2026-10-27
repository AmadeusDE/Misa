package term

import (
	"fmt"
	"strings"
	"testing"
)

// benchOutput builds a stream like the frontend benchmarks: lines that
// scroll, so scrollback fills and recycles.
func benchOutput(kind string) []byte {
	var sb strings.Builder
	for i := 0; sb.Len() < 4<<20; i++ {
		switch kind {
		case "ascii":
			fmt.Fprintf(&sb, "%-99d\n", i*7919)
		case "color":
			for w := 0; w < 12; w++ {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dmword%d ", i%256, w*20, (i+w)%256, w)
			}
			sb.WriteString("\x1b[m\n")
		case "unicode":
			sb.WriteString(strings.Repeat("世界│─╭⣿é", 8) + "\n")
		case "marks":
			sb.WriteString(strings.Repeat("éà ", 20) + "\n")
		}
	}
	return []byte(sb.String())
}

func feed(tm *Terminal, data []byte) {
	for i := 0; i < len(data); i += 32 << 10 {
		tm.processChunk(data[i:min(i+32<<10, len(data))])
	}
}

// BenchmarkProcessOutput measures a terminal whose scrollback is already
// full, the steady state of long output.
func BenchmarkProcessOutput(b *testing.B) {
	for _, kind := range []string{"ascii", "color", "unicode", "marks"} {
		data := benchOutput(kind)
		b.Run(kind, func(b *testing.B) {
			tm := New(32, 110)
			feed(tm, data)
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				feed(tm, data)
			}
		})
	}
}

// BenchmarkProcessOutputFresh includes filling an empty scrollback.
func BenchmarkProcessOutputFresh(b *testing.B) {
	data := benchOutput("ascii")
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		feed(New(32, 110), data)
	}
}
