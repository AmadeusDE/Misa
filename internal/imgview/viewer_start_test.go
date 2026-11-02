package imgview

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestViewerStartPath(t *testing.T) {
	paths := []string{"a.png", filepath.Join("folder", "b.png"), "c.png"}
	v, err := New(paths, false, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if err := v.SetStartPath(filepath.Join("folder", "sub", "..", "b.png")); err != nil {
		t.Fatal(err)
	}
	if v.Index() != 1 || !reflect.DeepEqual(v.Paths(), paths) {
		t.Fatalf("index=%d paths=%v", v.Index(), v.Paths())
	}
	if v.inFlight || v.anim != nil {
		t.Fatal("setting the initial item started decode work")
	}
	for _, path := range []string{"", "missing.png"} {
		if err := v.SetStartPath(path); err == nil {
			t.Fatalf("accepted absent start file %q", path)
		}
		if v.Index() != 1 {
			t.Fatal("invalid start file changed selection")
		}
	}
}
