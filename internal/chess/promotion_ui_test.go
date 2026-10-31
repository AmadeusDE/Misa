package chess

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// optionalVariant is a variant whose pawns promote inside a two-rank zone but
// may decline, as the YAML files spell it out.
func optionalVariant(t *testing.T) (*Variant, []*Variant) {
	t.Helper()
	vs, err := parseVariants(map[string][]byte{
		"optional.yaml": []byte("id: optional\nname: Optional\nextends: chess\npromotion_zone: 2\npromotion_optional: true\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "optional")
	if err != nil {
		t.Fatal(err)
	}
	return v, vs
}

// offerDecline adds the plain move that stays a pawn to the match's legal
// moves, the way a variant with optional promotion lists it, so the screen can
// be tested without depending on the move generator.
func offerDecline(t *testing.T, g *Game, from, to string) {
	t.Helper()
	v := g.match.Variant()
	f, err := v.ParseSquare(from)
	if err != nil {
		t.Fatal(err)
	}
	tq, err := v.ParseSquare(to)
	if err != nil {
		t.Fatal(err)
	}
	g.match.legal = append(g.match.legal, Move{From: f, To: tq, Take: -1, Castle: -1})
}

// playToE8 puts the game at the position of TestPromotionPrompt: a white pawn
// on e7 with only the black king in front of it. With decline, the plain move
// is offered alongside the promoted ones as a variant with optional promotion
// lists it. It opens the promotion prompt by clicking both squares.
func playToE8(t *testing.T, g *Game, decline bool) {
	t.Helper()
	p, err := ParseFEN(g.match.Variant(), "k7/4P3/8/8/8/8/8/K7 w -")
	if err != nil {
		t.Fatal(err)
	}
	g.Load(p)
	clickSquare(t, g, "e7", tea.MouseLeft)
	if decline {
		offerDecline(t, g, "e7", "e8")
	}
	clickSquare(t, g, "e8", tea.MouseLeft)
}

// TestMatchUnpromotedMoveToLastRank covers the move itself on Grand Chess, the
// variant that allows it: a pawn played to its eighth rank without promoting
// stays a pawn, and the match records it as an ordinary pawn move that can be
// taken back.
func TestMatchUnpromotedMoveToLastRank(t *testing.T) {
	v := builtin(t, "grand")
	m := NewMatch(v)
	m.Load(fen(t, v, "9k/10/10/4P5/10/10/10/10/10/4K5 w -"))
	declined := rulesDecline(t, m.Position(), "e7", "e8")
	m.Play(declined)
	if got := m.Notation(); len(got) != 1 || got[0] != "e7-e8" {
		t.Fatalf("notation = %v, want e7-e8", got)
	}
	to := rulesSquare(t, v, "e8")
	if pc := m.Position().Board[to]; pc.Type != v.TypeOf('P') || pc.Color != White {
		t.Fatalf("e8 holds %+v, want a white pawn", pc)
	}
	if m.Turn() != Black {
		t.Error("turn did not pass to Black")
	}
	if m.Position().Clock != 0 {
		t.Errorf("clock = %d, want a pawn move to reset it", m.Position().Clock)
	}
	if !m.Undo() {
		t.Fatal("cannot take the move back")
	}
	from := rulesSquare(t, v, "e7")
	if pc := m.Position().Board[from]; pc.Type != v.TypeOf('P') || pc.Color != White {
		t.Errorf("e7 holds %+v after undo, want the pawn back", pc)
	}
	if m.Position().Board[to].Type != 0 {
		t.Error("e8 not empty after undo")
	}
}

// TestDeclinePromotionWithKey picks the pawn from the prompt by pressing its
// own letter, the control the choice's glyph advertises.
func TestDeclinePromotionWithKey(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	playToE8(t, g, true)
	if len(g.promo) != 5 {
		t.Fatalf("%d promotion choices, want 5 including the pawn", len(g.promo))
	}
	if g.promo[0].Promote != 0 {
		t.Fatalf("first choice promotes to %d, want the plain move", g.promo[0].Promote)
	}
	labels := g.promoLabels(g.layout())
	if len(labels) != 5 || !strings.Contains(labels[0], "Pawn") || !strings.Contains(labels[1], "Queen") {
		t.Fatalf("labels = %q, want the pawn first", labels)
	}
	out := ansi.Strip(g.View())
	if !strings.Contains(out, "Promote to:") {
		t.Fatal("no promotion prompt on screen")
	}
	if pawn, queen := strings.Index(out, "Pawn"), strings.Index(out, "Queen"); pawn < 0 || queen < pawn {
		t.Error("the pawn is not the first choice on screen")
	}
	keys(g, "p")
	if g.promo != nil {
		t.Fatal("prompt still open")
	}
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e7-e8" {
		t.Fatalf("notation = %v, want e7-e8", got)
	}
	v := g.match.Variant()
	sq, err := v.ParseSquare("e8")
	if err != nil {
		t.Fatal(err)
	}
	if pc := g.match.Position().Board[sq]; pc.Type != v.TypeOf('P') || pc.Color != White {
		t.Fatalf("e8 holds %+v, want a white pawn", pc)
	}
	if !strings.Contains(g.status(), "e7-e8") {
		t.Errorf("status = %q, want the move played", g.status())
	}
	keys(g, "u")
	if n := len(g.match.Notation()); n != 0 {
		t.Errorf("undo kept %d moves", n)
	}
	from, err := v.ParseSquare("e7")
	if err != nil {
		t.Fatal(err)
	}
	if pc := g.match.Position().Board[from]; pc.Type != v.TypeOf('P') || pc.Color != White {
		t.Errorf("e7 holds %+v after undo, want the pawn back", pc)
	}
}

// TestDeclinePromotionByClick picks the pawn with the mouse.
func TestDeclinePromotionByClick(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	playToE8(t, g, true)
	l := g.layout()
	hit := -1
	for x := range g.width {
		if i := l.promoChoice(g, x, l.statusY()); i == 0 {
			hit = x
			break
		}
	}
	if hit < 0 {
		t.Fatal("the pawn choice is not clickable")
	}
	g.click(tea.Mouse{X: hit, Y: l.statusY(), Button: tea.MouseLeft})
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e7-e8" {
		t.Fatalf("notation = %v, want e7-e8", got)
	}
}

// TestKeepKeyIgnoredWithoutDecline leaves a plain pawn promotion alone: with no
// declining move on offer the pawn's letter picks nothing.
func TestKeepKeyIgnoredWithoutDecline(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	playToE8(t, g, false)
	if len(g.promo) != 4 {
		t.Fatalf("%d promotion choices, want 4", len(g.promo))
	}
	keys(g, "p")
	if g.promo == nil {
		t.Fatal("p closed the prompt")
	}
	if n := len(g.match.Notation()); n != 0 {
		t.Fatalf("p played %v", g.match.Notation())
	}
}

// TestInvalidPromotionKeyIgnored keeps an unbound key from playing anything.
func TestInvalidPromotionKeyIgnored(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	playToE8(t, g, true)
	keys(g, "z", "space", "enter")
	if g.promo == nil || len(g.match.Notation()) != 0 {
		t.Fatalf("a stray key answered the prompt: %v", g.match.Notation())
	}
}

// TestPromptFallsBackToGlyphsWhenNarrow shrinks the terminal until the names no
// longer fit: every choice stays reachable as its glyph and the status line
// never wraps.
func TestPromptFallsBackToGlyphsWhenNarrow(t *testing.T) {
	g := newGame(t, "chess", 80, 24)
	playToE8(t, g, true)
	g.Resize(56, 24)
	labels := g.promoLabels(g.layout())
	if len(labels) != len(g.promo) {
		t.Fatalf("%d of %d choices shown", len(labels), len(g.promo))
	}
	for i, label := range labels {
		if strings.Contains(label, "Pawn") || strings.Contains(label, "Queen") {
			t.Fatalf("choice %d keeps its name on a narrow screen: %q", i, label)
		}
	}
	for i, line := range strings.Split(g.View(), "\n") {
		if w := ansi.StringWidth(line); w > 56 {
			t.Errorf("line %d is %d wide, want at most 56", i, w)
		}
	}
}

// TestWidePromotionPromptFits offers more pieces than fit by name: the prompt
// falls back to glyphs so every choice stays clickable.
func TestWidePromotionPromptFits(t *testing.T) {
	vs, err := parseVariants(map[string][]byte{
		"widepromo.yaml": []byte(`id: widepromo
name: Wide Promotion
extends: chess
promotion: [Q, R, B, N, M, C]
pieces:
  - {letter: M, name: Marshal, betza: RN}
  - {letter: C, name: Cardinal, betza: BN}
`),
	})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "widepromo")
	if err != nil {
		t.Fatal(err)
	}
	g := New(vs, v)
	g.SetComputer(false, false)
	g.Resize(80, 24)
	playToE8(t, g, false)
	if len(g.promo) != 6 {
		t.Fatalf("%d promotion choices, want 6", len(g.promo))
	}
	labels := g.promoLabels(g.layout())
	if len(labels) != 6 {
		t.Fatalf("%d of 6 choices shown", len(labels))
	}
	if strings.Contains(labels[0], "Queen") || strings.Contains(labels[5], "Cardinal") {
		t.Fatalf("labels = %q, want glyphs only", labels)
	}
	for i, line := range strings.Split(g.View(), "\n") {
		if w := ansi.StringWidth(line); w > 80 {
			t.Errorf("line %d is %d wide, want at most 80", i, w)
		}
	}
	l := g.layout()
	last := -1
	for x := range g.width {
		if i := l.promoChoice(g, x, l.statusY()); i == 5 {
			last = x
			break
		}
	}
	if last < 0 {
		t.Fatal("the last choice is not clickable")
	}
	g.click(tea.Mouse{X: last, Y: l.statusY(), Button: tea.MouseLeft})
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e7-e8=C" {
		t.Fatalf("notation = %v, want e7-e8=C", got)
	}
}

// TestOptionalPromotionFromVariantFile is the end to end case: the YAML rule
// promotion_optional makes the generator list the plain move, and the screen
// offers it first and plays it.
func TestOptionalPromotionFromVariantFile(t *testing.T) {
	v, vs := optionalVariant(t)
	g := New(vs, v)
	g.SetComputer(false, false)
	g.Resize(80, 24)
	p, err := ParseFEN(v, "k7/8/4P3/8/8/8/8/K7 w -")
	if err != nil {
		t.Fatal(err)
	}
	g.Load(p)
	clickSquare(t, g, "e6", tea.MouseLeft)
	clickSquare(t, g, "e7", tea.MouseLeft)
	if len(g.promo) != 5 {
		t.Fatalf("%d choices %+v, want the pawn and four pieces", len(g.promo), g.promo)
	}
	if g.promo[0].Promote != 0 {
		t.Fatalf("first choice promotes to %d, want the plain move", g.promo[0].Promote)
	}
	keys(g, "p")
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e6-e7" {
		t.Fatalf("notation = %v, want e6-e7", got)
	}
}

// TestPromotionPromptKeepsThePickerClosed covers the two overlays the screen
// owns: while a promotion waits for an answer its keys belong to the prompt, so
// v/V must not open the variant picker behind it, and once the answer is in the
// picker opens again.
func TestPromotionPromptKeepsThePickerClosed(t *testing.T) {
	g := newGame(t, "chess", 100, 30)
	playToE8(t, g, true)
	keys(g, "v", "V")
	if g.picker != nil {
		t.Fatal("v/V opened the picker over the promotion prompt")
	}
	if g.promo == nil {
		t.Fatal("v/V closed the promotion prompt")
	}
	if out := ansi.Strip(g.View()); strings.Contains(out, "Chess variants") || strings.Contains(out, "Search:") {
		t.Error("the picker is drawn under the prompt")
	}
	// A click while the prompt is open is a promotion choice, never a variant.
	l := g.layout()
	if i := l.promoChoice(g, 0, l.statusY()); i >= 0 {
		t.Fatal("a click outside the prompt hit a choice")
	}
	keys(g, "q")
	if g.promo != nil {
		t.Fatal("q did not answer the prompt")
	}
	if n := len(g.match.Notation()); n != 1 {
		t.Fatalf("notation = %v, want one move", g.match.Notation())
	}
	// The picker works as usual afterwards, and opening it hides the board.
	keys(g, "v")
	if g.picker == nil {
		t.Fatal("v did not open the picker once the prompt was answered")
	}
	if g.promo != nil {
		t.Fatal("the picker opened with a promotion pending")
	}
	if out := ansi.Strip(g.View()); !strings.Contains(out, "Chess variants") {
		t.Error("the picker is not on screen")
	}
	keys(g, "g", "r", "a", "n", "d", "enter")
	if got := g.match.Variant().ID; got != "grand" {
		t.Fatalf("picker selected %s, want grand", got)
	}
	if g.promo != nil {
		t.Fatal("a new variant kept a promotion prompt")
	}
}
