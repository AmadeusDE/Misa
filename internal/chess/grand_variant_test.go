package chess

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// grandRows is Grand Chess's starting setup from the tenth rank down, with a
// dot for each empty square. The rooks stand alone on the first rank, the other
// pieces on the second and the pawns on the third.
var grandRows = []string{
	"r........r", // 10
	".nbqkmcbn.", // 9
	"pppppppppp", // 8
	"..........", // 7
	"..........", // 6
	"..........", // 5
	"..........", // 4
	"PPPPPPPPPP", // 3
	".NBQKMCBN.", // 2
	"R........R", // 1
}

func TestGrandChessLoads(t *testing.T) {
	v := builtin(t, "grand")
	if v.Name != "Grand Chess" || v.Files != 10 || v.Ranks != 10 {
		t.Fatalf("%s: %q on %d×%d", v.ID, v.Name, v.Files, v.Ranks)
	}
	if got := len(v.Pieces); got != 8 {
		t.Fatalf("%d piece types, want 8", got)
	}
	for letter, want := range map[byte]string{'M': "RN", 'C': "BN", 'P': "fmWfcFifmW2", 'Q': "Q"} {
		if p := v.Pieces[v.TypeOf(letter)-1]; p.Betza != want {
			t.Errorf("%c betza = %q, want %q", letter, p.Betza, want)
		}
	}
	if !v.Pieces[v.TypeOf('K')-1].Royal || !v.Pieces[v.TypeOf('P')-1].Pawn {
		t.Error("inherited king/pawn flags lost")
	}
	var promo []string
	for _, typ := range v.Promotion {
		promo = append(promo, string(v.Pieces[typ-1].Letter))
	}
	if want := []string{"Q", "R", "B", "N", "M", "C"}; !slices.Equal(promo, want) {
		t.Errorf("promotion = %v, want %v", promo, want)
	}
	// Pawns promote from their eighth rank on: the last three ranks. The move
	// limit is the usual one, as Freeling's rules add no drawing condition.
	if v.PromotionZone != 3 || v.MoveLimit != 50 {
		t.Errorf("promotion zone %d, move limit %d", v.PromotionZone, v.MoveLimit)
	}
	// H. G. Muller's values, scaled for ten files.
	for letter, want := range map[byte]int{'Q': 950, 'R': 500, 'B': 350, 'N': 300, 'M': 900, 'C': 875, 'P': 100} {
		if got := v.Pieces[v.TypeOf(letter)-1].Value; got != want {
			t.Errorf("%c value = %d, want %d", letter, got, want)
		}
	}
	// Both promotion rules come from the file, so a typo there would show here.
	if !v.PromotionOptional || !v.PromotionCapturedOnly {
		t.Errorf("promotion flags: optional %v, captured-only %v", v.PromotionOptional, v.PromotionCapturedOnly)
	}
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatalf("builtin variants: %v", errs)
	}
	found, err := Find(vs, "Grand Chess")
	if err != nil || found.ID != "grand" {
		t.Errorf("Find(Grand Chess) = %v, %v", found, err)
	}
}

func TestGrandChessSetup(t *testing.T) {
	v := builtin(t, "grand")
	p := NewPosition(v)
	for rank, row := range grandRows {
		if len(row) != v.Files {
			t.Fatalf("rank %d has %d files", 10-rank, len(row))
		}
		for file := range row {
			letter, sq := row[file], (9-rank)*v.Files+file
			if letter == '.' {
				if p.Board[sq].Type != 0 {
					t.Errorf("%s is not empty: %+v", v.SquareName(sq), p.Board[sq])
				}
				continue
			}
			pc, want := p.Board[sq], v.TypeOf(byte(letter))
			side := White
			if strings.ToLower(string(letter)) == string(letter) {
				side = Black
			}
			if pc.Type != want || pc.Color != side || pc.Moved {
				t.Errorf("%s: %+v, want %c %s", v.SquareName(sq), pc, letter, side)
			}
		}
	}
	// Ten pieces and ten pawns a side.
	for _, side := range []Color{White, Black} {
		n := 0
		for _, pc := range p.Board {
			if pc.Type != 0 && pc.Color == side {
				n++
			}
		}
		if n != 20 {
			t.Errorf("%s has %d pieces, want 20", side, n)
		}
	}
	// Pawns keep the usual first move of one or two squares, but not more.
	if got, want := destinations(p, "e3"), []string{"e4", "e5"}; !slices.Equal(got, want) {
		t.Errorf("e3 pawn: %v, want %v", got, want)
	}
	// The rooks are unblocked from the start: one step up, then the whole rank.
	if got := destinations(p, "a1"); len(got) != 9 || got[0] != "a2" || got[8] != "i1" {
		t.Errorf("a1 rook has %v", got)
	}
	moves := p.Moves()
	black := p.Apply(moves[0])
	if len(black.Moves()) != len(moves) {
		t.Errorf("Black has %d replies to White's %d opening moves", len(black.Moves()), len(moves))
	}
}

func TestGrandChessHasNoCastling(t *testing.T) {
	v := builtin(t, "grand")
	if len(v.Castling) != 0 {
		t.Fatalf("inherited %d castling moves: %+v", len(v.Castling), v.Castling)
	}
	// Even with chess's own castling squares occupied, the king only steps.
	p := fen(t, v, "9k/10/10/10/10/10/10/10/10/4K2R2 w -")
	if got, want := destinations(p, "e1"), []string{"d1", "d2", "e2", "f1", "f2"}; !slices.Equal(got, want) {
		t.Errorf("king from e1: %v, want %v", got, want)
	}
}

func TestGrandChessSearch(t *testing.T) {
	v := builtin(t, "grand")
	p := NewPosition(v)
	start := time.Now()
	r := NewEngine(v).Think(p, nil, Limits{Time: 300 * time.Millisecond})
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v for a 300ms limit", elapsed)
	}
	if !r.Found || !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("computer returned illegal move: %+v", r)
	}
	if r.Depth < 2 {
		t.Errorf("only reached depth %d", r.Depth)
	}
	// A queen hanging in the open is taken.
	q := fen(t, v, "9k/3q6/10/10/10/10/10/10/3Q6/4K5 w -")
	best := think(t, q, 3)
	if q.Board[best.Move.To].Type != v.TypeOf('Q') {
		t.Errorf("played %s instead of taking the queen", moveName(q, best.Move))
	}
}

func TestGrandChessView(t *testing.T) {
	g := newGame(t, "grand", 80, 24)
	out := g.View()
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > 80 {
			t.Errorf("line %d is %d wide", i, w)
		}
	}
	if strings.Contains(out, "too small") {
		t.Error("10×10 board does not fit 80×24")
	}
}

// grandWithPawn is White's untouched army with the pawn of its third rank moved
// to the named square, so no piece of White's has been captured and
// captured-only promotion offers nothing. The black king sits on d10, where a
// pawn on e9 checks it.
func grandWithPawn(t *testing.T, v *Variant, pawn string) *Position {
	t.Helper()
	rows := map[int]string{1: "R........R", 2: ".NBQKMCBN.", 3: "PPPP.PPPPP", 10: "...k......"}
	for rank := 1; rank <= 10; rank++ {
		if rows[rank] == "" {
			rows[rank] = strings.Repeat(".", 10)
		}
	}
	y := int(pawn[1] - '1')
	row := []byte(rows[y+1])
	row[pawn[0]-'a'] = 'P'
	rows[y+1] = string(row)
	var ranks []string
	for rank := 10; rank >= 1; rank-- {
		ranks = append(ranks, compressRow(rows[rank]))
	}
	return fen(t, v, strings.Join(ranks, "/")+" w -")
}

// compressRow writes a rank of dots and letters the way a FEN does.
func compressRow(row string) string {
	var b strings.Builder
	for i := 0; i < len(row); {
		if row[i] != '.' {
			b.WriteByte(row[i])
			i++
			continue
		}
		j := i
		for j < len(row) && row[j] == '.' {
			j++
		}
		b.WriteString(strconv.Itoa(j - i))
		i = j
	}
	return b.String()
}

func TestGrandPawnPromotionRanks(t *testing.T) {
	v := builtin(t, "grand")
	// Alone with the king, every type counts as captured: the seventh rank
	// offers both the plain arrival and all six pieces.
	p := fen(t, v, "9k/10/10/4P5/10/10/10/10/10/4K5 w -")
	plain, promos := rulesMoves(t, p, "e7")
	if want := []string{"e8"}; !slices.Equal(plain, want) {
		t.Errorf("e7-e8 without promoting: %v, want %v", plain, want)
	}
	if want := []string{"e8=B", "e8=C", "e8=M", "e8=N", "e8=Q", "e8=R"}; !slices.Equal(promos, want) {
		t.Errorf("promotions on the eighth rank: %v, want %v", promos, want)
	}
	// The same on the ninth rank, still short of the last one.
	p = fen(t, v, "9k/10/4P5/10/10/10/10/10/10/4K5 w -")
	if plain, promos := rulesMoves(t, p, "e8"); len(plain) != 1 || len(promos) != 6 {
		t.Errorf("e8 pawn: plain %v promotions %v, want both", plain, promos)
	}
	// On the tenth rank the promotion is forced.
	p = fen(t, v, "9k/4P5/10/10/10/10/10/10/10/4K5 w -")
	plain, promos = rulesMoves(t, p, "e9")
	if len(plain) != 0 {
		t.Errorf("e9 pawn declined on the last rank: %v", plain)
	}
	if want := []string{"e10=B", "e10=C", "e10=M", "e10=N", "e10=Q", "e10=R"}; !slices.Equal(promos, want) {
		t.Errorf("promotions on the tenth rank: %v, want %v", promos, want)
	}
	// Leaving the pawn as a pawn keeps it a pawn, and resets the move clock.
	p = fen(t, v, "9k/10/10/4P5/10/10/10/10/10/4K5 w -")
	declined := p.Apply(rulesDecline(t, p, "e7", "e8"))
	if pc := declined.Board[rulesSquare(t, v, "e8")]; pc.Type != v.TypeOf('P') || declined.Clock != 0 {
		t.Errorf("declined promotion left %+v, clock %d", pc, declined.Clock)
	}
}

func TestGrandPromotionOnlyToACapturedPiece(t *testing.T) {
	v := builtin(t, "grand")
	// White lost nothing, so its pawn inside the zone may only stay a pawn.
	p := grandWithPawn(t, v, "e7")
	if plain, promos := rulesMoves(t, p, "e7"); !slices.Equal(plain, []string{"e8"}) || len(promos) != 0 {
		t.Errorf("e7 pawn: plain %v promotions %v, want e8 alone", plain, promos)
	}
	// A captured queen is one choice, and only that one.
	lost := grandWithPawn(t, v, "e7")
	lost.Board[rulesSquare(t, v, "d2")] = Piece{}
	if plain, promos := rulesMoves(t, lost, "e7"); len(plain) != 1 || !slices.Equal(promos, []string{"e8=Q"}) {
		t.Errorf("after losing the queen: plain %v promotions %v, want e8=Q", plain, promos)
	}
	// A pawn on the ninth rank with nothing captured cannot enter the tenth
	// rank by push or by capture, yet it still attacks it: the king on d10 is
	// in check and may not step onto f10 or d10 itself.
	p = grandWithPawn(t, v, "e9")
	if plain, promos := rulesMoves(t, p, "e9"); len(plain)+len(promos) != 0 {
		t.Errorf("e9 pawn entered the tenth rank: %v %v", plain, promos)
	}
	if cplain, cpromos := rulesCaptures(t, p, "e9"); len(cplain)+len(cpromos) != 0 {
		t.Errorf("quiescence moved the e9 pawn: %v %v", cplain, cpromos)
	}
	p.Turn = Black
	if !p.InCheck() {
		t.Error("the frozen pawn on e9 gives no check")
	}
	if len(p.Moves()) == 0 {
		t.Error("Black has no way out of the check")
	}
	// Losing one rook frees the rook promotion, and only that one.
	freed := grandWithPawn(t, v, "e9")
	freed.Board[rulesSquare(t, v, "j1")] = Piece{}
	// Advancing and taking the checking king are the same rook promotion.
	if plain, promos := rulesMoves(t, freed, "e9"); len(plain) != 0 ||
		!slices.Equal(promos, []string{"d10=R", "e10=R"}) {
		t.Errorf("after losing the j1 rook: plain %v promotions %v, want the rook alone", plain, promos)
	}
}

func TestGrandDeclinePromotionOnScreen(t *testing.T) {
	g := newGame(t, "grand", 90, 30)
	g.Load(fen(t, g.match.Variant(), "9k/10/10/4P5/10/10/10/10/10/4K5 w -"))
	clickSquare(t, g, "e7", tea.MouseLeft)
	clickSquare(t, g, "e8", tea.MouseLeft)
	if len(g.promo) != 7 {
		t.Fatalf("%d choices %+v, want the pawn and six pieces", len(g.promo), g.promo)
	}
	if g.promo[0].Promote != 0 {
		t.Fatalf("first choice promotes to %d, want the plain move", g.promo[0].Promote)
	}
	keys(g, "p")
	if got := g.match.Notation(); len(got) != 1 || got[0] != "e7-e8" {
		t.Fatalf("notation = %v, want e7-e8", got)
	}
	if pc := g.match.Position().Board[rulesSquare(t, g.match.Variant(), "e8")]; pc.Type != g.match.Variant().TypeOf('P') {
		t.Errorf("e8 holds type %d, want the pawn", pc.Type)
	}
}
