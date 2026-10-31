package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSubstituteConfirmationStandaloneEntry(t *testing.T) {
	for _, cancel := range []string{"q", "<Esc>"} {
		t.Run(cancel, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sample")
			if err := os.WriteFile(path, []byte("aa aa aa\n"), 0600); err != nil {
				t.Fatal(err)
			}
			e := New(path, dir)
			m := &standaloneModel{editor: e}
			send := func(s string) {
				for _, k := range replayKeys(t, s) {
					_, cmd := m.Update(k)
					if cmd != nil {
						if _, ok := cmd().(tea.QuitMsg); ok {
							t.Fatal("confirmation quit standalone")
						}
					}
				}
			}
			send("yl:%s/aa/x/gc<CR>")
			m.Update(tea.WindowSizeMsg{Width: 60, Height: 15})
			send("yy" + cancel)
			if e.Quit || !reflect.DeepEqual(e.lines, []string{"x x aa"}) {
				t.Fatalf("after confirmation: lines=%q quit=%v", e.lines, e.Quit)
			}
			reg, line := e.Register()
			if !reflect.DeepEqual(reg, []string{"a"}) || line {
				t.Fatalf("register=%q linewise=%v", reg, line)
			}
			send("u")
			if !reflect.DeepEqual(e.lines, []string{"aa aa aa"}) || e.modified {
				t.Fatalf("undo: lines=%q modified=%v", e.lines, e.modified)
			}
			var cmd tea.Cmd
			for _, k := range replayKeys(t, ":q<CR>") {
				_, cmd = m.Update(k)
			}
			if !e.Quit || cmd == nil {
				t.Fatal("clean :q did not quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("missing quit message")
			}
		})
	}
}
