package vi

import "testing"

func TestPreviewViewportHighlights(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"old foo foo"}
	e.Resize(80, 24)
	e.searchPat = "old"
	e.hlSearch = true
	e.searchHighlightSuppressed = true
	e.incSearch = true
	send := func(s string) {
		for _, k := range replayKeys(t, s) {
			e.Update(k)
		}
	}
	send("/foo")
	l := e.rowLayout(0)
	all, current := e.highlightLineMarks(0, l)
	if all == nil || !all(4) || !all(8) || all(0) || current == nil || !current(4) || current(8) {
		t.Fatal("tentative all/current highlights incorrect")
	}
	e.hlSearch = false
	all, current = e.highlightLineMarks(0, l)
	if all != nil || current == nil || !current(4) || current(8) {
		t.Fatal("current preview must highlight even with hlsearch off")
	}
	send("\\(")
	all, current = e.highlightLineMarks(0, l)
	if all != nil || current != nil {
		t.Fatal("invalid preview highlights")
	}
	send("<Esc>")
	e.hlSearch = true
	e.searchHighlightSuppressed = false
	all, current = e.highlightLineMarks(0, l)
	if all == nil || !all(0) || all(4) || current != nil {
		t.Fatal("committed highlight not restored")
	}
}
