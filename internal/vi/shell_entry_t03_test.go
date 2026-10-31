package vi

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestT03StandaloneInvocationEnvironment(t *testing.T) {
	t03Shell(t)
	e := newStandaloneEditor("", t.TempDir(), []string{"MISA_T03_STANDALONE=invocation"})
	e.Resize(80, 20)
	t03Keys(t, e, ":!printf '%s' \"$MISA_T03_STANDALONE\"<CR>")
	if view := e.View(); !strings.Contains(view, "invocation") {
		t.Fatalf("standalone did not inherit invocation environment: %q", view)
	}
}

func TestT03StandaloneShellLifecycle(t *testing.T) {
	t03Shell(t)
	e, _ := f07Editor(t, "original\n")
	m := &standaloneModel{editor: e}
	var cmd tea.Cmd
	for _, key := range replayKeys(t, ":!printf 'standalone-output\\n'; printf 'standalone-error\\n' >&2; exit 3<CR>") {
		_, cmd = m.Update(key)
	}
	if cmd == nil {
		t.Fatal("standalone shell did not return an asynchronous command")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	_, next := m.Update(cmd())
	if next != nil || e.Quit {
		t.Fatal("shell completion quit standalone editor")
	}
	view := m.View().Content
	for _, text := range []string{"standalone-output", "standalone-error", "shell returned 3"} {
		if !strings.Contains(view, text) {
			t.Errorf("standalone output missing %q: %q", text, view)
		}
	}
	for _, key := range replayKeys(t, "<CR>:q<CR>") {
		_, cmd = m.Update(key)
	}
	if !e.Quit || cmd == nil {
		t.Fatal("standalone editor did not quit after dismissing output")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("standalone editor did not emit quit")
	}
}
