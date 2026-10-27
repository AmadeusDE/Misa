package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type botsMsg struct {
	bots []api.BotInfo
	err  error
}

type pickerModel struct {
	bots      []api.BotInfo
	cursor    int
	filter    string
	filtering bool
	err       error
	loaded    bool
}

func (p *pickerModel) visible() []api.BotInfo {
	if p.filter == "" {
		return p.bots
	}
	var out []api.BotInfo
	for _, b := range p.bots {
		if strings.Contains(strings.ToLower(b.Name+" "+b.Model+" "+b.State), strings.ToLower(p.filter)) {
			out = append(out, b)
		}
	}
	return out
}

func (p *pickerModel) clamp() {
	p.cursor = min(max(p.cursor, 0), max(len(p.visible())-1, 0))
}

func (p *pickerModel) load(msg botsMsg) {
	p.loaded, p.err = true, msg.err
	if msg.err == nil {
		p.bots = msg.bots
	}
	p.clamp()
}

// key returns the chosen bot name when enter picks one.
func (p *pickerModel) key(msg tea.KeyPressMsg) (picked string) {
	if p.filtering {
		switch msg.String() {
		case "enter":
			p.filtering = false
		case "esc":
			p.filtering, p.filter = false, ""
		case "backspace":
			if r := []rune(p.filter); len(r) > 0 {
				p.filter = string(r[:len(r)-1])
			}
		default:
			p.filter += msg.Text
		}
		p.clamp()
		return ""
	}
	switch msg.String() {
	case "up", "k":
		p.cursor--
	case "down", "j":
		p.cursor++
	case "home", "g":
		p.cursor = 0
	case "end", "G":
		p.cursor = len(p.visible()) - 1
	case "/":
		p.filtering = true
	case "esc":
		p.filter = ""
	case "enter":
		if vis := p.visible(); len(vis) > 0 {
			return vis[p.cursor].Name
		}
	}
	p.clamp()
	return ""
}

func (p *pickerModel) view(w, h int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Bots"))
	switch {
	case p.filtering:
		b.WriteString("  /" + p.filter + "_")
	case p.filter != "":
		b.WriteString(dimStyle.Render("  filter: " + p.filter))
	}
	b.WriteString("\n\n")
	if p.err != nil {
		b.WriteString(errStyle.Render("cannot list bots: "+p.err.Error()) + "\n")
	}
	vis := p.visible()
	if len(vis) == 0 && p.err == nil {
		msg := "no bots yet: press p to start from the bot-manager preset (a bot that creates the others,\n" +
			"each one after your approval), or n for a blank bot"
		switch {
		case !p.loaded:
			msg = "loading..."
		case p.filter != "":
			msg = "no bot matches the filter"
		}
		b.WriteString(dimStyle.Render(msg))
	}
	rows := max((h-3)/2, 1)
	start := min(max(p.cursor-rows+1, 0), max(len(vis)-rows, 0))
	for i := start; i < len(vis) && i < start+rows; i++ {
		bot := vis[i]
		name := fmt.Sprintf("%-16s", clip(bot.Name, 16))
		src := ""
		if !bot.Editable {
			src = dimStyle.Render("  (toml)")
		}
		state := stateStyle(bot.State).Render(fmt.Sprintf("%-10s", bot.State))
		lead := "  " + name
		if i == p.cursor {
			lead = selectedStyle.Render("> " + name)
		}
		b.WriteString(lead + " " + state + "  " + dimStyle.Render(bot.Model) + src + "\n")
		if bot.Capabilities != "" {
			b.WriteString(dimStyle.Render(fmt.Sprintf("%-30s%s", "", cut("may: "+bot.Capabilities, max(w-31, 10)))) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
