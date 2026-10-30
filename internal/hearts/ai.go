package hearts

import "sort"

func ChoosePass(o Observation) []int {
	indices := make([]int, len(o.Hand))
	counts := [4]int{}
	for i, c := range o.Hand {
		indices[i] = i
		counts[c.Suit]++
	}
	risk := func(i int) int {
		c := o.Hand[i]
		n := high(c) * 3
		if c.Suit == 2 && high(c) >= 12 {
			n += 40
		}
		if c.Suit == 3 {
			n += 10
		}
		if counts[c.Suit] <= 3 {
			n += 12
		}
		return n
	}
	sort.SliceStable(indices, func(a, b int) bool { return risk(indices[a]) > risk(indices[b]) })
	return append([]int(nil), indices[:min(3, len(indices))]...)
}
func ChoosePlay(o Observation) int {
	if len(o.Legal) == 0 {
		return -1
	}
	best := o.Legal[0]
	moonSeat := -1
	for p, n := range o.Points {
		if p != o.Seat && n >= 10 {
			only := true
			for q, m := range o.Points {
				if q != p && m > 0 {
					only = false
				}
			}
			if only {
				moonSeat = p
			}
		}
	}
	value := func(i int) int {
		c := o.Hand[i]
		r := high(c)
		if len(o.Trick) == 0 {
			n := -r
			if c.Suit == 2 && r >= 12 {
				n -= 30
			}
			for p := 0; p < 4; p++ {
				if p != o.Seat && o.Void[p][c.Suit] {
					n -= 15
				}
			}
			return n
		}
		winner := o.Trick[0]
		pts := 0
		for _, v := range o.Trick {
			pts += penalty(v.Card)
			if v.Card.Suit == winner.Card.Suit && high(v.Card) > high(winner.Card) {
				winner = v
			}
		}
		if c.Suit != winner.Card.Suit {
			if moonSeat == winner.Seat {
				return -penalty(c)*100 + r
			}
			return penalty(c)*20 + r
		}
		wins := r > high(winner.Card)
		if moonSeat == winner.Seat && pts > 0 && wins {
			return 500 - r
		}
		if !wins {
			return 100 + r
		}
		if len(o.Trick) == 3 && pts == 0 && penalty(c) == 0 {
			return 50 + r
		}
		return -r - pts*10
	}
	for _, i := range o.Legal {
		if value(i) > value(best) {
			best = i
		}
	}
	return best
}
