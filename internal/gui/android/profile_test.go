package android

import (
	"runtime"
	"testing"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imgview"
)

func TestImageViewerPlatformBudgets(t *testing.T) {
	c := config.DefaultImageViewer()
	want := [4]int{30, 10, 10, 2}
	if runtime.GOOS == "android" {
		want = [4]int{20, 8, 8, 1}
	}
	got := [4]int{c.AnimationFPS, c.HalfblockAnimationFPS, c.PreviewAnimationFPS, imgview.DefaultWorkers()}
	if got != want {
		t.Fatalf("pixel/halfblock/preview fps and decode workers = %v, want %v", got, want)
	}
}
