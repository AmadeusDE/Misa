//go:build linux || darwin || freebsd || openbsd || dragonfly || netbsd

package imgview

import (
	"image"
	"os"
	"testing"

	"github.com/creack/pty"
)

// A host reports its cell size through the PTY, which is how the Misa GUI
// hands its font metrics to a child.
func TestCellPixelsFromTTY(t *testing.T) {
	ptm, pts, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer ptm.Close()
	defer pts.Close()

	cases := []struct {
		name string
		ws   *pty.Winsize
		want image.Point
	}{
		{"exact", &pty.Winsize{Rows: 24, Cols: 80, X: 720, Y: 480}, image.Pt(9, 20)},
		{"rounded", &pty.Winsize{Rows: 24, Cols: 80, X: 1583, Y: 470}, image.Pt(20, 20)},
		{"no pixels", &pty.Winsize{Rows: 24, Cols: 80}, image.Point{}},
		{"no grid", &pty.Winsize{X: 720, Y: 480}, image.Point{}},
	}
	for _, c := range cases {
		if err := pty.Setsize(ptm, c.ws); err != nil {
			t.Fatal(err)
		}
		if got := cellPixelsOf(pts); got != c.want {
			t.Errorf("%s: cellPixelsOf = %v, want %v", c.name, got, c.want)
		}
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	if got := cellPixelsOf(devNull); got != (image.Point{}) {
		t.Errorf("non-terminal reported %v", got)
	}
}
