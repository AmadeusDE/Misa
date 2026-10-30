package mines

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// customSetup is separate from the live game so cancellation leaves the board intact.
type customSetup struct {
	level  Level
	field  int
	paused time.Time
}

func (g *Game) setupKey(k string) tea.Cmd {
	s := g.setup
	switch k {
	case "c", "esc":
		g.setup = nil
		if !g.started.IsZero() && g.ended.IsZero() {
			g.started = g.started.Add(g.now().Sub(s.paused))
			return g.tick()
		}
	case "enter":
		g.setup = nil
		g.newGame(s.level)
	case "up", "k":
		s.field = (s.field + 2) % 3
	case "down", "j":
		s.field = (s.field + 1) % 3
	case "left", "h", "right", "l":
		delta := 1
		if k == "left" || k == "h" {
			delta = -1
		}
		switch s.field {
		case 0:
			s.level.W = max(MinWidth, min(MaxWidth, s.level.W+delta))
		case 1:
			s.level.H = max(MinHeight, min(MaxHeight, s.level.H+delta))
		case 2:
			s.level.Mines = max(MinMines, min(min(MaxMines, s.level.W*s.level.H-1), s.level.Mines+delta))
		}
		s.level.Mines = max(MinMines, min(min(MaxMines, s.level.W*s.level.H-1), s.level.Mines))
	}
	return nil
}

func (g *Game) customView() string {
	s := g.setup
	lines := []string{"Custom game", ""}
	fields := []struct {
		name  string
		value int
	}{{"Width", s.level.W}, {"Height", s.level.H}, {"Mines", s.level.Mines}}
	for i, f := range fields {
		marker := "  "
		if i == s.field {
			marker = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%-6s  %d", marker, f.name, f.value))
	}
	lines = append(lines, "")
	l := boardLayout(s.level.W, s.level.H, g.width, g.height)
	if !l.fits {
		lines = append(lines, fmt.Sprintf("Board won't fit (needs %d×%d)", l.boardW, l.rows+chromeLines))
	} else {
		lines = append(lines, "Board fits")
	}
	lines = append(lines, "", "↑/↓ select  ←/→ adjust", "enter start  c/esc cancel")
	top := max(0, (g.height-len(lines))/2)
	out := make([]string, top, g.height)
	for _, line := range lines {
		if len(out) >= g.height {
			break
		}
		out = append(out, g.centre(truncate(line, g.width)))
	}
	for len(out) < g.height {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}
