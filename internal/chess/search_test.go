package chess

import (
	"runtime"
	"slices"
	"strings"
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

func TestEngineValuesStalemateAsWin(t *testing.T) {
	v := builtin(t, "stalematewins")
	p := fen(t, v, "7k/5Q2/6K1/8/8/8/8/8 b -")
	e := NewEngine(v)
	e.begin(nil, Limits{Depth: 1})
	if score := e.negamax(p, 1, 0, -infinity, infinity, true); score >= 0 || !IsMateScore(score) {
		t.Fatalf("stalemated side score = %d, want a mated (lost) score for the side to move", score)
	}
}

func TestEngineSeeksStalemateWin(t *testing.T) {
	v := builtin(t, "stalematewins")
	p := fen(t, v, "7k/5K2/8/6P1/8/8/8/8 w -")
	r := think(t, p, 3)
	after := p.Apply(r.Move)
	if len(after.Moves()) != 0 || after.InCheck() {
		t.Fatalf("engine played %+v, which does not stalemate", r.Move)
	}
	if !IsMateScore(r.Score) || r.Score <= 0 {
		t.Fatalf("score %d is not a win", r.Score)
	}
}

func TestEngineFindsMoveFromStart(t *testing.T) {
	v := builtin(t, "stalematewins")
	p := NewPosition(v)
	r := think(t, p, 2)
	if !slices.Contains(p.Moves(), r.Move) {
		t.Fatalf("engine returned illegal move %+v", r.Move)
	}
}

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

//go:noinline
func queuedSearch(e *Engine, p *Position, lim Limits, done chan<- Result) {
	done <- e.Think(p, nil, lim)
}

// queuedSearchWaiting observes B in the lock-acquisition path, rather than
// merely knowing that its goroutine was started. This also distinguishes the
// old blocking Mutex.Lock implementation from a pre-cancelled call.
func queuedSearchWaiting() bool {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	for _, stack := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(stack, "chess.queuedSearch(") &&
			strings.Contains(stack, "chess.(*Engine).think(") &&
			(strings.Contains(stack, "sync.(*Mutex).lockSlow(") || strings.Contains(stack, "time.Sleep(")) {
			return true
		}
	}
	return false
}

func TestEngineQueuedLimits(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			v := builtin(t, "bigboard")
			p := NewPosition(v)
			e := NewEngine(v)
			var stopA, stopB atomic.Bool
			doneA := make(chan Result, 1)
			doneB := make(chan Result, 1)
			go func() { doneA <- e.Think(p, nil, Limits{Depth: 48, Stop: &stopA}) }()
			defer func() {
				stopA.Store(true)
				stopB.Store(true)
				select {
				case <-doneA:
				case <-time.After(3 * time.Second):
					t.Error("search A did not exit")
				}
			}()

			// Wait until A owns the lock, not just until its goroutine starts.
			limit := time.After(3 * time.Second)
			for {
				if !e.mu.TryLock() {
					break
				}
				e.mu.Unlock()
				select {
				case <-limit:
					t.Fatal("search A never acquired the lock")
				default:
					runtime.Gosched()
				}
			}
			lim := Limits{Depth: 1, Stop: &stopB}
			if mode == "deadline" {
				lim.Time = 500 * time.Millisecond
			}
			go queuedSearch(e, p, lim, doneB)
			defer func() {
				stopB.Store(true)
				stopA.Store(true)
				select {
				case <-doneB:
				case <-time.After(3 * time.Second):
					t.Error("search B did not exit")
				}
			}()
			limit = time.After(3 * time.Second)
			for !queuedSearchWaiting() {
				select {
				case <-limit:
					t.Fatal("search B never reached the lock acquisition path")
				default:
					runtime.Gosched()
				}
			}
			if mode == "cancel" {
				stopB.Store(true)
			}
			// Deadline includes the 500ms budget; cancellation should return
			// quickly. A must still be running when B returns.
			wait := 200 * time.Millisecond
			if mode == "deadline" {
				wait += lim.Time
			}
			select {
			case result := <-doneB:
				// Put it back for the cleanup join.
				doneB <- result
				if !result.Found || !slices.Contains(p.Moves(), result.Move) || result.Depth != 0 || result.Nodes != 0 {
					t.Errorf("queued search returned invalid fallback: %+v", result)
				}
			case <-time.After(wait):
				t.Error("queued search did not return promptly while A held the lock")
			}
			select {
			case <-doneA:
				t.Error("search A finished before B")
				// Preserve the result for cleanup.
				doneA <- Result{}
			default:
			}
		})
	}
}
