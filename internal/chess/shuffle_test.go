package chess

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestChess960Arrangements(t *testing.T) {
	vs, errs := Builtins()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	v, err := Find(vs, "chess960")
	if err != nil {
		t.Fatal(err)
	}
	if n := v.shuffleCount(); n != 960 {
		t.Fatalf("count %d", n)
	}
	if !slices.Equal(v.shuffleAt(518), v.start) {
		t.Fatal("518 is not orthodox")
	}
	seen := map[string]bool{}
	for i := 0; i < 960; i++ {
		b := v.shuffleAt(i)
		row := make([]byte, v.Files)
		var bishops, rooks []int
		king := -1
		for x := range row {
			p := b[v.Square(x, 0)]
			row[x] = v.Type(p).Letter
			if b[v.Square(x, 7)].Type != p.Type || b[v.Square(x, 7)].Color != Black {
				t.Fatal("not mirrored")
			}
			switch row[x] {
			case 'B':
				bishops = append(bishops, x)
			case 'R':
				rooks = append(rooks, x)
			case 'K':
				king = x
			}
		}
		if bishops[0]%2 == bishops[1]%2 || !(rooks[0] < king && king < rooks[1]) {
			t.Fatalf("invalid %s", row)
		}
		if seen[string(row)] {
			t.Fatalf("duplicate %s", row)
		}
		seen[string(row)] = true
	}
	a := rand.New(rand.NewPCG(1, 2))
	b := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 100; i++ {
		if !slices.Equal(v.shuffledStart(a), v.shuffledStart(b)) {
			t.Fatal("seed mismatch")
		}
	}
	if v.shuffleAt(-1) != nil || v.shuffleAt(960) != nil {
		t.Fatal("invalid index")
	}
}
func TestShuffleInvalid(t *testing.T) {
	for _, s := range []string{
		"rank: 1\n  opposite_color: K\n  between: {royal: K, pieces: R}",
		"rank: 1\n  opposite_color: P\n  between: {royal: K, pieces: R}",
		"rank: 4\n  opposite_color: B\n  between: {royal: K, pieces: R}",
	} {
		_, err := parseVariants(map[string][]byte{"bad.yaml": []byte("id: bad\nextends: chess\nshuffle:\n  " + s + "\n")})
		if err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestShuffleAndCastlingUseConfiguredPiecesAndRank(t *testing.T) {
	const src = `
id: shuffled-six
files: 6
ranks: 6
pieces:
  - {letter: S, royal: true, betza: K}
  - {letter: T, betza: R}
  - {letter: U, betza: B}
  - {letter: V, betza: N}
setup: 6/tusuvt/6/6/TUSUVT/6
shuffle: {rank: 2, opposite_color: U, between: {royal: S, pieces: T}}
castling:
  - {rook_side: right, rook_piece: T, king_to: e2, rook_to: d2}
`
	vs, err := parseVariants(map[string][]byte{"six.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	v, err := Find(vs, "shuffled-six")
	if err != nil {
		t.Fatal(err)
	}
	if v.shuffleCount() != 36 {
		t.Fatalf("arrangements = %d, want 36", v.shuffleCount())
	}
	for i := range v.shuffleCount() {
		b := v.shuffleAt(i)
		var opposite, pair []int
		royal := -1
		for x := range v.Files {
			pc := b[v.Square(x, 1)]
			if mirror := b[v.Square(x, 4)]; mirror.Type != pc.Type || mirror.Color != Black {
				t.Fatal("configured rank was not mirrored")
			}
			switch pc.Type {
			case v.TypeOf('U'):
				opposite = append(opposite, x)
			case v.TypeOf('T'):
				pair = append(pair, x)
			case v.TypeOf('S'):
				royal = x
			}
		}
		if opposite[0]%2 == opposite[1]%2 || pair[0] >= royal || pair[1] <= royal {
			t.Fatalf("invalid configured arrangement %d", i)
		}
	}
	p := fen(t, v, "6/3s2/6/6/3S1T/6 w F")
	found := false
	for _, mv := range p.Moves() {
		if mv.Castle >= 0 {
			q := p.Apply(mv)
			if q.Board[v.Square(4, 1)].Type != v.TypeOf('S') || q.Board[v.Square(3, 1)].Type != v.TypeOf('T') {
				t.Fatal("configured castling destinations were ignored")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no castle for renamed pieces on configured rank")
	}
}

func TestDynamicCastleValidation(t *testing.T) {
	for _, castle := range []string{
		"{rook_side: sideways, rook_piece: R, king_to: g1, rook_to: f1}",
		"{rook_side: right, rook_piece: X, king_to: g1, rook_to: f1}",
		"{rook_side: right, rook_piece: R, king: e1, king_to: g1, rook_to: f1}",
		"{rook_side: right, rook_piece: R, king_to: g1, rook_to: f2}",
		"{rook_side: right, rook_piece: R, king_to: g1, rook_to: g1}",
	} {
		_, err := parseVariants(map[string][]byte{"bad.yaml": []byte("id: bad\nextends: chess\ncastling: [" + castle + "]\n")})
		if err == nil {
			t.Errorf("accepted %s", castle)
		}
	}
}
