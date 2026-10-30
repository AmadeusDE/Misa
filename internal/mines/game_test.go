package mines

import (
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// namedKeys are the keys a terminal sends without any text.
var namedKeys = map[string]rune{
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
	"home":      tea.KeyHome,
	"end":       tea.KeyEnd,
	"pgup":      tea.KeyPgUp,
	"pgdown":    tea.KeyPgDown,
	"enter":     tea.KeyEnter,
	"tab":       tea.KeyTab,
	"esc":       tea.KeyEscape,
	"backspace": tea.KeyBackspace,
	"delete":    tea.KeyDelete,
}

// pressMsg builds the key press a terminal would deliver for a key name: single
// characters arrive with their text (so "[" and "{" read as themselves), named
// keys use their bubbletea code, and a shift+ prefix adds the modifier. Shifted
// punctuation keeps its text empty so msg.String() reports "shift+[".
func pressMsg(k string) tea.KeyPressMsg {
	switch k {
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	if code, ok := namedKeys[k]; ok {
		return tea.KeyPressMsg{Code: code}
	}
	if name, ok := strings.CutPrefix(k, "shift+"); ok {
		if code, ok := namedKeys[name]; ok {
			return tea.KeyPressMsg{Code: code, Mod: tea.ModShift}
		}
		if r := []rune(name); len(r) == 1 {
			return tea.KeyPressMsg{Code: r[0], Mod: tea.ModShift}
		}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

func press(g *Game, keys ...string) {
	for _, k := range keys {
		g.Update(pressMsg(k))
	}
}

func click(g *Game, x, y int, button tea.MouseButton) tea.Cmd {
	return g.Update(tea.MouseClickMsg{X: x, Y: y, Button: button})
}

func sized(level Level, w, h int) *Game {
	g := New(level)
	g.Resize(w, h)
	return g
}

func TestCursorMovesAndClamps(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "h", "h", "h", "h", "h", "h", "k", "up", "up", "up", "up", "up")
	if x, y := g.Cursor(); x != 0 || y != 0 {
		t.Fatalf("cursor = %d,%d, want 0,0", x, y)
	}
	press(g, "l", "j")
	if x, y := g.Cursor(); x != 1 || y != 1 {
		t.Fatalf("cursor = %d,%d, want 1,1", x, y)
	}
}

func TestSpaceRevealsUnderCursor(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "space")
	if !g.Board().Revealed(4, 4) {
		t.Fatal("cell under cursor not revealed")
	}
}

func TestFlagKey(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "f")
	if !g.Board().Flagged(4, 4) {
		t.Fatal("flag not set")
	}
}

func TestLevelKeys(t *testing.T) {
	g := sized(Levels[0], 120, 40)
	press(g, "3")
	if g.Board().Width() != 30 || g.Board().Mines() != 99 {
		t.Fatalf("board %dx%d/%d, want expert", g.Board().Width(), g.Board().Height(), g.Board().Mines())
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "esc"} {
		g := sized(Levels[0], 80, 24)
		press(g, k)
		if !g.IsQuit() {
			t.Fatalf("%s did not quit", k)
		}
	}
}

func TestMouseHitsCell(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	x, y := l.left+1+2*l.cw+1, l.top+2+3
	click(g, x, y, tea.MouseLeft)
	if !g.Board().Revealed(2, 3) {
		t.Fatalf("click at %d,%d did not reveal 2,3", x, y)
	}
	if cx, cy := g.Cursor(); cx != 2 || cy != 3 {
		t.Fatalf("cursor = %d,%d, want 2,3", cx, cy)
	}
}

func TestRightClickFlags(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	click(g, l.left+1, l.top+2, tea.MouseRight)
	if !g.Board().Flagged(0, 0) {
		t.Fatal("right click did not flag")
	}
}

func TestClickOnBorderIgnored(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	click(g, l.left, l.top+2, tea.MouseLeft)
	if g.Board().Started() {
		t.Fatal("click on border started the game")
	}
}

func TestFaceRestarts(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "space")
	old := g.Board()
	click(g, g.layout().faceX(), g.layout().top, tea.MouseLeft)
	if g.Board() == old || g.Board().Started() {
		t.Fatal("face click did not start a new game")
	}
}

func TestHeaderPutsFaceWhereHitTestLooks(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	l := g.layout()
	line := strings.Split(g.View(), "\n")[l.top]
	plain := []rune(ansi.Strip(line))
	face := string(plain[l.faceX() : l.faceX()+faceWidth])
	if face != " :) " {
		t.Fatalf("face at hit-test position = %q", face)
	}
}

func TestBoardRowsAlignWithHitTest(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "f")
	l := g.layout()
	line := []rune(ansi.Strip(strings.Split(g.View(), "\n")[l.top+2+4]))
	_, x, _ := l.hit(l.left+1+4*l.cw+1, l.top+2+4)
	if x != 4 || string(line[l.left+1+4*l.cw+1]) != "⚑" {
		t.Fatalf("flag not drawn at cell 4: %q", string(line))
	}
}

func TestClockRunsFromFirstReveal(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	now := time.Unix(1000, 0)
	g.now = func() time.Time { return now }
	if cmd := g.Update(tea.KeyPressMsg{Code: 'f', Text: "f"}); cmd != nil {
		t.Fatal("flag before first reveal started the clock")
	}
	if cmd := g.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}); cmd == nil && g.Board().State() == Playing {
		t.Fatal("first reveal did not start ticking")
	}
	now = now.Add(5 * time.Second)
	if g.Board().State() == Playing && g.Elapsed() != 5*time.Second {
		t.Fatalf("elapsed = %v, want 5s", g.Elapsed())
	}
}

func TestStaleTickIgnored(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	press(g, "space")
	gen := g.gen
	press(g, "n")
	if cmd := g.Update(tickMsg{gen}); cmd != nil {
		t.Fatal("tick from the previous game kept ticking")
	}
}

func TestNarrowTerminalUsesTwoColumnCells(t *testing.T) {
	g := sized(Levels[2], 80, 24)
	if l := g.layout(); l.cw != 2 || !l.fits {
		t.Fatalf("expert on 80x24: cw=%d fits=%v", l.cw, l.fits)
	}
}

func TestTooSmall(t *testing.T) {
	// 40x10 is big enough for a scrolling viewport; neither a 5-column nor a
	// 7-row terminal can show the minimum three-cell viewport.
	for _, size := range [][2]int{{5, 7}, {40, 7}} {
		g := sized(Levels[2], size[0], size[1])
		l := g.layout()
		if l.viable || l.scrolls() {
			t.Fatalf("%dx%d: layout is viable: %+v", size[0], size[1], l)
		}
		for _, pt := range [][2]int{{0, 0}, {size[0] - 1, size[1] - 1}} {
			if target, _, _ := l.hit(pt[0], pt[1]); target != hitNone {
				t.Fatalf("%dx%d: hit test active at %d,%d while board is hidden", size[0], size[1], pt[0], pt[1])
			}
		}
		for _, line := range strings.Split(g.View(), "\n") {
			if lipgloss.Width(line) > size[0] {
				t.Fatalf("%dx%d: too-small view is %d wide", size[0], size[1], lipgloss.Width(line))
			}
		}
	}
	// A terminal wide enough for the text says what is missing.
	if g := sized(Levels[2], 40, 7); !strings.Contains(g.View(), "too small") {
		t.Fatalf("no too-small message: %q", g.View())
	}
}

func TestFortyByTenScrolls(t *testing.T) {
	g := sized(Levels[2], 40, 10)
	l := g.layout()
	if !l.viable || l.fits || !l.scrolls() {
		t.Fatalf("expert on 40x10: viable=%v fits=%v scrolls=%v", l.viable, l.fits, l.scrolls())
	}
	if l.cols*l.cw+l.pad+2 > 40 || l.rows > 10-chromeLines {
		t.Fatalf("viewport %dx%d does not fit 40x10", l.cols, l.rows)
	}
	if strings.Contains(g.View(), "too small") {
		t.Fatal("a scrollable board was reported as too small")
	}
}

func TestViewFillsHeight(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	lines := strings.Split(g.View(), "\n")
	if len(lines) != 24 {
		t.Fatalf("view has %d lines, want 24", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 80 {
			t.Fatalf("line %d is %d wide", i, w)
		}
	}
}

// hugeLevel is a board far larger than any terminal: the whole board never fits
// and always plays through the scrolling viewport.
var hugeLevel = Level{"Huge", MaxWidth, MaxHeight, 20000}

// hugeGame builds a huge game with a seeded rng so a board that is only planted
// on the first reveal stays reproducible.
func hugeGame(w, h int) *Game {
	g := New(hugeLevel)
	g.rng = rand.New(rand.NewPCG(7, 11))
	g.newGame(hugeLevel)
	g.Resize(w, h)
	return g
}

func TestBigBoardGeneratesAndFloods(t *testing.T) {
	start := time.Now()
	g := hugeGame(80, 24)
	if w, h, m := g.Board().Width(), g.Board().Height(), g.Board().Mines(); w != MaxWidth || h != MaxHeight || m != 20000 {
		t.Fatalf("board = %dx%d/%d mines", w, h, m)
	}
	if x, y := g.Cursor(); x != MaxWidth/2 || y != MaxHeight/2 {
		t.Fatalf("cursor starts at %d,%d, want the centre", x, y)
	}
	// The first reveal floods the empty region around the cursor.
	press(g, "space")
	if st := g.Board().State(); st != Playing {
		t.Fatalf("state = %v after the first reveal, want Playing", st)
	}
	if !g.Board().Revealed(MaxWidth/2, MaxHeight/2) {
		t.Fatal("centre cell not revealed")
	}
	// The first reveal is mine-free in its 3x3 block, so the flood fill always
	// reaches at least those nine cells.
	if n := g.board.revealed; n < 9 {
		t.Fatalf("flood fill revealed %d cells, want at least 9", n)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("building and revealing a %d×%d board took %v", MaxWidth, MaxHeight, d)
	}
}

func TestBigBoardViewFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 10}, {120, 40}, {60, 15}} {
		g := hugeGame(size[0], size[1])
		press(g, "space")
		lines := strings.Split(g.View(), "\n")
		if len(lines) != size[1] {
			t.Fatalf("%dx%d: view has %d lines, want %d", size[0], size[1], len(lines), size[1])
		}
		for i, line := range lines {
			if w := lipgloss.Width(line); w > size[0] {
				t.Fatalf("%dx%d: line %d is %d wide", size[0], size[1], i, w)
			}
		}
		if l := g.layout(); !l.scrolls() {
			t.Fatalf("%dx%d: huge board is not scrolled: %+v", size[0], size[1], l)
		}
	}
}

func TestBigBoardCursorStaysInView(t *testing.T) {
	g := hugeGame(80, 24)
	keys := []string{"l", "l", "l", "j", "j", "h", "k", "right", "down", "left", "up"}
	for i := range 400 {
		press(g, keys[i%len(keys)])
		l := g.layout()
		cx, cy := g.Cursor()
		if cx < l.x0 || cx >= l.x0+l.cols || cy < l.y0 || cy >= l.y0+l.rows {
			t.Fatalf("after move %d (%q) cursor %d,%d is outside cols %d-%d rows %d-%d",
				i, keys[i%len(keys)], cx, cy, l.x0, l.x0+l.cols-1, l.y0, l.y0+l.rows-1)
		}
	}
}

func TestBigBoardScrollsAtEdges(t *testing.T) {
	g := hugeGame(80, 24)
	// Walk to the top-left corner: the viewport origin follows the cursor.
	for i := 0; ; i++ {
		x, y := g.Cursor()
		if x == 0 && y == 0 {
			break
		}
		if i > 2000 {
			t.Fatalf("cursor stuck at %d,%d", x, y)
		}
		press(g, "h", "k")
	}
	l := g.layout()
	if l.x0 != 0 || l.y0 != 0 {
		t.Fatalf("origin = %d,%d at the top-left corner, want 0,0", l.x0, l.y0)
	}
	// One column past the viewport advances it by exactly one cell.
	for range l.cols {
		press(g, "l")
	}
	if x, _ := g.Cursor(); x != l.cols {
		t.Fatalf("cursor = %d, want %d", x, l.cols)
	}
	if got := g.layout().x0; got != 1 {
		t.Fatalf("origin x = %d after one step past the viewport, want 1", got)
	}
	// The far edge scrolls all the way, keeping the last column visible.
	for i := 0; ; i++ {
		if x, y := g.Cursor(); x == MaxWidth-1 && y == MaxHeight-1 {
			break
		}
		if i > 4000 {
			x, y := g.Cursor()
			t.Fatalf("cursor stuck at %d,%d", x, y)
		}
		press(g, "l", "j")
	}
	l = g.layout()
	if l.x0 != l.totalCols-l.cols || l.y0 != l.totalRows-l.rows {
		t.Fatalf("origin = %d,%d at the far edge, want %d,%d",
			l.x0, l.y0, l.totalCols-l.cols, l.totalRows-l.rows)
	}
	if !strings.Contains(ansi.Strip(g.View()), "cols ") {
		t.Fatal("status line does not report the visible cells")
	}
}

func TestBigBoardClickUsesViewport(t *testing.T) {
	g := hugeGame(80, 24)
	// Scroll away from the origin so the click has to add the offset.
	for range 60 {
		press(g, "l")
	}
	for range 40 {
		press(g, "j")
	}
	l := g.layout()
	if !l.scrolls() || l.x0 == 0 || l.y0 == 0 {
		t.Fatalf("viewport %d,%d does not scroll: %+v", l.x0, l.y0, l)
	}
	// The screen coordinate of the third visible column and fourth visible row,
	// the way layout.hit maps it.
	wantX, wantY := l.x0+2, l.y0+3
	x, y := l.left+1+2*l.cw+1, l.top+2+3
	click(g, x, y, tea.MouseLeft)
	if !g.Board().Revealed(wantX, wantY) {
		t.Fatalf("click at %d,%d did not reveal board cell %d,%d", x, y, wantX, wantY)
	}
	if cx, cy := g.Cursor(); cx != wantX || cy != wantY {
		t.Fatalf("cursor = %d,%d after the click, want %d,%d", cx, cy, wantX, wantY)
	}
}

func TestBigBoardRendersFast(t *testing.T) {
	g := hugeGame(80, 24)
	press(g, "space")
	const renders = 5
	start := time.Now()
	for range renders {
		if view := g.View(); len(strings.Split(view, "\n")) != 24 {
			t.Fatal("wrong view height")
		}
		press(g, "l", "j")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("%d renders of a %d×%d board took %v, want under a second", renders, MaxWidth, MaxHeight, d)
	}
}
