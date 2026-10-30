package mines

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// setupState is the custom screen's editable state; the mine count is derived
// from the percent, so it is checked along with the three editable fields.
type setupState struct {
	w, h, pct, mines int
}

func (s setupState) String() string {
	return fmt.Sprintf("%d×%d at %d%% = %d mines", s.w, s.h, s.pct, s.mines)
}

// openSetup starts a game on level and opens the custom screen with c.
func openSetup(t *testing.T, level Level, width, height int) *Game {
	t.Helper()
	g := sized(level, width, height)
	press(g, "c")
	if g.setup == nil {
		t.Fatalf("c did not open the custom screen on %+v", level)
	}
	return g
}

func stateOf(g *Game) setupState {
	s := g.setup
	return setupState{s.level.W, s.level.H, s.percent, s.level.Mines}
}

// setupRow finds the rendered row of an editable field.
func setupRow(t *testing.T, g *Game, field int) (customLine, int) {
	t.Helper()
	lines, top := g.customLines()
	for i, line := range lines {
		if line.field == field {
			return line, top + i
		}
	}
	t.Fatalf("no row for field %d", field)
	return customLine{}, 0
}

// clickSetup clicks the ±delta button on the field's row exactly where the
// view draws it.
func clickSetup(t *testing.T, g *Game, field, delta int) {
	t.Helper()
	row, y := setupRow(t, g, field)
	for _, b := range row.buttons {
		if b.delta == delta {
			click(g, b.x, y, tea.MouseLeft)
			return
		}
	}
	t.Fatalf("field %d has no %+d button", field, delta)
}

// selectField moves the custom screen's cursor to a field with j.
func selectField(g *Game, field int) {
	for range field {
		press(g, "j")
	}
}

// stepOf is the state the screen should hold after one ±delta step on a field.
func stepOf(base setupState, field, delta int) setupState {
	out := base
	switch field {
	case 0:
		out.w = max(MinWidth, min(MaxWidth, base.w+delta))
	case 1:
		out.h = max(MinHeight, min(MaxHeight, base.h+delta))
	default:
		out.pct = max(MinPercent, min(MaxPercent, base.pct+delta))
	}
	out.mines = PercentMines(out.w, out.h, out.pct)
	return out
}

// setupFieldsByName pairs the field rows with their names for table tests.
var setupFieldsByName = []struct {
	name  string
	field int
}{
	{"width", 0},
	{"height", 1},
	{"percent", 2},
}

// setupSteps is every key alias for one step size. A terminal reports shift+[
// either as "{" or, without its text, as "shift+[".
var setupSteps = []struct {
	key   string
	delta int
}{
	{"left", -1}, {"h", -1},
	{"right", 1}, {"l", 1},
	{"[", -10}, {"shift+left", -10},
	{"]", 10}, {"shift+right", 10},
	{"{", -100}, {"shift+{", -100}, {"shift+[", -100}, {"pgdown", -100},
	{"}", 100}, {"shift+}", 100}, {"shift+]", 100}, {"pgup", 100},
}

func TestCustomCancelPreservesAndPauses(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	now := time.Unix(1000, 0)
	g.now = func() time.Time { return now }
	// Start a clock without relying on a random first move remaining in play.
	g.started = now
	g.board.placed = true
	g.board.ToggleFlag(0, 0)
	board, cx, cy := g.board, g.cx, g.cy
	now = now.Add(5 * time.Second)
	oldGen := g.gen
	press(g, "c", "l", "j", "l", "q", "f", "n", "space")
	click(g, g.layout().faceX(), g.layout().top, tea.MouseLeft)
	now = now.Add(10 * time.Second)
	if g.Elapsed() != 5*time.Second || g.board != board || !board.Flagged(0, 0) || g.IsQuit() || g.cx != cx || g.cy != cy {
		t.Fatal("panel changed game or failed to pause")
	}
	if g.Update(tickMsg{oldGen}) != nil {
		t.Fatal("old tick active")
	}
	cmd := g.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil || g.setup != nil {
		t.Fatal("cancel did not resume ticking")
	}
	now = now.Add(2 * time.Second)
	if g.Elapsed() != 7*time.Second {
		t.Fatalf("elapsed %v", g.Elapsed())
	}
	if g.Update(tickMsg{oldGen}) != nil {
		t.Fatal("stale tick resumed")
	}
}

func TestCustomNewGameAndClamping(t *testing.T) {
	g := openSetup(t, Levels[2], 80, 24)
	old := g.board
	// Expert's 99 mines on 30×16 are a 21% density, which rounds back to 101.
	if got, want := stateOf(g), (setupState{30, 16, 21, 101}); got != want {
		t.Fatalf("expert setup = %v, want %v", got, want)
	}
	// Stepping the width keeps the percent and recomputes the count.
	press(g, "{")
	if got, want := stateOf(g), (setupState{MinWidth, 16, 21, 7}); got != want {
		t.Fatalf("setup after shrinking width = %v, want %v", got, want)
	}
	press(g, "}")
	if got, want := stateOf(g), (setupState{102, 16, 21, PercentMines(102, 16, 21)}); got != want {
		t.Fatalf("setup after growing width = %v, want %v", got, want)
	}
	g.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if g.setup != nil || g.board == old || g.level != (Level{"Custom", 102, 16, 343}) || g.board.Started() {
		t.Fatalf("custom game not started: %+v", g.level)
	}
	press(g, "c", "c")
	if g.setup != nil {
		t.Fatal("c did not cancel")
	}
}

func TestCustomViewFitAndBounds(t *testing.T) {
	g := sized(Levels[2], 80, 24)
	press(g, "c")
	view := ansi.Strip(g.View())
	if !strings.Contains(view, "Custom game") || !strings.Contains(view, "Board fits") || strings.Contains(view, ":)") {
		t.Fatalf("panel: %s", view)
	}
	for _, line := range strings.Split(g.View(), "\n") {
		if lipgloss.Width(line) > 80 {
			t.Fatal("line too wide")
		}
	}
	if len(strings.Split(view, "\n")) != 24 {
		t.Fatal("wrong height")
	}
	g.Resize(60, 20)
	if !strings.Contains(ansi.Strip(g.View()), "won't fit") || g.layout().fits {
		t.Fatal("fit warning disagrees with board layout")
	}
	g.Resize(5, 3)
	lines := strings.Split(g.View(), "\n")
	if len(lines) != 3 {
		t.Fatal("small panel too tall")
	}
	for _, line := range lines {
		if lipgloss.Width(line) > 5 {
			t.Fatal("small panel too wide")
		}
	}
}

func TestCustomShrinkingClampsMines(t *testing.T) {
	// A dense 3×3 board with 8 mines is an 89% density.
	g := openSetup(t, Level{"Dense", 3, 3, 8}, 80, 24)
	if got, want := stateOf(g), (setupState{3, 3, 89, 8}); got != want {
		t.Fatalf("dense setup = %v, want %v", got, want)
	}
	press(g, "h")
	if got, want := stateOf(g), (setupState{2, 3, 89, 5}); got != want {
		t.Fatalf("mines after shrinking width = %v, want %v", got, want)
	}
	press(g, "j", "h")
	if got, want := stateOf(g), (setupState{2, 2, 89, 3}); got != want {
		t.Fatalf("mines after shrinking height = %v, want %v", got, want)
	}
}

func TestCustomCancelFinishedClock(t *testing.T) {
	g := sized(Levels[0], 80, 24)
	now := time.Unix(1000, 0)
	g.now = func() time.Time { return now }
	g.started = now
	g.ended = now.Add(4 * time.Second)
	now = now.Add(10 * time.Second)
	press(g, "c")
	now = now.Add(time.Minute)
	if g.Elapsed() != 4*time.Second {
		t.Fatal("finished clock changed")
	}
	if cmd := g.Update(tea.KeyPressMsg{Code: 'c', Text: "c"}); cmd != nil {
		t.Fatal("finished game resumed ticking")
	}
	if g.Elapsed() != 4*time.Second {
		t.Fatal("finished clock changed after cancel")
	}
}

// TestCustomStepsByKeyAndMouse walks every step alias on every field and checks
// that the key, the matching on-screen button and the expected state agree.
func TestCustomStepsByKeyAndMouse(t *testing.T) {
	// 50×50 with 100 mines opens at 4% (100 mines), low enough that neither
	// direction clamps.
	base := Level{"test", 50, 50, 100}
	for _, f := range setupFieldsByName {
		for _, step := range setupSteps {
			t.Run(f.name+"/"+step.key, func(t *testing.T) {
				key := openSetup(t, base, 80, 24)
				start := stateOf(key)
				if start != (setupState{50, 50, 4, 100}) {
					t.Fatalf("base setup = %v", start)
				}
				selectField(key, f.field)
				press(key, step.key)
				want := stepOf(start, f.field, step.delta)

				mouse := openSetup(t, base, 80, 24)
				clickSetup(t, mouse, f.field, step.delta)

				if got := stateOf(key); got != want {
					t.Fatalf("key %q on %s = %v, want %v", step.key, f.name, got, want)
				}
				if got := stateOf(mouse); got != want {
					t.Fatalf("%+d click on %s = %v, want %v", step.delta, f.name, got, want)
				}
			})
		}
	}
}

// TestCustomClampsAtLimits runs each field past both ends with keys and with
// clicks.
func TestCustomClampsAtLimits(t *testing.T) {
	base := setupState{30, 16, 21, 101}
	for _, tt := range []struct {
		name    string
		field   int
		key     string
		delta   int
		clamped int
	}{
		{"width low", 0, "{", -100, MinWidth},
		{"width high", 0, "}", 100, MaxWidth},
		{"height low", 1, "{", -100, MinHeight},
		{"height high", 1, "}", 100, MaxHeight},
		{"percent low", 2, "{", -100, MinPercent},
		{"percent high", 2, "}", 100, MaxPercent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := base
			switch tt.field {
			case 0:
				want.w = tt.clamped
			case 1:
				want.h = tt.clamped
			default:
				want.pct = tt.clamped
			}
			want.mines = PercentMines(want.w, want.h, want.pct)

			const steps = 10 // enough for ±100 to hit the far end from 30×16
			key := openSetup(t, Levels[2], 80, 24)
			selectField(key, tt.field)
			for range steps {
				press(key, tt.key)
			}
			if got := stateOf(key); got != want {
				t.Fatalf("key %q ×%d on field %d = %v, want %v", tt.key, steps, tt.field, got, want)
			}

			mouse := openSetup(t, Levels[2], 80, 24)
			for range steps {
				clickSetup(t, mouse, tt.field, tt.delta)
			}
			if got := stateOf(mouse); got != want {
				t.Fatalf("%+d click ×%d on field %d = %v, want %v", tt.delta, steps, tt.field, got, want)
			}

			// Clamped values still leave a playable, rendered panel.
			for _, g := range []*Game{key, mouse} {
				view := ansi.Strip(g.View())
				if lines := strings.Split(view, "\n"); len(lines) != 24 {
					t.Fatalf("panel has %d lines, want 24", len(lines))
				}
				if g.setup.level.Mines < MinMines || g.setup.level.Mines > g.setup.level.W*g.setup.level.H-1 {
					t.Fatalf("clamped setup is unplayable: %v", stateOf(g))
				}
				for _, line := range strings.Split(g.View(), "\n") {
					if lipgloss.Width(line) > 80 {
						t.Fatal("line too wide")
					}
				}
			}
		})
	}
}

func TestCustomClicksOffButtonsIgnored(t *testing.T) {
	g := openSetup(t, Levels[2], 80, 24)
	start := stateOf(g)
	lines, top := g.customLines()
	for i, line := range lines {
		y := top + i
		if line.field < 0 {
			// Text lines carry no controls at all.
			click(g, line.x+1, y, tea.MouseLeft)
			continue
		}
		// The marker column and the gap around the value are not buttons.
		click(g, line.x, y, tea.MouseLeft)
		click(g, (line.buttons[2].x+line.buttons[2].w+line.buttons[3].x)/2, y, tea.MouseLeft)
	}
	if got := stateOf(g); got != start {
		t.Fatalf("clicks off the buttons changed the setup: %v, want %v", got, start)
	}
	// A right click on a [+1] button is not a left click.
	row, y := setupRow(t, g, 0)
	click(g, row.buttons[3].x, y, tea.MouseRight)
	click(g, row.buttons[3].x, y, tea.MouseMiddle)
	// Clicks past the panel and past the terminal are ignored too.
	click(g, 10, top+len(lines)+1, tea.MouseLeft)
	click(g, 10, 23, tea.MouseLeft)
	if got := stateOf(g); got != start {
		t.Fatalf("non-left clicks changed the setup: %v, want %v", got, start)
	}
	// A click lands on the row it is drawn on, not on the selected field.
	row, y = setupRow(t, g, 1)
	click(g, row.buttons[4].x, y, tea.MouseLeft) // the height row's [+10]
	if got, want := stateOf(g), stepOf(start, 1, 10); got != want {
		t.Fatalf("height row click = %v, want %v", got, want)
	}
}

func TestCustomRendersButtonsAndMineCount(t *testing.T) {
	g := openSetup(t, Levels[2], 80, 24)
	plain := ansi.Strip(g.View())
	lines := strings.Split(plain, "\n")
	if len(lines) != 24 {
		t.Fatalf("panel has %d lines, want 24", len(lines))
	}
	note := fmt.Sprintf("(= %d mines)", PercentMines(g.setup.level.W, g.setup.level.H, g.setup.percent))
	if !strings.Contains(plain, "Mines %") || !strings.Contains(plain, note) {
		t.Fatalf("mine percent row missing %q: %s", note, plain)
	}
	if g.setup.level.Mines != PercentMines(g.setup.level.W, g.setup.level.H, g.setup.percent) {
		t.Fatalf("%v does not match the percent", stateOf(g))
	}
	rows, top := g.customLines()
	if len(rows)+top > 24 {
		t.Fatalf("%d rows from %d do not fit the panel", len(rows), top)
	}
	for i, line := range rows {
		// customView draws the indent, then the truncated text: the mouse's
		// columns must be the view's columns.
		if got, want := lines[top+i], strings.Repeat(" ", line.x)+truncate(line.text, max(0, g.width-line.x)); got != want {
			t.Fatalf("row %d renders %q, want %q", i, got, want)
		}
		if line.field < 0 {
			if len(line.buttons) != 0 {
				t.Fatalf("text row %d carries buttons", i)
			}
			continue
		}
		// Every field row offers the same six steps, drawn where the mouse
		// looks for them.
		if len(line.buttons) != len(setupDeltas) {
			t.Fatalf("field %d has %d buttons, want %d", line.field, len(line.buttons), len(setupDeltas))
		}
		r := []rune(lines[top+i])
		for j, b := range line.buttons {
			if b.delta != setupDeltas[j] {
				t.Fatalf("field %d button %d steps %+d, want %+d", line.field, j, b.delta, setupDeltas[j])
			}
			if b.x+b.w > len(r) {
				t.Fatalf("button %+d runs past the line", b.delta)
			}
			if got, want := string(r[b.x:b.x+b.w]), setupLabel(b.delta); got != want {
				t.Fatalf("button at %d,%d reads %q, want %q", b.x, top+i, got, want)
			}
		}
	}
	for _, label := range []string{"[-100]", "[-10]", "[-1]", "[+1]", "[+10]", "[+100]"} {
		if !strings.Contains(plain, label) {
			t.Fatalf("label %q missing: %s", label, plain)
		}
	}
	for _, f := range setupFieldsByName {
		row, _ := setupRow(t, g, f.field)
		if strings.TrimSpace(row.text) == "" {
			t.Fatalf("%s row is empty", f.name)
		}
		if !strings.Contains(row.text, setupFields[f.field]) {
			t.Fatalf("%s row %q does not name the field", f.name, row.text)
		}
	}
}
