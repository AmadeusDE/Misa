package android

import (
	"encoding/base64"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
	"unsafe"
)

const (
	renderHelperEnv = "MISA_GUI_RENDER_HELPER"
	renderReadyEnv  = "MISA_GUI_RENDER_READY"
	renderEmitEnv   = "MISA_GUI_RENDER_EMITS"
	renderTintEnv   = "MISA_GUI_RENDER_TINT"
)

func TestMain(m *testing.M) {
	if runtime.GOOS != "android" && os.Getenv(renderHelperEnv) != "" {
		os.Exit(renderHelper())
	}
	os.Exit(m.Run())
}

// renderHelper is the re-executed PTY child: it paints one 4x4 image of four
// quadrants, records that single write under its own pid, then blocks so later
// frames can only come from the terminal's store.
func renderHelper() int {
	tl := [4]uint8{255, 0, 0, 255}
	if os.Getenv(renderTintEnv) == "yellow" {
		tl = [4]uint8{255, 255, 0, 255}
	}
	// Each source pixel is doubled in both axes, so every quadrant of the
	// 4x4 image fills one of the 2x2 cells it covers.
	var src []byte
	for _, row := range [][2][4]uint8{
		{tl, {0, 255, 0, 255}},
		{{0, 0, 255, 255}, {255, 0, 0, 128}},
	} {
		for v := 0; v < 2; v++ {
			for _, px := range row {
				src = append(src, px[:]...)
				src = append(src, px[:]...)
			}
		}
	}
	pid := strconv.Itoa(os.Getpid())
	if dir := os.Getenv(renderEmitEnv); dir != "" {
		if f, err := os.OpenFile(filepath.Join(dir, pid+".emits"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600); err == nil {
			f.Write([]byte("x"))
			f.Close()
		}
	}
	seq := "\x1b[?25l\x1b[H\x1b_Ga=T,f=32,i=1,s=4,v=4,c=2,r=2;" +
		base64.StdEncoding.EncodeToString(src) + "\x1b\\"
	if _, err := os.Stdout.Write([]byte(seq)); err != nil {
		return 1
	}
	if dir := os.Getenv(renderReadyEnv); dir != "" {
		os.WriteFile(filepath.Join(dir, pid+".ready"), []byte("painted"), 0o600)
	}
	var buf [256]byte
	for {
		if _, err := os.Stdin.Read(buf[:]); err != nil {
			return 0
		}
	}
}

func renderTestSkip(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "android" {
		t.Skip("the render helper needs a host Linux PTY")
	}
}

// writeRenderConfig pins the palette: a #404040 background at 0.5 opacity is
// premultiplied {32, 32, 32, 128} wherever the terminal keeps its default.
func writeRenderConfig(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, "misa")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := []byte("font_size: 10\npadding: 4\nbackground: \"#404040\"\nforeground: \"#ffffff\"\ncursor: \"#ffffff\"\nopacity: 0.5\n")
	if err := os.WriteFile(filepath.Join(dir, "gui.yaml"), cfg, 0o600); err != nil {
		t.Fatal(err)
	}
}

func renderExe(t *testing.T) string {
	t.Helper()
	if exe := os.Args[0]; filepath.IsAbs(exe) {
		return exe
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// renderSurface is the window the rig presents into. It counts whole-frame
// presents (a full-buffer damage), so a partial repaint such as the cursor row
// of a focus change cannot be mistaken for a reconstruction.
type renderSurface struct {
	mu       sync.Mutex
	pix      []byte
	stride   int
	w, h     int
	presents int
	full     int
}

func newRenderSurface(w, h int) *renderSurface {
	stride := w * 4
	return &renderSurface{pix: make([]byte, stride*h), stride: stride, w: w, h: h}
}

func (s *renderSurface) Present(f Frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.WindowW != s.w || f.WindowH != s.h || f.WindowW <= 0 || f.WindowH <= 0 {
		return
	}
	Blit(s.pix, s.stride, s.w, s.h, f.Src, f.SrcStride, f.SrcW, f.SrcH, f.OriginX, f.OriginY, f.Bounds, f.Swap)
	s.presents++
	if f.Bounds == image.Rect(f.OriginX, f.OriginY, f.OriginX+f.SrcW, f.OriginY+f.SrcH) {
		s.full++
	}
}

func (s *renderSurface) Release() {}

var _ Surface = (*renderSurface)(nil)

type renderRig struct {
	t     *testing.T
	m     *Manager
	id    int
	surf  *renderSurface
	state string
	w, h  int
}

// startRenderRig runs gui.RunSession on a Manager-owned session. PATH and
// PREFIX point at empty dirs so fonts fall back to the embedded Go Mono;
// HOME and XDG_CONFIG_HOME make the palette config hermetic.
func startRenderRig(t *testing.T, w, h int) *renderRig {
	t.Helper()
	renderTestSkip(t)
	home := t.TempDir()
	emptyBin := t.TempDir()
	state := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("PREFIX", "")
	t.Setenv("PATH", emptyBin)
	t.Setenv(renderHelperEnv, "1")
	t.Setenv(renderReadyEnv, state)
	t.Setenv(renderEmitEnv, state)
	writeRenderConfig(t, home)

	m := NewManager(nil)
	if err := m.OnCreate([]string{
		"HOME=" + home, "XDG_CONFIG_HOME=" + home, "PREFIX=", "PATH=" + emptyBin,
	}, renderExe(t)); err != nil {
		t.Fatal(err)
	}
	m.OnVisibility(true)
	m.OnFocus(true)
	id := m.NewSession()
	if id == 0 {
		t.Fatal("manager started no session")
	}
	t.Cleanup(func() {
		for _, id := range m.SessionIDs() {
			m.CloseSession(id)
		}
		waitCount(t, m, 0)
	})
	r := &renderRig{t: t, m: m, id: id, state: state, w: w, h: h}
	r.attach(newRenderSurface(w, h))
	return r
}

func (r *renderRig) attach(surf *renderSurface) {
	r.t.Helper()
	old := nativeSurface
	nativeSurface = func(unsafe.Pointer) Surface { return surf }
	var token int
	r.m.OnSurfaceCreated(unsafe.Pointer(&token))
	nativeSurface = old
	r.surf = surf
}

func (r *renderRig) resize(density float64) {
	r.t.Helper()
	r.m.OnSurfaceChanged(r.w, r.h, density, 0, 60, 0, 90)
}

func (r *renderRig) waitPainted(n int) {
	r.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if countExt(r.state, ".ready") >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	r.t.Fatalf("%d painted children, want %d", countExt(r.state, ".ready"), n)
}

// emits is the number of protocol writes every child made together.
func (r *renderRig) emits() int {
	entries, err := os.ReadDir(r.state)
	if err != nil {
		return 0
	}
	total := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".emits" {
			continue
		}
		if info, err := e.Info(); err == nil {
			total += int(info.Size())
		}
	}
	return total
}

func (r *renderRig) requireEmits(want int) {
	r.t.Helper()
	if got := r.emits(); got != want {
		r.t.Fatalf("children emitted %d times, want %d", got, want)
	}
}

// requireSingleEmit proves one child painted the image once and never wrote
// again, so any later frame can only come from the terminal's own store.
func (r *renderRig) requireSingleEmit() {
	r.t.Helper()
	r.waitPainted(1)
	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) && r.emits() == 1 && countExt(r.state, ".ready") == 1 {
		time.Sleep(5 * time.Millisecond)
	}
	if n := countExt(r.state, ".ready"); n != 1 {
		r.t.Fatalf("%d children painted, want 1 (a restarted child re-emitted)", n)
	}
	r.requireEmits(1)
}

func countExt(dir, ext string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ext {
			n++
		}
	}
	return n
}

var (
	renderRed    = [4]uint8{255, 0, 0, 255}
	renderYellow = [4]uint8{255, 255, 0, 255}
	renderGreen  = [4]uint8{0, 255, 0, 255}
	renderBlue   = [4]uint8{0, 0, 255, 255}
	renderHalf   = [4]uint8{143, 15, 15, 191}
	renderBG     = [4]uint8{32, 32, 32, 128}
)

func surfacePixels(s *renderSurface) (pix []byte, stride, w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.pix...), s.stride, s.w, s.h
}

func surfaceFull(s *renderSurface) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.full
}

func at(pix []byte, stride, x, y int) [4]uint8 {
	i := y*stride + x*4
	return [4]uint8{pix[i], pix[i+1], pix[i+2], pix[i+3]}
}

func find(pix []byte, stride, w, h int, want [4]uint8) (x, y int, ok bool) {
	for y = 0; y < h; y++ {
		for x = 0; x < w; x++ {
			if at(pix, stride, x, y) == want {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func run(pix []byte, stride, w, h, x, y, dx, dy int, want [4]uint8) int {
	n := 0
	for x >= 0 && y >= 0 && x < w && y < h && at(pix, stride, x, y) == want {
		n++
		x += dx
		y += dy
	}
	return n
}

func (r *renderRig) waitForPixels() (x, y, cellW, cellH int) {
	r.t.Helper()
	return r.waitForColor(renderRed)
}

// waitForColor polls the window until the top-left quadrant of want is in it
// and returns its origin and cell size.
func (r *renderRig) waitForColor(want [4]uint8) (x, y, cellW, cellH int) {
	r.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		pix, stride, w, h := surfacePixels(r.surf)
		if x, y, ok := find(pix, stride, w, h, want); ok {
			cw := run(pix, stride, w, h, x, y, 1, 0, want)
			ch := run(pix, stride, w, h, x, y, 0, 1, want)
			if cw > 1 && ch > 1 {
				return x, y, cw, ch
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	r.t.Fatalf("the window never showed %v", want)
	return 0, 0, 0, 0
}

func waitFull(t *testing.T, s *renderSurface, want int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if surfaceFull(s) >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("whole-frame presents %d, want %d", surfaceFull(s), want)
}

// quiet holds want for a bounded window, so a repeated redraw or a child that
// writes again is caught rather than polled away.
func quiet(t *testing.T, s *renderSurface, want int) {
	t.Helper()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if n := surfaceFull(s); n != want {
			t.Fatalf("whole-frame presents %d, want %d to stay put", n, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// requireFull waits for want whole-frame presents, then proves the count
// settles there once the focus and resize events a surface change leaves
// behind have been delivered.
func requireFull(t *testing.T, s *renderSurface, want int) {
	t.Helper()
	waitFull(t, s, want)
	quiet(t, s, want)
}
