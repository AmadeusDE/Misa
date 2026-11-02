//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package imgview

import (
	"image"
	"os"

	"golang.org/x/sys/unix"
)

// MeasureCellPixels asks the controlling terminal for its cell size, which is
// how a host such as the Misa GUI reports the font metrics of its child. The
// zero point means the terminal does not know.
func MeasureCellPixels() image.Point { return cellPixelsOf(os.Stdout, os.Stderr, os.Stdin) }

// cellPixelsOf is the first file that reports a usable size: a redirected
// stdout still leaves the terminal on stderr or stdin.
func cellPixelsOf(files ...*os.File) image.Point {
	for _, f := range files {
		ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
		if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
			continue
		}
		return image.Pt(roundDiv(int(ws.Xpixel), int(ws.Col)), roundDiv(int(ws.Ypixel), int(ws.Row)))
	}
	return image.Point{}
}

// roundDiv divides with rounding, since a terminal may report a pixel width
// that is not an exact multiple of its column count.
func roundDiv(n, d int) int { return (n + d/2) / d }
