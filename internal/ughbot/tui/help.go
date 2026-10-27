package tui

import (
	"fmt"
	"strings"
)

var helpLines = []string{
	"Everywhere",
	"  ctrl+o            next screen (also while typing in the chat)",
	"  f1                this help",
	"  ctrl+c            quit",
	"",
	"Other screens (not while typing)",
	"  tab / shift+tab   next / previous screen",
	"  ] / [             next / previous screen",
	"  1-9               jump to the nth screen in the header",
	"  ? / q             help / quit",
	"  up down  /        move, filter (bots); enter opens a bot",
	"  left right  r     cost: bot filter, refresh",
	"",
	"Bots",
	"  n / e / x         new bot, edit (UI-made bots only), delete (y confirms)",
	"  p                 new bot from the bot-manager preset",
	"  in the form       tab / shift+tab move, ctrl+s saves, esc cancels",
	"                    enter is a newline in the prompt; space or left/right",
	"                    change a choice or checkbox",
	"",
	"Approvals (bots asking for bots, routines, repos or flagged calls)",
	"  up down j k       move; enter shows the exact request",
	"  a / d             approve / deny (y confirms)",
	"                    a request nobody answers expires as denied",
	"  A  r              all requests or only pending ones, refresh",
	"",
	"Routines (mail a bot on a cron schedule or once at a time)",
	"  n                 new routine; the form shows when it first fires",
	"  space / x         pause or resume / delete (y confirms)",
	"  enter  r          details, refresh",
	"",
	"Repos (code dirs bots were granted through approvals)",
	"  x  r              revoke (y confirms; its code tasks stop), refresh",
	"",
	"Devices",
	"  left right        devices, sessions, event log",
	"  up down j k       move (k ends the session on the sessions view)",
	"  r / x             rename / revoke a device (x ends a session); y confirms",
	"  d  R              dismiss the first notice, refresh",
	"",
	"Chat",
	"  enter             send",
	"  alt+enter ctrl+j  newline in the message",
	"  pgup pgdown       scroll the conversation",
	"  esc               back to the bot list",
	"",
	"Browser-reserved chords (ctrl+n/t/w, ctrl+tab, ctrl+1-9, F5, F11,",
	"F12...) are never used, so everything works in the web terminal.",
}

// helpView shows the help from line off, h rows tall; it returns the offset clamped to the text.
func helpView(off, h int) (string, int) {
	room := max(h-2, 1)
	off = min(max(off, 0), max(len(helpLines)-room, 0))
	end := min(off+room, len(helpLines))
	more := "any other key closes"
	if len(helpLines) > room {
		more = fmt.Sprintf("lines %d-%d of %d: up/down pgup/pgdn scroll, any other key closes", off+1, end, len(helpLines))
	}
	return titleStyle.Render("Help") + "  " + hintStyle.Render(more) + "\n\n" + strings.Join(helpLines[off:end], "\n"), off
}

// helpKey scrolls the help; it reports false for a key that closes it.
func (m *Model) helpKey(k string) bool {
	page := max(m.bodyHeight()-3, 1)
	switch k {
	case "up", "k":
		m.helpOff--
	case "down", "j":
		m.helpOff++
	case "pgup":
		m.helpOff -= page
	case "pgdown", "space":
		m.helpOff += page
	case "home", "g":
		m.helpOff = 0
	case "end", "G":
		m.helpOff = len(helpLines)
	default:
		return false
	}
	_, m.helpOff = helpView(m.helpOff, m.bodyHeight())
	return true
}
