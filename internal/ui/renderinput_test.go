package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEmptyPromptShowsWholePlaceholder(t *testing.T) {
	m := fullShellModel(t)
	if view := ansi.Strip(m.render()); !strings.Contains(view, "enter a command...") {
		t.Errorf("placeholder cut short in:\n%s", view)
	}
}
