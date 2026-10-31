package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestVimPatternsStandaloneEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample")
	if err := os.WriteFile(path, []byte("cat 12 ünï 345\nfoo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	m := &standaloneModel{editor: e}
	send := func(s string) {
		for _, k := range replayKeys(t, s) {
			if _, cmd := m.Update(k); cmd != nil {
				if _, ok := cmd().(tea.QuitMsg); ok {
					t.Fatal("pattern command quit standalone")
				}
			}
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 15})
	send("/\\v(ünï|345)<CR>")
	if e.row != 0 || e.col != 7 {
		t.Fatalf("search cursor (%d,%d)", e.row, e.col)
	}
	send(":s/\\d\\+/N/g<CR>")
	if want := []string{"cat N ünï N", "foo"}; !reflect.DeepEqual(e.lines, want) {
		t.Fatalf("substitute: %q want %q", e.lines, want)
	}
	send("/\\zs<CR>")
	if !strings.Contains(e.statusMsg, `Unsupported pattern construct: \zs`) || e.mode != ModeNormal {
		t.Fatalf("status %q mode %v", e.statusMsg, e.mode)
	}
	send(":g/\\<lt>foo\\>/d<CR>")
	if want := []string{"cat N ünï N"}; !reflect.DeepEqual(e.lines, want) {
		t.Fatalf("global: %q want %q", e.lines, want)
	}
	send("u")
	var cmd tea.Cmd
	for _, k := range replayKeys(t, ":q!<CR>") {
		_, cmd = m.Update(k)
	}
	if !e.Quit || cmd == nil {
		t.Fatal(":q! did not quit")
	}
}
