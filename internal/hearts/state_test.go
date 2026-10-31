package hearts

import "testing"

func TestBotMatches(t *testing.T) {
	for seed := uint64(0); seed < 12; seed++ {
		s := NewState(seed, 30)
		for n := 0; !s.Over && n < 10000; n++ {
			if s.Passing {
				var pass [4][]int
				for p := range pass {
					pass[p] = ChoosePass(s.Observe(p))
				}
				if !s.Pass(pass) {
					t.Fatal("pass")
				}
			} else if s.RoundOver {
				s.NextRound()
			} else {
				p := s.Turn
				if !s.Play(p, ChoosePlay(s.Observe(p))) {
					t.Fatal("AI illegal")
				}
			}
		}
		if !s.Over {
			t.Fatal("nontermination", seed)
		}
	}
}
