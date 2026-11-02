//go:build linux || darwin || freebsd || openbsd || dragonfly || netbsd

package term

import (
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// winSize is the four values a program reads back with TIOCGWINSZ.
type winSize struct{ rows, cols, xpixel, ypixel uint16 }

func ioctlWinsize(f *os.File) (winSize, error) {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return winSize{}, err
	}
	return winSize{ws.Row, ws.Col, ws.Xpixel, ws.Ypixel}, nil
}

func mustIoctlWinsize(t *testing.T, f *os.File) winSize {
	t.Helper()
	ws, err := ioctlWinsize(f)
	if err != nil {
		t.Fatalf("TIOCGWINSZ: %v", err)
	}
	return ws
}

// setWinsize must carry the pixel dimensions to the kernel, on both ends of the
// pty pair, and saturate rather than wrap when a grid is very wide.
func TestSetWinsizePixels(t *testing.T) {
	ptm, pts, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer ptm.Close()
	defer pts.Close()

	cases := []struct {
		name string
		ws   *pty.Winsize
		want winSize
	}{
		{"known cell", ptySize(24, 80, 9, 20), winSize{24, 80, 720, 480}},
		{"unknown cell", ptySize(24, 80, 0, 0), winSize{24, 80, 0, 0}},
		{"half known cell", ptySize(24, 80, 9, 0), winSize{24, 80, 0, 0}},
		{"saturated", ptySize(24, 100, 4096, 4096), winSize{24, 100, 65535, 65535}},
	}
	for _, c := range cases {
		if err := setWinsize(ptm, c.ws); err != nil {
			t.Fatalf("%s: setWinsize: %v", c.name, err)
		}
		for _, f := range []*os.File{ptm, pts} {
			if got := mustIoctlWinsize(t, f); got != c.want {
				t.Errorf("%s: child winsize = %+v, want %+v", c.name, got, c.want)
			}
		}
	}
}

// The values that matter are the ones a child reads from its own tty: a
// StartFunc program must see the pixel size at start, after a resize, and
// after a cell-size change that leaves the grid alone.
func TestChildSeesCellPixels(t *testing.T) {
	tm := New(24, 80)
	tm.SetCellPixels(9, 20)
	defer tm.Close()

	type result struct {
		ws  winSize
		err error
	}
	sizes := make(chan result, 3)
	proceed := make(chan struct{}, 1)
	if err := tm.StartFunc(func(tty *os.File) int {
		for range 3 {
			ws, err := ioctlWinsize(tty)
			sizes <- result{ws, err}
			select {
			case <-proceed:
			case <-time.After(10 * time.Second):
				return 1
			}
		}
		return 0
	}); err != nil {
		t.Fatal(err)
	}

	next := func(step string, want winSize) {
		t.Helper()
		select {
		case r := <-sizes:
			if r.err != nil {
				t.Fatalf("%s: child TIOCGWINSZ: %v", step, r.err)
			}
			if r.ws != want {
				t.Fatalf("%s: child winsize = %+v, want %+v", step, r.ws, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: child sent no winsize", step)
		}
	}

	next("start", winSize{24, 80, 720, 480})
	tm.Resize(30, 100)
	proceed <- struct{}{}
	next("resize", winSize{30, 100, 900, 600})
	tm.SetCellPixels(10, 22)
	proceed <- struct{}{}
	next("cell change", winSize{30, 100, 1000, 660})
}
