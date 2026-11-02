package gui

import (
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/imgview"
	"github.com/AmadeusDE/Misa/internal/term"
)

// This is re-executed under the terminal's PTY; never start another test runner
// or construct a fake viewer/render stream in the parent.
func TestImgviewPTYHelper(t *testing.T) {
	if os.Getenv("MISA_IMGVIEW_TEST_CHILD") != "1" {
		return
	}
	cfg := config.DefaultImageViewer()
	cfg.Fit = "contain"
	v, err := imgview.NewConfigured([]string{os.Getenv("MISA_IMGVIEW_TEST_FIRST"), os.Getenv("MISA_IMGVIEW_TEST_SECOND")}, false, "auto", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := imgview.Run(v); err != nil {
		t.Fatal(err)
	}
}

func imgviewFixture(t *testing.T, path string, left, right color.RGBA) {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			c := left
			if x >= 32 {
				c = right
			}
			im.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, im); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func imgviewTerminal(t *testing.T, graphics bool, graphicsYAML ...string) *term.Terminal {
	t.Helper()
	if runtime.GOOS == "android" || runtime.GOOS == "windows" {
		t.Skip("PTY child unsupported on this platform")
	}
	dir := t.TempDir()
	first := filepath.Join(dir, "a.png")
	second := filepath.Join(dir, "b.png")
	imgviewFixture(t, first, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255})
	imgviewFixture(t, second, color.RGBA{0, 255, 0, 255}, color.RGBA{255, 255, 0, 255})
	if os.Getenv("MISA_IMGVIEW_ANIM_TEST") == "1" {
		first = filepath.Join(dir, "a.gif")
		palette := color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}}
		red := image.NewPaletted(image.Rect(0, 0, 64, 32), palette)
		green := image.NewPaletted(red.Rect, palette)
		for i := range green.Pix {
			green.Pix[i] = 1
		}
		f, err := os.Create(first)
		if err != nil {
			t.Fatal(err)
		}
		if err := gif.EncodeAll(f, &gif.GIF{Image: []*image.Paletted{red, green}, Delay: []int{60, 60}, LoopCount: 0}); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		second = first
	}
	t.Setenv("MISA_IMGVIEW_TEST_CHILD", "1")
	t.Setenv("MISA_IMGVIEW_TEST_FIRST", first)
	t.Setenv("MISA_IMGVIEW_TEST_SECOND", second)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("COLORTERM", "truecolor")
	if len(graphicsYAML) != 0 {
		configDir := filepath.Join(dir, "misa")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "graphics.yaml"), []byte(graphicsYAML[0]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tm := term.New(12, 40)
	tm.SetGraphicsLimits(config.LoadGraphics().Kitty.Limits())
	tm.SetGraphicsEnabled(graphics)
	tm.SetFocused(true)
	tm.SetCellPixels(10, 20)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := tm.StartArgv(dir, exe, "-test.run=^TestImgviewPTYHelper$"); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	t.Cleanup(tm.Close)
	return tm
}

func imgviewWait(t *testing.T, tm *term.Terminal, description string, pred func(*term.Snapshot) bool) term.Snapshot {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var s term.Snapshot
	for time.Now().Before(deadline) {
		tm.Snapshot(&s)
		if pred(&s) {
			return s
		}
		select {
		case <-tm.Exited():
			t.Fatalf("child exited (%d) waiting for %s; snapshot %+v", tm.ExitCode(), description, s)
		case <-tm.Dirty():
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatalf("timeout waiting for %s; alt=%v placements=%d generation=%d (probe reply and Kitty APC are not exposed by terminal snapshots, so cannot distinguish a missing probe reply or APC from a rejected placement)", description, tm.InAltScreen(), len(s.Graphics), s.GraphicsGeneration)
	return s
}

func imgviewRender(t *testing.T, s *term.Snapshot) (*renderer, *canvas) {
	t.Helper()
	r := testRenderer(t)
	r.fonts.cellW = 10
	r.fonts.cellH = 20
	c := canvasFor(r, s.Rows, s.Cols)
	r.draw(c, s, cursorState{}, &frame{})
	return r, c
}

func imgviewPixels(t *testing.T, s *term.Snapshot, left, right rgb, dump bool) {
	t.Helper()
	if len(s.Graphics) != 1 {
		t.Fatalf("placements = %d, want 1", len(s.Graphics))
	}
	p := s.Graphics[0]
	r, c := imgviewRender(t, s)
	rect := r.placementRect(&p)
	if rect.Empty() {
		t.Fatal("empty placement")
	}
	for _, tc := range []struct {
		x    int
		want rgb
	}{{rect.Min.X + rect.Dx()/4, left}, {rect.Min.X + 3*rect.Dx()/4, right}} {
		if got := colourAt(c, tc.x, rect.Min.Y+rect.Dy()/2); got != tc.want {
			t.Errorf("pixel (%d,%d) = %v, want %v; placement %v", tc.x, rect.Min.Y+rect.Dy()/2, got, tc.want, rect)
		}
	}
	for y := max(0, p.Clip.Min.Y); y < min(s.Rows, p.Clip.Max.Y); y++ {
		for x := max(0, p.Clip.Min.X); x < min(s.Cols, p.Clip.Max.X); x++ {
			ch := s.Cells[y*s.Cols+x].Ch
			if ch == '▀' || ch == '▄' || ch == '█' {
				t.Fatalf("halfblock %q under placement at %d,%d", ch, x, y)
			}
		}
	}
	// These coordinates are independent of the reported placement: contain-fit
	// puts the 2:1 image at (0,10)..(400,210), then after resize at
	// (10,0)..(290,140), in viewport pixels (excluding renderer padding).
	var lx, rx, y int
	switch {
	case s.Rows == 12 && s.Cols == 40:
		lx, rx, y = 100, 300, 110
	case s.Rows == 8 && s.Cols == 30:
		lx, rx, y = 80, 220, 70
	default:
		t.Fatalf("unexpected geometry %dx%d", s.Cols, s.Rows)
	}
	for _, tc := range []struct {
		x    int
		want rgb
	}{{lx, left}, {rx, right}} {
		if got := colourAt(c, r.pad+tc.x, r.pad+y); got != tc.want {
			t.Errorf("viewport pixel (%d,%d) = %v, want %v", tc.x, y, got, tc.want)
		}
	}
	if dump {
		if path := os.Getenv("MISA_IMGSHOT"); path != "" {
			im := image.NewRGBA(image.Rect(0, 0, c.w, c.h))
			for y := 0; y < c.h; y++ {
				for x := 0; x < c.w; x++ {
					v, a := pixelAt(c, x, y)
					im.SetRGBA(x, y, color.RGBA{v.r, v.g, v.b, a})
				}
			}
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(f, im); err != nil {
				f.Close()
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestImgviewRealGUIPixels(t *testing.T) {
	tm := imgviewTerminal(t, true)
	s := imgviewWait(t, tm, "initial graphics", func(s *term.Snapshot) bool { return tm.InAltScreen() && len(s.Graphics) == 1 })
	imgviewPixels(t, &s, rgb{255, 0, 0}, rgb{0, 0, 255}, true)
	old := s.Graphics[0].CellRect
	gen := s.GraphicsGeneration
	tm.Resize(8, 30)
	s = imgviewWait(t, tm, "resized replacement", func(s *term.Snapshot) bool {
		return len(s.Graphics) == 1 && s.GraphicsGeneration > gen && s.Graphics[0].CellRect != old
	})
	imgviewPixels(t, &s, rgb{255, 0, 0}, rgb{0, 0, 255}, false)
	gen = s.GraphicsGeneration
	tm.SendBytes([]byte("n"))
	s = imgviewWait(t, tm, "next image", func(s *term.Snapshot) bool {
		if len(s.Graphics) != 1 || s.GraphicsGeneration <= gen {
			return false
		}
		r, c := imgviewRender(t, s)
		return colourAt(c, r.pad+80, r.pad+70) == (rgb{0, 255, 0})
	})
	imgviewPixels(t, &s, rgb{0, 255, 0}, rgb{255, 255, 0}, false)
	gen = s.GraphicsGeneration
	tm.SendBytes([]byte("p"))
	s = imgviewWait(t, tm, "previous image", func(s *term.Snapshot) bool {
		if len(s.Graphics) != 1 || s.GraphicsGeneration <= gen {
			return false
		}
		r, c := imgviewRender(t, s)
		return colourAt(c, r.pad+80, r.pad+70) == (rgb{255, 0, 0})
	})
	imgviewPixels(t, &s, rgb{255, 0, 0}, rgb{0, 0, 255}, false)
	tm.SendBytes([]byte("q"))
	select {
	case <-tm.Exited():
	case <-time.After(8 * time.Second):
		t.Fatal("q did not exit")
	}
	tm.Snapshot(&s)
	if tm.ExitCode() != 0 || tm.InAltScreen() || len(s.Graphics) != 0 {
		t.Fatalf("exit=%d alt=%v placements=%d", tm.ExitCode(), tm.InAltScreen(), len(s.Graphics))
	}
}

func TestImgviewGraphicsDisabledFallsBack(t *testing.T) {
	tm := imgviewTerminal(t, false)
	s := imgviewWait(t, tm, "halfblock fallback", func(s *term.Snapshot) bool {
		for _, c := range s.Cells {
			if c.Ch == '▀' || c.Ch == '▄' {
				return true
			}
		}
		return false
	})
	if len(s.Graphics) != 0 {
		t.Fatalf("disabled terminal has %d placements", len(s.Graphics))
	}
	tm.SendBytes([]byte("q"))
	select {
	case <-tm.Exited():
	case <-time.After(8 * time.Second):
		t.Fatal("q did not exit")
	}
}
