package cardui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/AmadeusDE/Misa/internal/cards"
)

func (g *Game) headerLines() []string {
	wrap := func(text string) []string {
		return strings.Split(lipgloss.NewStyle().Width(max(1, g.width)).Render(text), "\n")
	}
	deal := wrap(cards.DealID(g.id, g.opts.Seed, g.opts.Rule))
	status := wrap(g.table.Status())
	// Reserve the controls, pile heading, one playable row and footer at minimum size.
	limit := min(4, max(1, g.height-len(deal)-8))
	if len(status) > limit {
		status = append(status[:limit-1], "? more status in Help")
	}
	return append(deal, status...)
}
