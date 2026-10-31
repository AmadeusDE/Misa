package chess

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func pickerType(g *Game, s string) {
	for _, r := range s {
		g.key(string(r))
	}
}

func TestVariantPickerListingAndFilter(t *testing.T) {
	g := newGame(t, "chess", 100, 30)
	custom := *g.match.Variant()
	custom.ID, custom.Name = "custom-id", "Unique Example"
	g.variants = append(g.variants, &custom)
	keys(g, "V")
	out := ansi.Strip(g.View())
	for _, v := range g.variants {
		if !strings.Contains(out, v.Name) || !strings.Contains(out, v.ID) {
			t.Errorf("picker missing name or id of %q (%q)", v.ID, v.Name)
		}
	}
	pickerType(g, "AMA")
	keys(g, "enter")
	if got := g.match.Variant().ID; got != "amazon" {
		t.Fatalf("AMA selected %q", got)
	}
	keys(g, "v")
	pickerType(g, "capa")
	keys(g, "enter")
	if got := g.match.Variant().ID; got != "capablanca" {
		t.Fatalf("capa selected %q", got)
	}
	keys(g, "v")
	pickerType(g, "UNIQUE")
	keys(g, "enter")
	if got := g.match.Variant().ID; got != "custom-id" {
		t.Fatalf("name filter selected %q", got)
	}
}

func TestVariantPickerPreservesAndResets(t *testing.T) {
	for _, open := range []string{"v", "V"} {
		t.Run(open, func(t *testing.T) {
			g := newGame(t, "chess", 80, 24)
			clickSquare(t, g, "e2", tea.MouseLeft)
			clickSquare(t, g, "e4", tea.MouseLeft)
			before := g.match
			keys(g, open, "enter")
			if g.match != before || len(g.match.Notation()) != 1 {
				t.Fatal("selecting current variant reset the game")
			}
			keys(g, open, "esc")
			if g.IsQuit() || g.match != before || len(g.match.Notation()) != 1 {
				t.Fatal("escape changed the game")
			}
			keys(g, open)
			pickerType(g, "amazon")
			keys(g, "enter")
			if g.match == before || g.match.Variant().ID != "amazon" || len(g.match.Notation()) != 0 {
				t.Fatal("switch did not start a fresh game")
			}
			keys(g, "u")
			if len(g.match.Notation()) != 0 {
				t.Fatal("undo crossed variant switch")
			}
		})
	}
}

func TestVariantPickerNavigation(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	a, _ := Find(vs, "amazon")
	b, _ := Find(vs, "chess")
	c, _ := Find(vs, "capablanca")
	for _, tc := range []struct{ key, want string }{{"up", "amazon"}, {"k", "amazon"}, {"ctrl+p", "amazon"}, {"down", "capablanca"}, {"j", "capablanca"}, {"ctrl+n", "capablanca"}} {
		t.Run(tc.key, func(t *testing.T) {
			g := New([]*Variant{a, b, c}, b)
			g.SetComputer(false, false)
			g.Resize(80, 24)
			keys(g, "v", tc.key, tc.key, tc.key, "enter")
			if got := g.match.Variant().ID; got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestVariantPickerInputIsolation(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	old, x, y, flip, letters := g.match, g.cx, g.cy, g.flipped, g.letters
	keys(g, "v")
	pickerType(g, "qjkufnt")
	if out := ansi.Strip(g.View()); !strings.Contains(out, "No matching variants") {
		t.Fatal("no-results message missing")
	}
	keys(g, "enter") // no matches: stays open, does not crash or quit
	if out := ansi.Strip(g.View()); !strings.Contains(out, "No matching variants") {
		t.Fatal("enter closed the no-results picker")
	}
	if g.IsQuit() || g.match != old || len(g.match.Notation()) != 1 || g.cx != x || g.cy != y || g.flipped != flip || g.letters != letters {
		t.Fatal("picker input leaked to board")
	}
	keys(g, "esc")
	if g.IsQuit() || g.match != old {
		t.Fatal("escape changed game")
	}
	keys(g, "v", "ctrl+c")
	if !g.IsQuit() {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestVariantPickerBackspace(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	keys(g, "v")
	pickerType(g, "amaé")
	keys(g, "backspace", "enter")
	if got := g.match.Variant().ID; got != "amazon" {
		t.Fatalf("rune backspace selected %q", got)
	}
}

func TestVariantPickerSmallView(t *testing.T) {
	g := newGame(t, "chess", 30, 8)
	keys(g, "v")
	for range 20 {
		keys(g, "down")
	}
	out := g.View()
	lines := strings.Split(out, "\n")
	if len(lines) > 8 {
		t.Fatalf("picker has %d lines, want at most 8", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > 30 {
			t.Errorf("line %d width %d", i, w)
		}
	}
	keys(g, "enter")
	if g.match.Variant().ID != g.variants[len(g.variants)-1].ID {
		t.Fatal("scrolled selection not selected")
	}
}

func TestVariantPickerSearchCompletionAndCancellation(t *testing.T) {
	g := vsComputer(t, "chess")
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	cmd := g.think()
	if cmd == nil {
		t.Fatal("search did not start")
	}
	keys(g, "v")
	msg := cmd()
	g.Update(msg)
	if len(g.match.Notation()) != 2 {
		t.Fatal("search result did not apply behind picker")
	}
	keys(g, "esc")
	if len(g.match.Notation()) != 2 {
		t.Fatal("closing picker lost search result")
	}
	// Complete another search, but switch variants before its result is delivered.
	keys(g, "u")
	clickSquare(t, g, "e2", tea.MouseLeft)
	clickSquare(t, g, "e4", tea.MouseLeft)
	cmd = g.think()
	if cmd == nil {
		t.Fatal("second search did not start")
	}
	msg = cmd()
	keys(g, "v")
	pickerType(g, "amazon")
	keys(g, "enter")
	g.Update(msg)
	if g.match.Variant().ID != "amazon" || len(g.match.Notation()) != 0 {
		t.Fatal("stale search changed new game")
	}
}

func TestVariantPickerJKAreFilterText(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	custom := *g.match.Variant()
	custom.ID, custom.Name = "ajk", "Special"
	g.variants = append(g.variants, &custom)
	keys(g, "v")
	pickerType(g, "ajk")
	keys(g, "enter")
	if got := g.match.Variant().ID; got != "ajk" {
		t.Fatalf("nonempty filter j/k selected %q", got)
	}
}

func TestVariantPickerMouseSelection(t *testing.T) {
	g := newGame(t, "chess", 100, 30)
	keys(g, "v")
	for y, line := range strings.Split(ansi.Strip(g.View()), "\n") {
		if i := strings.Index(line, "capablanca"); i >= 0 {
			x := ansi.StringWidth(line[:i])
			g.click(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
			if got := g.match.Variant().ID; got != "capablanca" {
				t.Fatalf("clicked capablanca, selected %q", got)
			}
			return
		}
	}
	t.Fatal("capablanca not visible")
}

func TestVariantPickerTextEventsAndSpace(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	custom := *g.match.Variant()
	custom.ID, custom.Name = "custom", "雪 Dragon"
	g.variants = append(g.variants, &custom)
	keys(g, "v")
	g.Update(tea.KeyPressMsg{Code: '雪', Text: "雪"})
	keys(g, "space")
	g.Update(tea.KeyPressMsg{Code: 'D', Text: "Dragon"})
	if out := ansi.Strip(g.View()); !strings.Contains(out, "雪 Dragon") {
		t.Fatal("Unicode filter or matching name not displayed")
	}
	keys(g, "enter")
	if g.match.Variant().ID != "custom" {
		t.Fatal("space or multi-rune text did not filter by name")
	}
}

func TestVariantPickerTinyViews(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {8, 4}, {30, 8}} {
		g := newGame(t, "chess", size[0], size[1])
		custom := *g.match.Variant()
		custom.ID, custom.Name = "unicode", "雪龍"
		g.variants = append(g.variants, &custom)
		keys(g, "v")
		for range 20 {
			keys(g, "down")
		}
		for y, line := range strings.Split(g.View(), "\n") {
			if y >= size[1] || ansi.StringWidth(line) > size[0] {
				t.Errorf("%dx%d view overflows at line %d: width %d", size[0], size[1], y, ansi.StringWidth(line))
			}
		}
	}
}
