package tui

import (
	"fmt"
	"strings"
	"time"
)

var partialBlocks = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}

// bar draws frac (0..1) of width cells using eighth-block characters.
func bar(width int, frac float64) string {
	if width <= 0 {
		return ""
	}
	frac = min(max(frac, 0), 1)
	eighths := int(frac*float64(width)*8 + 0.5)
	full, rem := eighths/8, eighths%8
	out := strings.Repeat("█", full)
	if rem > 0 {
		out += string(partialBlocks[rem])
	}
	return out
}

func usd(v float64) string { return fmt.Sprintf("$%.4f", v) }

func uptime(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd %dh %dm", int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func clip(s string, limit int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > limit {
		return string(r[:limit]) + "..."
	}
	return string(r)
}
