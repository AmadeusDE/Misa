package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// These exercise commands through the root mux, including delivery of the
// command's routed result, rather than calling the editor directly.
func TestEditorShellLifecycleThroughMux(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh unavailable")
	}
	for _, tc := range []struct {
		name, command, want string
	}{
		{"shell", ":!printf 'first\\nsecond\\n'; printf 'shell-error\\n' >&2; exit 7", "first|second|shell-error|7"},
		{"read", ":r !printf 'read-one\\nread-two\\n'", "read-one|read-two"},
		{"filter", ":%!printf 'filter-one\\nfilter-two\\n'", "filter-one|filter-two"},
		{"write", ":w !printf 'write-one\\nwrite-two\\n'; printf 'write-error\\n' >&2; exit 7", "write-one|write-two|write-error|7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			m := newRoot(t, 80, 24)
			m.addMuxPane()
			pane := m.activePane()
			path := filepath.Join(home, "sample")
			if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
				t.Fatal(err)
			}
			pane.misa.screen = screenShell
			pane.misa.enterEditor(path)
			editor := pane.misa.viEditor
			for _, r := range tc.command {
				m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("shell operation must return an asynchronous command")
			}
			// Resize while the operation is pending, then route its result through
			// the same root Update path Bubble Tea uses.
			m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
			msg := cmd()
			if _, quit := msg.(tea.QuitMsg); quit {
				t.Fatal("shell operation quit Misa")
			}
			if _, ok := msg.(paneMsg); !ok {
				t.Fatalf("result is not pane-routed: %T", msg)
			}
			m.Update(msg)
			if m.activePane() != pane || pane.misa.viEditor != editor || pane.misa.screen != screenEditor {
				t.Fatal("shell operation left its editor pane")
			}
			view := ansi.Strip(editor.View())
			for _, s := range strings.Split(tc.want, "|") {
				if !strings.Contains(view, s) {
					t.Errorf("view missing %q: %q", s, view)
				}
			}
			if tc.name == "shell" || tc.name == "write" {
				m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			}
			// The editor remains usable after dismissing the shell output.
			for _, r := range "ix" {
				m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			for _, r := range "u:q!" {
				m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd != nil {
				if _, quit := cmd().(tea.QuitMsg); quit {
					t.Fatal("editor exit quit Misa")
				}
			}
			if m.activePane() != pane || pane.misa.viEditor != nil || pane.misa.screen != screenShell {
				t.Fatal("editor did not return to the same shell pane")
			}
		})
	}
}

func TestEditorShellNamedCommandsThroughMux(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	bin := filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"ls": "printf 'fake-listing\\n'", "rm": "printf 'fake-removed:%s\\n' \"$1\""} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/bin")
	m := newRoot(t, 80, 24)
	m.addMuxPane()
	pane := m.activePane()
	pane.misa.screen = screenShell
	pane.misa.enterEditor("")
	for _, tc := range []struct{ command, want string }{{":!ls", "fake-listing"}, {":!rm FILE", "fake-removed:FILE"}} {
		for _, r := range tc.command {
			m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatalf("%s did not return a command", tc.command)
		}
		msg := cmd()
		if _, ok := msg.(paneMsg); !ok {
			t.Fatalf("%s result not routed: %T", tc.command, msg)
		}
		m.Update(msg)
		if pane.misa.viEditor == nil || m.activePane() != pane {
			t.Fatal("editor pane lost")
		}
		if view := ansi.Strip(pane.misa.viEditor.View()); !strings.Contains(view, tc.want) {
			t.Errorf("%s: missing %q in %q", tc.command, tc.want, view)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
}
