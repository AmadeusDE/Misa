package chess

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func think(t *testing.T, p *Position, depth int) Result {
	t.Helper()
	r := NewEngine(p.V).Think(p, nil, Limits{Depth: depth, Time: 20 * time.Second})
	if !r.Found {
		t.Fatal("no move found")
	}
	return r
}

func moveName(p *Position, m Move) string { return describe(p, m) }

func TestEngineMatesInOne(t *testing.T) {
	v := builtin(t, "chess")
	p := fen(t, v, "6k1/5ppp/8/8/8/8/8/R5K1 w -")
	r := think(t, p, 3)
	if got := moveName(p, r.Move); got != "Ra1-a8" {
		t.Errorf("played %s, want Ra1-a8", got)
	}
	if !IsMateScore(r.Score) {
		t.Errorf("score %d is not a mate score", r.Score)
	}
}

func TestEngineMatesInTwo(t *testing.T) {
	v := builtin(t, "chess")
	m := NewMatch(v)
	m.Load(fen(t, v, "7k/8/8/8/8/8/R7/1R4K1 w -"))
	e := NewEngine(v)
	for range 3 {
		if m.Over() {
			break
		}
		r := e.Think(m.Position(), m.Hashes(), Limits{Depth: 5, Time: 20 * time.Second})
		m.Play(r.Move)
	}
	if st, winner := m.Status(); st != Checkmate || winner != White {
		t.Errorf("after %v: %v, winner %v", m.Notation(), st, winner)
	}
}

func TestEngineTakesHangingQueen(t *testing.T) {
	v := builtin(t, "chess")
	p := fen(t, v, "4k3/8/8/3q4/8/8/8/3RK3 w -")
	if got := moveName(p, think(t, p, 3).Move); got != "Rd1xd5" {
		t.Errorf("played %s, want Rd1xd5", got)
	}
}

func TestEngineRefusesPoisonedPawn(t *testing.T) {
	v := builtin(t, "chess")
	p := fen(t, v, "4k3/8/2p5/3p4/8/8/8/3QK3 w -")
	if got := moveName(p, think(t, p, 4).Move); got == "Qd1xd5" {
		t.Error("queen took a pawn defended by a pawn")
	}
}

func TestEngineEscapesMateThreat(t *testing.T) {
	v := builtin(t, "chess")
	// Black threatens Qxg2#; White has to deal with it.
	p := fen(t, v, "6k1/5ppp/8/8/8/6q1/5PPP/3R2K1 w -")
	r := think(t, p, 4)
	after := p.Apply(r.Move)
	for _, reply := range after.Moves() {
		if len(after.Apply(reply).Moves()) == 0 && after.Apply(reply).InCheck() {
			t.Fatalf("after %s Black mates with %s", moveName(p, r.Move), moveName(after, reply))
		}
	}
}

func TestEnginePlaysEveryVariant(t *testing.T) {
	for _, id := range []string{"chess", "double", "bigboard"} {
		v := builtin(t, id)
		p := NewPosition(v)
		start := time.Now()
		r := NewEngine(v).Think(p, nil, Limits{Time: 300 * time.Millisecond})
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("%s: took %v for a 300ms limit", id, elapsed)
		}
		if !r.Found || !slices.Contains(p.Moves(), r.Move) {
			t.Errorf("%s: %+v is not a legal move", id, r)
		}
		if r.Depth < 2 {
			t.Errorf("%s: only reached depth %d", id, r.Depth)
		}
	}
}

func TestEngineStops(t *testing.T) {
	v := builtin(t, "bigboard")
	p := NewPosition(v)
	var stop atomic.Bool
	go func() {
		time.Sleep(50 * time.Millisecond)
		stop.Store(true)
	}()
	start := time.Now()
	r := NewEngine(v).Think(p, nil, Limits{Time: time.Minute, Stop: &stop})
	if time.Since(start) > 2*time.Second {
		t.Error("search ignored the stop flag")
	}
	if !r.Found {
		t.Error("stopped search returned no move")
	}
}

func TestEngineHasNoMoveWhenMated(t *testing.T) {
	v := builtin(t, "chess")
	p := fen(t, v, "R5k1/5ppp/8/8/8/8/8/6K1 b -")
	if r := NewEngine(v).Think(p, nil, Limits{Depth: 2}); r.Found {
		t.Errorf("found %+v in a mated position", r)
	}
}

func TestHash(t *testing.T) {
	v := builtin(t, "chess")
	m := NewMatch(v)
	start := m.Position().Hash()
	playAll(t, m, "g1", "f3", "g8", "f6", "f3", "g1", "f6", "g8")
	if m.Position().Hash() != start {
		t.Error("knights back home hash differently")
	}
	m2 := NewMatch(v)
	playAll(t, m2, "e2", "e4")
	if m2.Position().Hash() == start {
		t.Error("different positions share a hash")
	}
	white := NewPosition(v)
	black := NewPosition(v)
	black.Turn = Black
	if white.Hash() == black.Hash() {
		t.Error("side to move not hashed")
	}
}

func TestPieceValues(t *testing.T) {
	v := builtin(t, "chess")
	if got := v.Pieces[v.TypeOf('Q')-1].Value; got != 900 {
		t.Errorf("queen = %d, want the 900 from chess.yaml", got)
	}
	src := `
id: est
name: Estimates
files: 8
ranks: 8
pieces:
  - {letter: K, name: King, betza: K, royal: true}
  - {letter: N, name: Knight, betza: N}
  - {letter: Q, name: Queen, betza: Q}
  - {letter: X, name: Cannon, betza: mRcpR}
  - {letter: P, name: Pawn, betza: fmWfcF, pawn: true}
setup: 4k3/8/8/8/8/8/8/NQXPK3
`
	vs, err := parseVariants(map[string][]byte{"e.yaml": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	est, _ := Find(vs, "est")
	value := func(c byte) int { return est.Pieces[est.TypeOf(c)-1].Value }
	if n := value('N'); n < 250 || n > 350 {
		t.Errorf("estimated knight %d, want about 300", n)
	}
	if q := value('Q'); q < 800 || q > 1000 {
		t.Errorf("estimated queen %d, want about 900", q)
	}
	if x := value('X'); x <= value('N') || x >= value('Q') {
		t.Errorf("estimated cannon %d, want between knight and queen", x)
	}
	if p := value('P'); p != 100 {
		t.Errorf("pawn = %d, want 100", p)
	}
}
