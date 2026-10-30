package chess

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func newGame(t *testing.T, id string, w, h int) *Game {
	t.Helper()
	vs, _ := Builtins()
	v, err := Find(vs, id)
	if err != nil {
		t.Fatal(err)
	}
	g := New(vs, v)
	g.SetComputer(false, false)
	g.Resize(w, h)
	return g
}

func vsComputer(t *testing.T, id string) *Game {
	t.Helper()
	g := newGame(t, id, 100, 30)
	g.SetComputer(false, true)
	g.SetThinkTime(100 * time.Millisecond)
	return g
}

func press(g *Game, k string) tea.Cmd {
	return g.Update(tea.KeyPressMsg{Code: rune(k[0]), Text: k})
}

// runThought carries out a search command the way Bubble Tea would.
func runThought(t *testing.T, g *Game, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("no search started")
	}
	if next := g.Update(cmd()); next != nil {
		t.Fatal("started another search after replying")
	}
}

func keys(g *Game, ks ...string) {
	for _, k := range ks {
		g.key(k)
	}
}

func clickSquare(t *testing.T, g *Game, name string, button tea.MouseButton) {
	t.Helper()
	sq, err := g.match.Variant().ParseSquare(name)
	if err != nil {
		t.Fatal(err)
	}
	l := g.layout()
	for y := range g.height {
		for x := range g.width {
			if got, ok := l.square(g, x, y); ok && got == sq {
				g.click(tea.Mouse{X: x, Y: y, Button: button})
				return
			}
		}
	}
	t.Fatalf("square %s not on screen", name)
}

func TestKeyboardMove(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	// The cursor starts on d2; step right to e2, pick up the pawn, go up two.
	keys(g, "l", "space", "k", "k", "enter")
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e2-e4" {
		t.Fatalf("notation = %v", got)
	}
	if g.match.Turn() != Black {
		t.Error("turn did not pass to Black")
	}
}

func TestMouseMoveAndDeselect(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	clickSquare(t, g, "g1", tea.MouseLeft)
	if g.sel < 0 {
		t.Fatal("knight not selected")
	}
	clickSquare(t, g, "g1", tea.MouseRight)
	if g.sel >= 0 {
		t.Fatal("right click did not deselect")
	}
	clickSquare(t, g, "g1", tea.MouseLeft)
	clickSquare(t, g, "f3", tea.MouseLeft)
	if got := g.match.Notation(); len(got) != 1 || got[0] != "Ng1-f3" {
		t.Fatalf("notation = %v", got)
	}
}

func TestFlippedBoardClicks(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	keys(g, "f")
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e2-e4" {
		t.Fatalf("notation = %v", got)
	}
}

func TestPromotionPrompt(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	v := g.match.Variant()
	p, _ := ParseFEN(v, "k7/4P3/8/8/8/8/8/K7 w -")
	g.Load(p)
	clickSquare(t, g, "e7", tea.MouseLeft)
	clickSquare(t, g, "e8", tea.MouseLeft)
	if len(g.promo) != 4 {
		t.Fatalf("%d promotion choices, want 4", len(g.promo))
	}
	if !strings.Contains(ansi.Strip(g.View()), "Promote to:") {
		t.Error("no promotion prompt on screen")
	}
	keys(g, "n")
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e7-e8=N" {
		t.Fatalf("notation = %v", got)
	}
}

func TestPromotionByClick(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	p, _ := ParseFEN(g.match.Variant(), "k7/4P3/8/8/8/8/8/K7 w -")
	g.Load(p)
	clickSquare(t, g, "e7", tea.MouseLeft)
	clickSquare(t, g, "e8", tea.MouseLeft)
	l := g.layout()
	for x := range g.width {
		if l.promoChoice(g, x, l.statusY()) == 1 {
			g.click(tea.Mouse{X: x, Y: l.statusY(), Button: tea.MouseLeft})
			break
		}
	}
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e7-e8=R+" {
		t.Fatalf("notation = %v", got)
	}
}

func TestViewFitsEachVariant(t *testing.T) {
	for _, id := range []string{"chess", "double", "bigboard"} {
		g := newGame(t, id, 80, 24)
		out := g.View()
		lines := strings.Split(out, "\n")
		if len(lines) != 24 {
			t.Errorf("%s: %d lines, want 24", id, len(lines))
		}
		for i, line := range lines {
			if w := ansi.StringWidth(line); w > 80 {
				t.Errorf("%s: line %d is %d wide", id, i, w)
			}
		}
		if strings.Contains(out, "too small") {
			t.Errorf("%s does not fit 80×24", id)
		}
	}
}

func TestTooSmall(t *testing.T) {
	g := newGame(t, "bigboard", 30, 10)
	if !strings.Contains(ansi.Strip(g.View()), "too small") {
		t.Error("no too-small message")
	}
}

func TestCycleVariantAndUndo(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	keys(g, "v")
	if g.match.Variant().ID != "double" {
		t.Fatalf("v went to %s", g.match.Variant().ID)
	}
	keys(g, "V")
	if g.match.Variant().ID != "chess" {
		t.Fatalf("V went to %s", g.match.Variant().ID)
	}
	keys(g, "V")
	if g.match.Variant().ID != "bughouse" {
		t.Fatalf("V wrapped to %s", g.match.Variant().ID)
	}
	keys(g, "V")
	if g.match.Variant().ID != "crazyhouse" {
		t.Fatalf("V went to %s", g.match.Variant().ID)
	}
	keys(g, "V")
	if g.match.Variant().ID != "doublebig" {
		t.Fatalf("V went to %s", g.match.Variant().ID)
	}
	keys(g, "V")
	if g.match.Variant().ID != "bigboard" {
		t.Fatalf("V went to %s", g.match.Variant().ID)
	}
	clickSquare(t, g, "f1", tea.MouseLeft)
	clickSquare(t, g, "d6", tea.MouseLeft)
	if len(g.match.Notation()) != 1 {
		t.Fatal("rose did not move")
	}
	keys(g, "u")
	if len(g.match.Notation()) != 0 {
		t.Error("undo kept the move")
	}
}

func TestLettersToggle(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	if !strings.Contains(g.View(), "♞") {
		t.Fatal("no knight glyph")
	}
	keys(g, "t")
	out := ansi.Strip(g.View())
	if strings.Contains(out, "♞") || !strings.Contains(out, "n") {
		t.Error("letters mode still shows glyphs")
	}
}

func TestComputerReplies(t *testing.T) {
	g := vsComputer(t, "chess")
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	cmd := g.think()
	if !g.thinking || !strings.Contains(ansi.Strip(g.View()), "thinking") {
		t.Fatal("not shown as thinking")
	}
	clickSquare(t, g, "e7", tea.MouseLeft)
	if g.sel >= 0 {
		t.Error("selected a black piece while the computer is to move")
	}
	runThought(t, g, cmd)
	if n := len(g.match.Notation()); n != 2 || g.match.Turn() != White {
		t.Fatalf("after the reply: %v", g.match.Notation())
	}
	if !strings.Contains(g.status(), "Computer played") {
		t.Errorf("status = %q", g.status())
	}
	press(g, "u")
	if len(g.match.Notation()) != 0 || g.match.Turn() != White {
		t.Errorf("undo left %v", g.match.Notation())
	}
}

func TestAbandonedSearchIsIgnored(t *testing.T) {
	g := vsComputer(t, "chess")
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	cmd := g.think()
	press(g, "u")
	if g.thinking {
		t.Fatal("still thinking after undo")
	}
	g.Update(cmd())
	if len(g.match.Notation()) != 0 {
		t.Errorf("stale search moved: %v", g.match.Notation())
	}
}

func TestComputerOpensAsWhite(t *testing.T) {
	g := vsComputer(t, "chess")
	g.SetComputer(true, false)
	if !g.flipped {
		t.Error("board not turned for the player of Black")
	}
	runThought(t, g, g.Init())
	if g.match.Turn() != Black || len(g.match.Notation()) != 1 {
		t.Errorf("White did not open: %v", g.match.Notation())
	}
	press(g, "u")
	if len(g.match.Notation()) != 1 {
		t.Error("undo took back the computer's opening move with nothing of yours to undo")
	}
}

func TestCycleComputer(t *testing.T) {
	g := vsComputer(t, "chess")
	want := [][2]bool{{true, false}, {true, true}, {false, false}, {false, true}}
	for _, w := range want {
		g.key("c")
		if g.computer != w {
			t.Fatalf("computer = %v, want %v", g.computer, w)
		}
	}
}

func TestComputerVsComputer(t *testing.T) {
	g := vsComputer(t, "chess")
	g.SetComputer(true, true)
	cmd := g.Init()
	for range 4 {
		if cmd == nil {
			t.Fatal("no search for the next side")
		}
		cmd = g.Update(cmd())
	}
	if n := len(g.match.Notation()); n != 4 {
		t.Errorf("%d moves played, want 4", n)
	}
}

func TestQuitStopsSearch(t *testing.T) {
	g := vsComputer(t, "chess")
	g.SetComputer(true, false)
	g.SetThinkTime(time.Minute)
	cmd := g.Init()
	stop := g.stop
	press(g, "q")
	if !stop.Load() {
		t.Fatal("quitting did not stop the search")
	}
	done := make(chan struct{})
	go func() { cmd(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("search kept running after quit")
	}
}

func TestRosePassByClickingItAgain(t *testing.T) {
	g := newGame(t, "bigboard", 100, 30)
	p, err := ParseFEN(g.match.Variant(), "15k/16/16/16/16/16/16/16/7O8/16/16/16/16/16/16/K15 w -")
	if err != nil {
		t.Fatal(err)
	}
	g.Load(p)
	clickSquare(t, g, "h8", tea.MouseLeft)
	if !strings.Contains(g.status(), "pass") {
		t.Errorf("no pass hint: %q", g.status())
	}
	clickSquare(t, g, "h8", tea.MouseLeft)
	if got := g.match.Notation(); len(got) != 1 || got[0] != "Oh8 pass" {
		t.Errorf("notation = %v", got)
	}
}

func TestLevelChangeDropsQueuedSearch(t *testing.T) {
	g := vsComputer(t, "chess")
	g.SetComputer(true, false)
	old := g.Init()
	stop, generation := g.stop, g.gen
	next := press(g, "d")
	if !stop.Load() || g.gen <= generation || g.Level() != Hard || next == nil {
		t.Fatal("difficulty change did not replace search")
	}
	// Run the newer command first: cancelled, queued work must not monopolize
	// the cached engine or apply its obsolete move when eventually scheduled.
	runThought(t, g, next)
	g.Update(old())
	if len(g.match.Played()) != 1 {
		t.Fatal("stale result changed the board")
	}
	if !strings.Contains(g.header(), "hard") {
		t.Fatal("level not visible")
	}
}

func TestLevelPreservedAndSettingsCaptured(t *testing.T) {
	g := newGame(t, "chess", 100, 30)
	g.SetLevel(Beginner)
	keys(g, "n", "v", "c", "c", "c", "c")
	if g.Level() != Beginner {
		t.Fatal("level lost across game, variant or hotseat changes")
	}
	g.SetComputer(true, false)
	cmd := g.Init()
	// Mutate without cancellation to detect accidental closure reads; the
	// public setters cancel instead, and are covered separately.
	g.level, g.thinkTime = Expert, time.Minute
	msg := cmd().(thoughtMsg)
	if msg.result.Depth > 1 || msg.result.Nodes > Beginner.Limits(0).Nodes {
		t.Fatalf("command read mutable settings: %+v", msg.result)
	}
	g.cancel()
}

func TestLevelChangeCancelsRunningSearch(t *testing.T) {
	g := vsComputer(t, "bigboard")
	g.SetComputer(true, false)
	g.SetThinkTime(time.Minute)
	cmd := g.Init()
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	g.SetLevel(Beginner)
	next := g.Init()
	if next == nil {
		t.Fatal("replacement search not scheduled")
	}
	replacement := make(chan tea.Msg, 1)
	go func() { replacement <- next() }()
	select {
	case msg := <-replacement:
		g.Update(msg)
	case <-time.After(3 * time.Second):
		t.Fatal("replacement blocked behind abandoned search")
	}
	select {
	case msg := <-done:
		g.Update(msg)
	case <-time.After(3 * time.Second):
		t.Fatal("abandoned goroutine did not exit")
	}
	if len(g.match.Played()) != 1 {
		t.Fatal("abandoned move applied")
	}
}
