package tui

import (
	"fmt"
	"strings"

	"github.com/AmadeusDE/Misa/internal/ughbot/api"
)

type statusMsg struct {
	st  api.Status
	err error
}

type statusModel struct {
	st     api.Status
	err    error
	loaded bool
}

func (s *statusModel) load(msg statusMsg) {
	s.loaded, s.err = true, msg.err
	if msg.err == nil {
		s.st = msg.st
	}
}

func (s *statusModel) view(w, h int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Daemon status") + "\n\n")
	if s.err != nil {
		b.WriteString(errStyle.Render("status unavailable: "+s.err.Error()) + "\n")
	}
	if !s.loaded {
		b.WriteString(dimStyle.Render("loading..."))
		return b.String()
	}
	if s.err == nil {
		fmt.Fprintf(&b, "%s %s\n%s %s\n", dimStyle.Render("version"), s.st.Version, dimStyle.Render("uptime "), uptime(s.st.UptimeSeconds))
		if s.st.VM != "" {
			fmt.Fprintf(&b, "%s %s\n", dimStyle.Render("vm     "), s.st.VM)
		}
		b.WriteString("\n")
		b.WriteString(headStyle.Render(fmt.Sprintf("%-16s %-10s %s", "BOT", "STATE", "MODEL")) + "\n")
		for _, bot := range s.st.Bots {
			fmt.Fprintf(&b, "%-16s %s %s\n", clip(bot.Name, 16), stateStyle(bot.State).Render(fmt.Sprintf("%-10s", bot.State)), dimStyle.Render(bot.Model))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
