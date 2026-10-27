package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type costMsg struct {
	bot  string
	rows []api.CostRow
	err  error
}

type costModel struct {
	rows   []api.CostRow
	filter string
	err    error
	loaded bool
	scroll int
}

func (c *costModel) load(msg costMsg) {
	if msg.bot != c.filter {
		return
	}
	c.loaded, c.err = true, msg.err
	if msg.err == nil {
		c.rows = msg.rows
		sort.SliceStable(c.rows, func(i, j int) bool {
			if c.rows[i].Day != c.rows[j].Day {
				return c.rows[i].Day > c.rows[j].Day
			}
			return c.rows[i].Bot < c.rows[j].Bot
		})
	}
	c.scroll = 0
}

// cycle steps the bot filter through "" (all) and names; it reports whether it changed.
func (c *costModel) cycle(names []string, step int) {
	all := append([]string{""}, names...)
	at := 0
	for i, n := range all {
		if n == c.filter {
			at = i
		}
	}
	c.filter = all[(at+step+len(all))%len(all)]
	c.loaded = false
}

func (c *costModel) key(msg tea.KeyPressMsg, names []string) (refetch bool) {
	switch msg.String() {
	case "left", "h":
		c.cycle(names, -1)
		return true
	case "right", "l", "b":
		c.cycle(names, 1)
		return true
	case "up", "k":
		c.scroll = max(c.scroll-1, 0)
	case "down", "j":
		c.scroll++
	case "r":
		return true
	}
	return false
}

func (c *costModel) totals() (perBot map[string]float64, bots []string, sum, top float64) {
	perBot = map[string]float64{}
	for _, r := range c.rows {
		perBot[r.Bot] += r.USD
		sum += r.USD
		top = max(top, r.USD)
	}
	for b := range perBot {
		bots = append(bots, b)
	}
	sort.Strings(bots)
	return
}

func (c *costModel) view(w, h int) string {
	var b strings.Builder
	scope := "all bots"
	if c.filter != "" {
		scope = c.filter
	}
	b.WriteString(titleStyle.Render("Cost") + dimStyle.Render("  bot: ") + scope + "\n\n")
	if c.err != nil {
		b.WriteString(errStyle.Render("cost unavailable: "+c.err.Error()) + "\n")
		return b.String()
	}
	if !c.loaded {
		b.WriteString(dimStyle.Render("loading..."))
		return b.String()
	}
	if len(c.rows) == 0 {
		b.WriteString(dimStyle.Render("no spend recorded"))
		return b.String()
	}
	perBot, bots, sum, top := c.totals()
	barW := max(min(w-40, 40), 8)
	maxBot := 0.0
	for _, v := range perBot {
		maxBot = max(maxBot, v)
	}
	b.WriteString(headStyle.Render("Total per bot") + "\n")
	for _, name := range bots {
		fmt.Fprintf(&b, "%-14s %10s %s\n", clip(name, 14), usd(perBot[name]), userStyle.Render(bar(barW, perBot[name]/maxBot)))
	}
	fmt.Fprintf(&b, "%-14s %10s\n\n", "all", usd(sum))
	b.WriteString(headStyle.Render(fmt.Sprintf("%-10s %-14s %10s", "DAY", "BOT", "USD")) + "\n")
	used := strings.Count(b.String(), "\n") + 1
	rows := max(h-used, 1)
	c.scroll = min(c.scroll, max(len(c.rows)-rows, 0))
	for i := c.scroll; i < len(c.rows) && i < c.scroll+rows; i++ {
		r := c.rows[i]
		fmt.Fprintf(&b, "%-10s %-14s %10s %s\n", r.Day, clip(r.Bot, 14), usd(r.USD), userStyle.Render(bar(barW, r.USD/top)))
	}
	return strings.TrimRight(b.String(), "\n")
}
