package battleship

import (
	"math/rand/v2"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func press(g *Game, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "left":
			msg = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		g.Update(msg)
	}
}

// moveTo walks the cursor to a square with the arrow keys.
func moveTo(g *Game, x, y int) {
	cx, cy := g.Cursor()
	for ; cx > x; cx-- {
		press(g, "left")
	}
	for ; cx < x; cx++ {
		press(g, "right")
	}
	for ; cy > y; cy-- {
		press(g, "up")
	}
	for ; cy < y; cy++ {
		press(g, "down")
	}
}

func click(g *Game, x, y int, button tea.MouseButton) tea.Cmd {
	return g.Update(tea.MouseClickMsg{X: x, Y: y, Button: button})
}

func sized(opts Options, w, h int) *Game {
	g := New(opts)
	g.Resize(w, h)
	return g
}

func firedCount(b *Board) int {
	n := 0
	for y := range Size {
		for x := range Size {
			if b.Fired(x, y) {
				n++
			}
		}
	}
	return n
}

// waterSquare is the first unfired enemy square with no ship on it, so a test
// can shoot back and forth without ever winning.
func waterSquare(b *Board) (int, int, bool) {
	for y := range Size {
		for x := range Size {
			if !b.Fired(x, y) && b.ShipAt(x, y) == nil {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// sink fires at every enemy ship square. The pending reply is dropped between
// shots: the computer's answer has its own test, and this one only wants the
// player's moves.
func sink(g *Game) {
	for _, ship := range g.Enemy().Ships() {
		for _, p := range ship.Cells() {
			moveTo(g, p.X, p.Y)
			press(g, "space")
			g.pending = false
		}
	}
}

func TestRandomModeStartsFiring(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	if g.Phase() != Firing {
		t.Fatalf("phase = %v, want firing", g.Phase())
	}
	if n := len(g.Player().Ships()); n != len(Fleet) {
		t.Fatalf("%d player ships, want %d", n, len(Fleet))
	}
	occupied := 0
	for y := range Size {
		for x := range Size {
			if g.Player().ShipAt(x, y) != nil {
				occupied++
			}
		}
	}
	if occupied != 17 {
		t.Fatalf("%d occupied squares, want 17", occupied)
	}
	if n := len(g.Enemy().Ships()); n != len(Fleet) {
		t.Fatalf("%d enemy ships, want %d", n, len(Fleet))
	}
}

func TestCursorClampsToBoard(t *testing.T) {
	g := sized(Options{}, 80, 24)
	press(g, "left", "left", "left", "up", "up", "up")
	if x, y := g.Cursor(); x != 0 || y != 0 {
		t.Fatalf("cursor = %d,%d, want 0,0", x, y)
	}
	press(g, "right", "down")
	if x, y := g.Cursor(); x != 1 || y != 1 {
		t.Fatalf("cursor = %d,%d, want 1,1", x, y)
	}
	for range Size + 5 {
		press(g, "right", "down")
	}
	if x, y := g.Cursor(); x != Size-1 || y != Size-1 {
		t.Fatalf("cursor = %d,%d, want %d,%d", x, y, Size-1, Size-1)
	}
}

func TestRotateKeyFlipsShip(t *testing.T) {
	g := sized(Options{}, 80, 24)
	press(g, "r", "space")
	ships := g.Player().Ships()
	if len(ships) != 1 {
		t.Fatalf("%d ships placed, want 1", len(ships))
	}
	if ships[0].Horizontal {
		t.Fatal("r did not turn the ship upright")
	}
	press(g, "r", "right", "space")
	ships = g.Player().Ships()
	if len(ships) != 2 {
		t.Fatalf("%d ships placed, want 2", len(ships))
	}
	if !ships[1].Horizontal {
		t.Fatal("r did not lay the ship down again")
	}
}

func TestPlacementConsumesShips(t *testing.T) {
	g := sized(Options{}, 80, 24)
	if g.Phase() != Placing {
		t.Fatalf("phase = %v, want placing", g.Phase())
	}
	pending := len(g.Player().Pending())
	for i := range len(Fleet) {
		moveTo(g, 0, i*2)
		press(g, "space")
		if got := len(g.Player().Pending()); got != pending-i-1 {
			t.Fatalf("after %d placements %d pending, want %d", i+1, got, pending-i-1)
		}
	}
	if g.Phase() != Firing {
		t.Fatalf("phase = %v after the whole fleet, want firing", g.Phase())
	}
	if x, y := g.Cursor(); x != Size/2 || y != Size/2 {
		t.Fatalf("cursor = %d,%d, want it on the enemy board", x, y)
	}
}

func TestInvalidPlacementKeepsPending(t *testing.T) {
	g := sized(Options{}, 80, 24)
	before := len(g.Player().Pending())
	moveTo(g, Size-2, 0) // a five-long ship from x=8 runs off the edge
	press(g, "space")
	if got := len(g.Player().Pending()); got != before {
		t.Fatalf("%d pending after an impossible placement, want %d", got, before)
	}
	if len(g.Player().Ships()) != 0 {
		t.Fatal("an impossible placement still put a ship down")
	}
	if !strings.Contains(ansi.Strip(g.View()), "doesn't fit") {
		t.Fatal("no note about the placement")
	}
}

func TestFiringMarksSquare(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	moveTo(g, 3, 4)
	press(g, "space")
	if !g.Enemy().Fired(3, 4) {
		t.Fatal("the enemy square was not fired at")
	}
	if g.Enemy().Shot(3, 4) == Unfired {
		t.Fatal("the enemy square still reads unfired")
	}
	if g.shots != 1 {
		t.Fatalf("%d shots counted, want 1", g.shots)
	}
}

func TestRepeatShotIgnored(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	moveTo(g, 3, 4)
	press(g, "space")
	g.Update(tickMsg{gen: g.gen}) // let the computer answer
	state, shots := g.State(), g.shots
	press(g, "space")
	if g.State() != state || g.shots != shots {
		t.Fatalf("a repeated shot moved the game on: state %v shots %d", g.State(), g.shots)
	}
	if g.pending {
		t.Fatal("a repeated shot asked the computer to answer")
	}
	if !strings.Contains(ansi.Strip(g.View()), "already") {
		t.Fatal("no note about the repeated shot")
	}
}

func TestAIRepliesOncePerShot(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	moveTo(g, 0, 0)
	press(g, "space")
	if !g.pending {
		t.Fatal("the shot did not wait for the computer")
	}
	if n := firedCount(g.Player()); n != 0 {
		t.Fatalf("the computer fired %d times before its timer", n)
	}
	moveTo(g, 5, 5)
	press(g, "space") // refused while the computer is answering
	if n := firedCount(g.Enemy()); n != 1 {
		t.Fatalf("%d player shots, want 1", n)
	}
	g.Update(tickMsg{gen: g.gen})
	if n := firedCount(g.Player()); n != 1 {
		t.Fatalf("the computer fired %d times for one shot, want 1", n)
	}
	g.Update(tickMsg{gen: g.gen}) // a clock tick, not a second reply
	if n := firedCount(g.Player()); n != 1 {
		t.Fatalf("the computer fired %d times for one shot, want 1", n)
	}
}

func TestNoReplyAfterGameOver(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	sink(g)
	if g.State() != Won || g.Phase() != Over {
		t.Fatalf("state %v phase %v after sinking the fleet, want won and over", g.State(), g.Phase())
	}
	before := firedCount(g.Player())
	g.Update(tickMsg{gen: g.gen})
	if n := firedCount(g.Player()); n != before {
		t.Fatalf("the computer fired %d times after the game ended", n-before)
	}
}

func TestWinBySinkingFleet(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	sink(g)
	if g.State() != Won {
		t.Fatalf("state = %v, want won", g.State())
	}
	if g.Phase() != Over {
		t.Fatalf("phase = %v, want over", g.Phase())
	}
	if !g.Enemy().AllSunk() {
		t.Fatal("the enemy fleet still floats")
	}
	if !strings.Contains(ansi.Strip(g.View()), "Fleet sunk") {
		t.Fatal("no winning message")
	}
}

func TestLoseWhenFleetSunk(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	// A fixed seed keeps the computer's hunt reproducible, so this test never
	// depends on how lucky its random shots are.
	g.rng = rand.New(rand.NewPCG(7, 11))
	g.newGame(g.opts)
	for i := 0; g.State() == Playing && i < Size*Size; i++ {
		x, y, ok := waterSquare(g.Enemy())
		if !ok {
			t.Fatal("no water left for the player to shoot at")
		}
		moveTo(g, x, y)
		press(g, "space") // the player never hits, so only the computer can win
		g.Update(tickMsg{gen: g.gen})
	}
	if g.State() != Lost {
		t.Fatalf("state = %v after the fleet went down, want lost", g.State())
	}
	if g.Phase() != Over {
		t.Fatalf("phase = %v, want over", g.Phase())
	}
	if !strings.Contains(ansi.Strip(g.View()), "bottom") {
		t.Fatal("no losing message")
	}
}

func TestNewGameResets(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	oldEnemy, oldPlayer := g.Enemy(), g.Player()
	moveTo(g, 0, 0)
	press(g, "space")
	gen := g.gen
	press(g, "n")
	if g.Enemy() == oldEnemy || g.Player() == oldPlayer {
		t.Fatal("n kept the old boards")
	}
	if g.Phase() != Firing {
		t.Fatalf("phase = %v, want firing in random mode", g.Phase())
	}
	if n := firedCount(g.Enemy()); n != 0 {
		t.Fatalf("%d enemy squares fired at after a new game", n)
	}
	if g.shots != 0 || g.State() != Playing {
		t.Fatalf("shots %d state %v after a new game", g.shots, g.State())
	}
	g.Update(tickMsg{gen: gen}) // the old game's reply must not land
	if n := firedCount(g.Player()); n != 0 {
		t.Fatalf("the previous game's reply fired %d times", n)
	}
}

func TestHandPlacedGameStartsAtFiring(t *testing.T) {
	g := sized(Options{}, 80, 24)
	for i := range len(Fleet) {
		moveTo(g, 0, i*2)
		press(g, "enter")
	}
	if g.Phase() != Firing {
		t.Fatalf("phase = %v after placing by hand, want firing", g.Phase())
	}
	press(g, "space")
	if !g.Enemy().Fired(Size/2, Size/2) {
		t.Fatal("enter and space do not both fire")
	}
}

func TestRandomFleetKey(t *testing.T) {
	g := sized(Options{}, 80, 24)
	press(g, "f")
	if g.Phase() != Firing {
		t.Fatalf("phase = %v after f, want firing", g.Phase())
	}
	if len(g.Player().Ships()) != len(Fleet) {
		t.Fatalf("%d ships after f, want %d", len(g.Player().Ships()), len(Fleet))
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "esc"} {
		g := sized(Options{}, 80, 24)
		press(g, k)
		if !g.IsQuit() {
			t.Fatalf("%s did not quit", k)
		}
	}
	g := sized(Options{}, 80, 24)
	g.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !g.IsQuit() {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestClickActsOnBoards(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	l := g.layout()
	ox, oy := l.origin(true)
	click(g, ox+1+2*l.cw+1, oy+1+3, tea.MouseLeft)
	if !g.Enemy().Fired(2, 3) {
		t.Fatalf("click at %d,%d did not fire at 2,3", ox+1+2*l.cw+1, oy+1+3)
	}
	if x, y := g.Cursor(); x != 2 || y != 3 {
		t.Fatalf("cursor = %d,%d, want 2,3", x, y)
	}

	placing := sized(Options{}, 80, 24)
	l = placing.layout()
	ox, oy = l.origin(false)
	click(placing, ox+1+1*l.cw+1, oy+1+1, tea.MouseLeft)
	if placing.Player().ShipAt(1, 1) == nil {
		t.Fatal("a click on the player's board did not place the ship")
	}
	if x, y := placing.Cursor(); x != 1 || y != 1 {
		t.Fatalf("cursor = %d,%d, want 1,1", x, y)
	}
}

func TestClickOnBorderIgnored(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	l := g.layout()
	ox, oy := l.origin(true)
	click(g, ox, oy+1, tea.MouseLeft)
	if firedCount(g.Enemy()) != 0 {
		t.Fatal("a click on the border fired a shot")
	}
}

func TestViewSizes(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {40, 12}, {10, 5}} {
		g := sized(Options{Random: true}, size.w, size.h)
		if v := g.View(); v == "" {
			t.Fatalf("%dx%d: empty view", size.w, size.h)
		}
	}
	g := sized(Options{Random: true}, 10, 5)
	if plain := ansi.Strip(g.View()); !strings.Contains(plain, "too small") {
		t.Fatalf("10x5 view has no too-small message: %q", plain)
	}
	if target, _, _ := g.layout().hit(5, 5); target != hitNone {
		t.Fatal("hit test active while the boards are hidden")
	}
}

func TestViewShowsBothBoards(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	plain := ansi.Strip(g.View())
	for _, want := range []string{title, "Your fleet", "Your shots", "afloat"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("view has no %q:\n%s", want, plain)
		}
	}
	if !g.layout().fits || g.layout().stacked {
		t.Fatal("80x24 did not lay the boards out side by side")
	}
}

func TestViewFillsScreen(t *testing.T) {
	g := sized(Options{Random: true}, 80, 24)
	lines := strings.Split(g.View(), "\n")
	if len(lines) != 24 {
		t.Fatalf("view has %d lines, want 24", len(lines))
	}
	for i, line := range lines {
		if w := len([]rune(ansi.Strip(line))); w > 80 {
			t.Fatalf("line %d is %d wide", i, w)
		}
	}
}

func TestNarrowTerminalStacksBoards(t *testing.T) {
	g := sized(Options{Random: true}, 40, 30)
	l := g.layout()
	if !l.fits || !l.stacked {
		t.Fatalf("40x30: fits=%v stacked=%v", l.fits, l.stacked)
	}
	plain := ansi.Strip(g.View())
	if !strings.Contains(plain, "Your fleet") || !strings.Contains(plain, "Your shots") {
		t.Fatal("stacked view lost a board title")
	}
}
