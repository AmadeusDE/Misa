package cardui

import "github.com/AmadeusDE/Misa/internal/cards"

func VisiblePile(name string, p []cards.Card, hidden int) Pile {
	copy := cards.Copy(p)
	for i := 0; i < hidden && i < len(copy); i++ {
		copy[i] = cards.Card{}
	}
	return Pile{Name: name, Cards: copy, Hidden: hidden}
}
func SafeFoundation(c cards.Card, foundations [4][]cards.Card) bool {
	if c.Rank <= 2 {
		return true
	}
	for suit, p := range foundations {
		required := c.Rank - 2
		if suit%2 != c.Suit%2 {
			required = c.Rank - 1
		}
		if suit != c.Suit && len(p) < required {
			return false
		}
	}
	return true
}
