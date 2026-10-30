package chess

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func wideViewGame(files, width int) *Game {
	v := &Variant{Name: "Wide", Files: files, Ranks: 2}
	p := &Position{V: v, Board: make([]Piece, files*2), EPPawn: -1}
	return &Game{match: &Match{variant: v, history: []*Position{p}}, width: width, height: 12, sel: -1}
}

func TestWideFileLabels(t *testing.T) {
	for _, flipped := range []bool{false, true} {
		g := wideViewGame(28, 100)
		g.flipped = flipped
		l := g.layout()
		if !l.fits || l.cw != 3 {
			t.Fatalf("layout = %+v", l)
		}
		lines := strings.Split(ansi.Strip(g.board(l)), "\n")
		labels := lines[len(lines)-1]
		for col := range l.cols {
			x, _ := g.boardXY(col, 0)
			cell := labels[l.labelW+1+col*l.cw : l.labelW+1+(col+1)*l.cw]
			want := string(rune('a' + x))
			if x == 26 {
				want = "aa"
			} else if x == 27 {
				want = "ab"
			}
			if strings.TrimSpace(cell) != want {
				t.Errorf("flipped=%v col=%d: %q", flipped, col, cell)
			}
			if _, ok := l.square(g, l.left+l.labelW+1+col*l.cw, l.top+2); !ok {
				t.Errorf("column %d not clickable", col)
			}
		}
	}
}

func TestWideBoardTooSmall(t *testing.T) {
	g := wideViewGame(27, 58) // 2-column cells would fit, but two-letter labels need 3.
	l := g.layout()
	if l.fits || l.cw != 3 || l.boardW != 85 {
		t.Fatalf("layout = %+v", l)
	}
	if !strings.Contains(ansi.Strip(g.View()), "Terminal too small") {
		t.Fatal("missing too-small fallback")
	}
	if _, ok := l.square(g, 3, l.top+2); ok {
		t.Fatal("oversized board is clickable")
	}
	g = wideViewGame(26, 58)
	if l = g.layout(); !l.fits || l.cw != 2 {
		t.Fatalf("compact layout = %+v", l)
	}
}
