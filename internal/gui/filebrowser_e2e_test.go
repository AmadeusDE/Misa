package gui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
)

func filebrowserText(s *term.Snapshot) string {
	var b strings.Builder
	for y := 0; y < s.Rows; y++ {
		for x := 0; x < s.Cols; x++ {
			c := s.Cells[y*s.Cols+x]
			if c.Ch != 0 {
				b.WriteRune(c.Ch)
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func filebrowserWait(t *testing.T, tm *term.Terminal, what string, pred func(*term.Snapshot) bool) term.Snapshot {
	t.Helper()
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	var s term.Snapshot
	for {
		tm.Snapshot(&s)
		if pred(&s) {
			return s
		}
		select {
		case <-tm.Exited():
			t.Fatalf("child exited %d waiting for %s; placements=%d\n%s", tm.ExitCode(), what, len(s.Graphics), filebrowserText(&s))
		case <-deadline.C:
			t.Fatalf("timeout waiting for %s; placements=%d generation=%d\n%s", what, len(s.Graphics), s.GraphicsGeneration, filebrowserText(&s))
		case <-tm.Dirty():
		case <-time.After(30 * time.Millisecond):
		}
	}
}

func filebrowserDump(t *testing.T, s *term.Snapshot) {
	t.Helper()
	path := os.Getenv("MISA_FILEBROWSER_IMGSHOT")
	if path == "" {
		return
	}
	_, c := imgviewRender(t, s)
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

func TestFilebrowserRealGUIPixels(t *testing.T) {
	if runtime.GOOS == "android" || runtime.GOOS == "windows" {
		t.Skip("PTY child unsupported on this platform")
	}
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	data := filepath.Join(root, "images")
	home := filepath.Join(root, "home")
	for _, d := range []string{binDir, data, home} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(binDir, "misa")
	cmd := exec.Command("go", "build", "-o", exe, "../..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build misa: %v\n%s", err, out)
	}
	for name, content := range map[string]string{"config.yaml": "{}\n", "imageviewer.yaml": "fit: contain\npreview: false\n"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	imgviewFixture(t, filepath.Join(data, "a.png"), color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255})
	imgviewFixture(t, filepath.Join(data, "b.png"), color.RGBA{0, 255, 0, 255}, color.RGBA{255, 255, 0, 255})
	if err := os.WriteFile(filepath.Join(data, "notes.txt"), []byte("notes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("TERM", "xterm-256color")
	tm := term.New(40, 100)
	tm.SetGraphicsLimits(config.DefaultGraphics().Kitty.Limits())
	tm.SetGraphicsEnabled(true)
	tm.SetFocused(true)
	tm.SetCellPixels(10, 20)
	if err := tm.StartArgv(data, exe); err != nil {
		t.Fatal(err)
	}
	tm.StartReader()
	t.Cleanup(tm.Close)
	filebrowserWait(t, tm, "root menu", func(s *term.Snapshot) bool { return strings.Contains(filebrowserText(s), "Browse files") })
	// Root filtering is disabled. Walk the real menu, synchronizing on each
	// selected row rather than guessing the host's package-manager count.
	selected := func(s *term.Snapshot) string {
		for _, line := range strings.Split(filebrowserText(s), "\n") {
			if strings.Contains(line, "> ") {
				return strings.TrimSpace(line)
			}
		}
		return ""
	}
	var menu term.Snapshot
	tm.Snapshot(&menu)
	for i := 0; i < 50 && !strings.Contains(selected(&menu), "> Browse files"); i++ {
		prev := selected(&menu)
		tm.SendBytes([]byte("j"))
		menu = filebrowserWait(t, tm, "next root menu selection", func(s *term.Snapshot) bool { return selected(s) != "" && selected(s) != prev })
	}
	if !strings.Contains(selected(&menu), "> Browse files") {
		t.Fatalf("Browse files not found: %s", selected(&menu))
	}
	tm.SendBytes([]byte("\r"))
	filebrowserWait(t, tm, "file browser", func(s *term.Snapshot) bool {
		text := filebrowserText(s)
		return strings.Contains(text, "a.png") && strings.Contains(text, "b.png") && strings.Contains(text, "notes.txt")
	})
	tm.SendBytes([]byte("jj"))
	filebrowserWait(t, tm, "second image selected", func(s *term.Snapshot) bool {
		for _, line := range strings.Split(filebrowserText(s), "\n") {
			if strings.Contains(line, "> b.png") {
				return true
			}
		}
		return false
	})
	tm.SendBytes([]byte("\r"))
	waitColor := func(what string, want rgb) term.Snapshot {
		return filebrowserWait(t, tm, what, func(s *term.Snapshot) bool {
			if len(s.Graphics) != 1 {
				return false
			}
			r, c := imgviewRender(t, s)
			rect := r.placementRect(&s.Graphics[0])
			return !rect.Empty() && colourAt(c, rect.Min.X+rect.Dx()/4, rect.Min.Y+rect.Dy()/2) == want
		})
	}
	check := func(s *term.Snapshot, left, right rgb) {
		t.Helper()
		if len(s.Graphics) != 1 {
			t.Fatalf("placements=%d", len(s.Graphics))
		}
		r, c := imgviewRender(t, s)
		rect := r.placementRect(&s.Graphics[0])
		for _, p := range []struct {
			x    int
			want rgb
		}{{rect.Min.X + rect.Dx()/4, left}, {rect.Min.X + 3*rect.Dx()/4, right}} {
			if got := colourAt(c, p.x, rect.Min.Y+rect.Dy()/2); got != p.want {
				t.Fatalf("pixel=%v want=%v rect=%v", got, p.want, rect)
			}
		}
	}
	s := waitColor("second PNG pixels", rgb{0, 255, 0})
	check(&s, rgb{0, 255, 0}, rgb{255, 255, 0})
	filebrowserDump(t, &s)
	tm.SendBytes([]byte("p"))
	s = waitColor("previous first PNG pixels", rgb{255, 0, 0})
	check(&s, rgb{255, 0, 0}, rgb{0, 0, 255})
	tm.SendBytes([]byte("n"))
	s = waitColor("next second PNG pixels", rgb{0, 255, 0})
	check(&s, rgb{0, 255, 0}, rgb{255, 255, 0})
	// The browser must refresh its directory, not merely repaint a cached list.
	if err := os.WriteFile(filepath.Join(data, "newly-added.txt"), []byte("added while viewing\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tm.SendBytes([]byte("q"))
	filebrowserWait(t, tm, "browser restored without placements", func(s *term.Snapshot) bool {
		text := filebrowserText(s)
		return len(s.Graphics) == 0 && strings.Contains(text, "a.png") && strings.Contains(text, "b.png") && strings.Contains(text, "notes.txt") && strings.Contains(text, "newly-added.txt")
	})
	tm.SendBytes([]byte("\r"))
	s = waitColor("reopened selected second PNG", rgb{0, 255, 0})
	check(&s, rgb{0, 255, 0}, rgb{255, 255, 0})
}
