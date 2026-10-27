package gui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/term"
)

const benchRows, benchCols = 32, 110

// benchScreen is a busy TUI-like screen: colored text, box drawing and CJK.
func benchScreen(seed int) *term.Snapshot {
	var sb strings.Builder
	for row := 0; row < benchRows; row++ {
		fmt.Fprintf(&sb, "\x1b[%dH", row+1)
		for col := 0; col < benchCols/10; col++ {
			n := seed + row*31 + col*7
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%s", n%256, n*3%256, n*7%256, []string{"misa rend ", "│ box ─╭╮ ", "世界 é ⣿▄ ", "\x1b[1mbold\x1b[22m text"}[n%4])
		}
	}
	return screen(benchRows, benchCols, sb.String())
}

func benchSetup(b *testing.B, opacity float64) (*renderer, *canvas) {
	r := testRenderer(b)
	r.pal.alpha = opacity255(opacity)
	return r, canvasFor(r, benchRows, benchCols)
}

func opacity255(o float64) uint8 { return opacity(&o) }

func BenchmarkDrawFullFrame(b *testing.B) {
	for _, op := range []float64{1, 0.55} {
		b.Run(fmt.Sprintf("opacity=%v", op), func(b *testing.B) {
			r, c := benchSetup(b, op)
			s := benchScreen(0)
			cur := cursorState{visible: true, focused: true}
			var warm frame
			r.draw(c, s, cur, &warm)
			b.ResetTimer()
			for b.Loop() {
				var prev frame
				r.draw(c, s, cur, &prev)
			}
		})
	}
}

func BenchmarkDrawOneRowChanged(b *testing.B) {
	r, c := benchSetup(b, 1)
	screens := [2]*term.Snapshot{benchScreen(0), benchScreen(0)}
	screens[1].Cells[5*benchCols].Ch = 'X'
	cur := cursorState{visible: true, focused: true}
	var prev frame
	r.draw(c, screens[0], cur, &prev)
	b.ResetTimer()
	i := 0
	for b.Loop() {
		i++
		r.draw(c, screens[i%2], cur, &prev)
	}
}

func BenchmarkSnapshotAndDiff(b *testing.B) {
	tm := term.New(benchRows, benchCols)
	tm.ProcessOutput([]byte(strings.Repeat("\x1b[31mhello\x1b[m world ", 400)))
	var s term.Snapshot
	var prev frame
	tm.Snapshot(&s)
	prev.record(&s, cursorState{})
	for b.Loop() {
		tm.Snapshot(&s)
		changedRows(&s, cursorState{}, &prev)
	}
}
