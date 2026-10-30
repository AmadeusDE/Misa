//go:build !nough

package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestAIChatOpensStreamsAndReturnsToPrompt(t *testing.T) {
	isolatedUgh(t)
	m := fullShellModel(t)
	start := typeCommand(m, "aichat -m fixture/m")
	if m.screen != screenProcMon || m.procViewer == nil {
		t.Fatalf("aichat did not open: screen %v, %q", m.screen, m.shellBuffer)
	}
	p := newPump(t, m)
	p.run(start)
	p.run(typeKeys(m, "ping"))
	p.until("the reply", func() bool { return strings.Contains(ansi.Strip(m.render()), "fixture-pong") })
	files, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_DATA_HOME"), "misa", "chats", "*.json"))
	if len(files) != 1 {
		t.Fatalf("saved conversations: %v", files)
	}
	m.Update(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	if m.screen == screenProcMon || m.procViewer != nil {
		t.Error("ctrl+q did not return to the prompt")
	}
}

func TestAIChatReportsBadArguments(t *testing.T) {
	isolatedUgh(t)
	m := fullShellModel(t)
	for _, line := range []string{"aichat --nope", "aichat no-such-id"} {
		typeCommand(m, line)
		if m.screen == screenProcMon {
			t.Fatalf("%q opened the chat", line)
		}
		if last := m.shellBuffer[len(m.shellBuffer)-1]; !strings.HasPrefix(last, "aichat: ") {
			t.Errorf("%q: last line %q", line, last)
		}
	}
}

func TestAIChatIsInTheMainMenu(t *testing.T) {
	found := false
	for _, it := range buildRootItems(t.TempDir(), nil) {
		if mi, ok := it.(menuItem); ok && mi.title == "AI Chat" {
			found = true
		}
	}
	if !found {
		t.Error("no AI Chat entry in the root menu")
	}
}
