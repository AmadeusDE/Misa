package vi

import "testing"

func TestSearchCommandsRestoreHighlights(t *testing.T) {
	e := &Editor{lines: []string{"one one"}, searchPat: "one", searchDir: 1, searchHighlightSuppressed: true}
	e.searchCounted(1, false)
	if e.searchHighlightSuppressed {
		t.Fatal("n did not restore highlights")
	}
	e.searchHighlightSuppressed = true
	e.complexMotion("N", 1, false)
	if e.searchHighlightSuppressed {
		t.Fatal("operator N did not restore highlights")
	}
	e.searchHighlightSuppressed = true
	e.searchNext()
	if e.searchHighlightSuppressed {
		t.Fatal("visual n did not restore highlights")
	}
	e.searchHighlightSuppressed = true
	e.usePattern("one", false)
	if e.searchHighlightSuppressed {
		t.Fatal("ex pattern did not restore highlights")
	}
}
