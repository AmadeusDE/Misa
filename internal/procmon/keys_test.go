package procmon

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHtopFilterTypesSpace(t *testing.T) {
	h := &HtopViewer{mode: htopFilter}
	for _, k := range []tea.KeyPressMsg{
		{Code: 'a', Text: "a"},
		{Code: tea.KeySpace, Text: " "},
		{Code: 'b', Text: "b"},
	} {
		h.handleKey(k)
	}
	if h.filterBuf != "a b" {
		t.Errorf("filter = %q, want %q", h.filterBuf, "a b")
	}
}
