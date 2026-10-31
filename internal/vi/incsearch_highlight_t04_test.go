package vi

import "testing"

func TestIncSearchHighlightState(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"é x éé", "other"}
	e.Resize(80, 24)
	send := func(s string) {
		for _, k := range replayKeys(t, s) {
			e.Update(k)
		}
	}
	send(":set incsearch<CR>/éé")
	pat, row, start, end, active, matched := e.previewHighlight()
	if pat != "éé" || row != 0 || start != 5 || end != 9 || !active || !matched {
		t.Fatalf("preview: %q %d %d:%d active=%v matched=%v", pat, row, start, end, active, matched)
	}
	send("z")
	_, _, _, _, active, matched = e.previewHighlight()
	if !active || matched {
		t.Fatalf("missing preview active=%v matched=%v", active, matched)
	}
	send("<Esc>")
	_, _, _, _, active, _ = e.previewHighlight()
	if active {
		t.Fatal("canceled preview remains active")
	}
}
