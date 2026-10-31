package vi

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestT02StandaloneRangeSaveAsQuit(t *testing.T) {
	e, p := f07Editor(t, "one\ntwo\nthree\n")
	m := &standaloneModel{editor: e}
	out := filepath.Join(filepath.Dir(p), "part")
	for _, key := range replayKeys(t, ":2,3w "+out+"<CR>iX<Esc>:saveas saved<CR>:q<CR>") {
		_, cmd := m.Update(key)
		if e.Quit && (cmd == nil || func() bool { _, ok := cmd().(tea.QuitMsg); return !ok }()) {
			t.Error("standalone exit did not send quit")
		}
	}
	if got := f07Read(t, out); got != "two\nthree\n" {
		t.Errorf("range=%q", got)
	}
	if got := f07Read(t, filepath.Join(filepath.Dir(p), "saved")); got != "Xone\ntwo\nthree\n" {
		t.Errorf("saveas=%q", got)
	}
	if !e.Quit {
		t.Error("did not quit")
	}
}
