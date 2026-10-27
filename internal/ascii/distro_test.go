package ascii

import (
	"strings"
	"testing"
)

func TestAndroidAndTermuxArt(t *testing.T) {
	android := LoadDistroArt("android", "")
	if android == nil || android.Source != "pfetch" || !strings.EqualFold(android.Name, "android") {
		t.Fatalf("android art: %+v", android)
	}
	if !strings.Contains(strings.Join(android.Lines, "\n"), "';,.-----.,;'") {
		t.Fatalf("android art is not the pfetch bugdroid: %q", android.Lines)
	}
	if named := LoadDistroArt("Android", ""); named == nil || named.Source != android.Source || named.Name != android.Name {
		t.Fatalf("Android query = %+v, want pfetch android", named)
	}
	termux := LoadDistroArt("termux", "")
	if termux == nil || !strings.EqualFold(termux.Name, "termux") {
		t.Fatalf("termux art: %+v", termux)
	}
	if len(termux.Lines) == 0 {
		t.Fatal("termux art is empty")
	}
	names := ListDistroNames()
	var sawAndroid, sawTermux bool
	for _, n := range names {
		if n == "pfetch/android" {
			sawAndroid = true
		}
		if n == "pfetch/termux" {
			sawTermux = true
		}
	}
	if !sawAndroid || !sawTermux {
		t.Fatalf("listed art missing android/termux: android=%v termux=%v", sawAndroid, sawTermux)
	}
}
