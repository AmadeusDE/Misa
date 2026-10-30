// Package cards provides original card primitives and version-one reproducible deals.
package cards

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// Suits are clubs, diamonds, spades, hearts; ranks run ace (1) to king (13).
type Card struct{ ID, Rank, Suit int }

func (c Card) Red() bool { return c.Suit%2 == 1 }
func (c Card) Label(ascii bool) string {
	if c.Rank == 0 {
		return "·"
	}
	ranks := []string{"", "A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}
	suits := []string{"♣", "♦", "♠", "♥"}
	if ascii {
		suits = []string{"C", "D", "S", "H"}
	}
	return ranks[c.Rank] + suits[c.Suit]
}
func Deck() []Card {
	d := make([]Card, 0, 52)
	for s := 0; s < 4; s++ {
		for r := 1; r <= 13; r++ {
			d = append(d, Card{len(d), r, s})
		}
	}
	return d
}
func SpiderDeck(suits int) []Card {
	d := make([]Card, 0, 104)
	for copy := 0; copy < 8/suits; copy++ {
		for s := 0; s < suits; s++ {
			for r := 1; r <= 13; r++ {
				suit := s
				if suits == 1 {
					suit = 2
				}
				d = append(d, Card{len(d), r, suit})
			}
		}
	}
	return d
}
func FreshSeed() (uint64, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	return binary.LittleEndian.Uint64(b[:]), err
}

// Shuffle freezes SplitMix64, rejection sampling, and descending Fisher–Yates as v1.
func Shuffle(d []Card, seed uint64) {
	next := func() uint64 {
		seed += 0x9e3779b97f4a7c15
		z := seed
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		return z ^ (z >> 31)
	}
	for i := len(d) - 1; i > 0; i-- {
		j := int(bounded(next, uint64(i+1)))
		d[i], d[j] = d[j], d[i]
	}
}
func bounded(next func() uint64, n uint64) uint64 {
	threshold := -n % n
	x := next()
	for x < threshold {
		x = next()
	}
	return x % n
}
func DealID(game string, seed uint64, option int) string {
	return fmt.Sprintf("v1:%s:%d:%d", game, seed, option)
}
func Copy(p []Card) []Card {
	if p == nil {
		return nil
	}
	return append([]Card{}, p...)
}
func Descending(p []Card, sameSuit bool) bool {
	for i := 1; i < len(p); i++ {
		if p[i-1].Rank != p[i].Rank+1 {
			return false
		}
		if sameSuit {
			if p[i-1].Suit != p[i].Suit {
				return false
			}
		} else if p[i-1].Red() == p[i].Red() {
			return false
		}
	}
	return len(p) > 0
}
func Top(p []Card) Card {
	if len(p) == 0 {
		return Card{}
	}
	return p[len(p)-1]
}
func Foundation(c Card, p []Card) bool {
	return c.Rank > 0 && c.Rank == len(p)+1 && (len(p) == 0 || Top(p).Suit == c.Suit)
}

type History[T any] struct{ past, future []T }

func (h *History[T]) Push(s T) { h.past = append(h.past, s); h.future = nil }
func (h *History[T]) Undo(current T) (T, bool) {
	if len(h.past) == 0 {
		return current, false
	}
	h.future = append(h.future, current)
	s := h.past[len(h.past)-1]
	h.past = h.past[:len(h.past)-1]
	return s, true
}
func (h *History[T]) Redo(current T) (T, bool) {
	if len(h.future) == 0 {
		return current, false
	}
	h.past = append(h.past, current)
	s := h.future[len(h.future)-1]
	h.future = h.future[:len(h.future)-1]
	return s, true
}
