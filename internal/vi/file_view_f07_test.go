package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFileF07MixedCRVisible(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed")
	if err := os.WriteFile(path, []byte("a\r\n\tb\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.Resize(80, 24)
	if !reflect.DeepEqual(e.lines, []string{"a\r", "\tb"}) {
		t.Fatalf("lines = %#v", e.lines)
	}
	for _, keys := range []string{"", "l", "v", "<Esc>j0"} {
		for _, key := range replayKeys(t, keys) {
			e.Update(key)
		}
		view := e.View()
		if strings.Contains(view, "\r") || !strings.Contains(view, "^M") {
			t.Errorf("keys %q: CR not visibly rendered: %q", keys, view)
		}
	}
}
