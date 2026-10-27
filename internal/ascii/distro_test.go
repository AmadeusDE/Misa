package ascii

import (
	"strings"
	"testing"
)

func TestAndroidAndTermuxArt(t *testing.T) {
	android := LoadDistroArt("android", "")
	if android == nil || !strings.EqualFold(android.Name, "android") {
		t.Fatalf("android art: %+v", android)
	}
	if len(android.Lines) == 0 {
		t.Fatal("android art is empty")
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
