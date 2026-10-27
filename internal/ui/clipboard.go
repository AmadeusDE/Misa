package ui

import (
	"github.com/AmadeusDE/Misa/internal/clipboard"

	tea "charm.land/bubbletea/v2"
)

func readClipboard() string {
	text, err := clipboard.Read()
	if err != nil {
		return ""
	}
	return text
}

// writeClipboard copies text with the host tool. When that tool is missing
// (Termux without termux-api, or no xclip/wl-copy) the command is OSC 52.
func writeClipboard(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	if clipboard.Write(text) == nil {
		return nil
	}
	return tea.SetClipboard(text)
}
