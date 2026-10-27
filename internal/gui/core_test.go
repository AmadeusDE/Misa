package gui

import (
	"bytes"
	"image"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
	"github.com/AmadeusDE/Misa/internal/ui"
	"golang.org/x/image/font/gofont/gomono"
	xterm "golang.org/x/term"
)

func newTestApp(t *testing.T, be Backend) *app {
	t.Helper()
	a := newApp(be, config.GUIConfig{}, t.TempDir(), nil)
	a.lib.fcMatch = false
	a.lib.dirsSet = true
	if err := a.loadFonts(); err != nil {
		t.Fatal(err)
	}
	return a
}

func captureTerm(t *testing.T, rows, cols int) (*term.Terminal, func() string) {
	t.Helper()
	var mu sync.Mutex
	var buf bytes.Buffer
	tm := term.New(rows, cols)
	if err := tm.StartFunc(func(tty *os.File) int {
		if _, err := xterm.MakeRaw(int(tty.Fd())); err != nil {
			return 1
		}
		b := make([]byte, 256)
		for {
			n, err := tty.Read(b)
			if n > 0 {
				mu.Lock()
				buf.Write(b[:n])
				mu.Unlock()
			}
			if err != nil {
				return 0
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tm.Close() })
	return tm, func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}
}

func waitFor(t *testing.T, want string, got func() string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(got(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("got %q, want it to contain %q", got(), want)
}

func TestCoreResizeSetsGridAndMaps(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.term = term.New(2, 2)
	a.Handle(Event{Kind: EventResize, Width: 240, Height: 80, Scale: 2})
	if !a.mapped || a.logicalW != 240 || a.logicalH != 80 || a.scale != 2 {
		t.Fatalf("mapped %v size %dx%d scale %v", a.mapped, a.logicalW, a.logicalH, a.scale)
	}
	rows, cols := a.gridSize()
	if a.term.Rows != rows || a.term.Cols != cols || rows < 2 || cols < 2 {
		t.Fatalf("grid %dx%d term %dx%d", cols, rows, a.term.Cols, a.term.Rows)
	}
	if len(fb.frames) == 0 {
		t.Fatal("resize did not present a frame")
	}
}

func TestCoreScaleReloadsFonts(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	before := a.rend.fonts.cellW
	a.logicalW, a.logicalH = 200, 80
	a.term = term.New(2, 2)
	a.Handle(Event{Kind: EventScale, Scale: 2})
	if a.scale != 2 || a.rend.fonts.cellW <= before {
		t.Fatalf("scale %v cell %d -> %d", a.scale, before, a.rend.fonts.cellW)
	}
}

func TestCoreDamageRectsTrackChanges(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 200, 80
	rows, cols := a.gridSize()
	a.term = term.New(rows, cols)
	a.term.ProcessOutput([]byte("a"))
	a.requestDraw()
	if len(fb.frames) != 1 {
		t.Fatalf("frames %d", len(fb.frames))
	}
	pw, ph := a.pixelSize()
	full := image.Rect(0, 0, pw, ph)
	if len(fb.frames[0].damage) != 1 || fb.frames[0].damage[0] != full {
		t.Fatalf("first damage %v, want %v", fb.frames[0].damage, full)
	}
	a.term.ProcessOutput([]byte("\r\nb"))
	a.requestDraw()
	if len(fb.frames) != 2 {
		t.Fatalf("frames %d", len(fb.frames))
	}
	if len(fb.frames[1].damage) == 0 || (len(fb.frames[1].damage) == 1 && fb.frames[1].damage[0] == full) {
		t.Fatalf("second damage %v still covers the frame", fb.frames[1].damage)
	}
}

func TestCoreDrawsBackendPixelOrder(t *testing.T) {
	fb := newFakeBackend()
	fb.format = FormatRGBA8888
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 120, 40
	rows, cols := a.gridSize()
	a.term = term.New(rows, cols)
	a.term.ProcessOutput([]byte("\x1b[41mX"))
	a.requestDraw()
	if len(fb.frames) != 1 {
		t.Fatal("no frame")
	}
	pix := fb.frames[0].pix
	var found bool
	for i := 0; i+3 < len(pix); i += 4 {
		if pix[i] == 0xcc && pix[i+1] == 0x24 && pix[i+2] == 0x1d {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("red cell was not stored as RGBA")
	}
}

func TestCoreFrameWaitsUntilPresentable(t *testing.T) {
	fb := newFakeBackend()
	fb.canPresent = false
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 100, 40
	a.term = term.New(2, 2)
	a.term.ProcessOutput([]byte("x"))
	a.requestDraw()
	if len(fb.frames) != 0 || !a.drawPending {
		t.Fatalf("frames %d pending %v", len(fb.frames), a.drawPending)
	}
	fb.canPresent = true
	a.Handle(Event{Kind: EventFrame})
	if len(fb.frames) != 1 || a.drawPending {
		t.Fatalf("frames %d pending %v", len(fb.frames), a.drawPending)
	}
}

func TestCoreCommitWhenNothingChanged(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 100, 40
	a.term = term.New(2, 2)
	a.term.ProcessOutput([]byte("x"))
	a.requestDraw()
	a.commitPending = true
	a.requestDraw()
	if fb.commits != 1 {
		t.Fatalf("commits %d", fb.commits)
	}
	if len(fb.frames) != 1 {
		t.Fatalf("unchanged frame was presented again: %d", len(fb.frames))
	}
}

func TestCoreKeyTextMouseAndPasteReachTerminal(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 200, 80
	rows, cols := a.gridSize()
	tm, got := captureTerm(t, rows, cols)
	tm.ProcessOutput([]byte("\x1b[?1000h\x1b[?1006h"))
	a.term = tm
	a.snap = term.Snapshot{Rows: rows, Cols: cols}
	a.term.Snapshot(&a.snap)

	a.Handle(Event{Kind: EventKey, Key: tea.Key{Code: 'a', Text: "a"}})
	a.Handle(Event{Kind: EventText, Text: "é"})
	fb.clip.mu.Lock()
	fb.clip.text = "zz"
	fb.clip.mu.Unlock()
	a.Handle(Event{Kind: EventKey, Key: tea.Key{Code: 'v', Mod: tea.ModCtrl | tea.ModShift}})

	x := float64(a.rend.pad + 1)
	y := float64(a.rend.pad + 1)
	a.Handle(Event{Kind: EventPointer, Action: PointerDown, Button: tea.MouseLeft, X: x, Y: y})
	a.Handle(Event{Kind: EventScroll, Axis: 0, Steps: 1, X: x, Y: y})

	col, row := a.cellAt(x, y)
	click := string(ui.MouseBytes(tea.MouseClickMsg(tea.Mouse{X: col, Y: row, Button: tea.MouseLeft}), col, row, tm.Mouse()))
	wheel := string(ui.MouseBytes(tea.MouseWheelMsg(tea.Mouse{X: col, Y: row, Button: tea.MouseWheelDown}), col, row, tm.Mouse()))
	waitFor(t, "aézz"+click+wheel, got)
}

func TestCoreZoomAndTitleAndClipboard(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.logicalW, a.logicalH = 80, 40
	a.term = term.New(2, 2)
	a.Handle(Event{Kind: EventKey, Key: tea.Key{Code: '=', Mod: tea.ModCtrl | tea.ModShift}})
	if a.fontSize != defaultFontSize+1 || fb.drops != 1 {
		t.Fatalf("size %v drops %d", a.fontSize, fb.drops)
	}
	a.handleOSC("0;hello")
	a.handleOSC("2;there")
	if fb.title != "there" {
		t.Fatalf("title %q", fb.title)
	}
	a.handleOSC("52;c;aGVsbG8=")
	fb.clip.mu.Lock()
	got := fb.clip.text
	fb.clip.mu.Unlock()
	if got != "hello" || fb.clip.writes != 1 {
		t.Fatalf("clipboard %q writes %d", got, fb.clip.writes)
	}
}

func TestCoreHoldKeyCloses(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	a.mapped = true
	a.logicalW, a.logicalH = 80, 24
	a.term = term.New(3, 40)
	a.term.ProcessOutput([]byte("nope"))
	if !a.hold(9) {
		t.Fatal("hold refused")
	}
	a.Handle(Event{Kind: EventKey, Key: tea.Key{Code: 'q', Text: "q"}, DismissHold: true})
	if !a.closed || a.exitCode != 9 {
		t.Fatalf("closed %v code %d", a.closed, a.exitCode)
	}
}

func TestCoreLoopResizePasteAndClose(t *testing.T) {
	fb := newFakeBackend()
	a := newTestApp(t, fb)
	rows, cols := 4, 20
	tm, got := captureTerm(t, rows, cols)
	a.term = tm
	done := make(chan struct{})
	go func() {
		a.loop()
		close(done)
	}()
	fb.push(Event{Kind: EventResize, Width: 160, Height: 48, Scale: 1})
	deadline := time.Now().Add(2 * time.Second)
	for fb.frameCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if fb.frameCount() == 0 {
		t.Fatal("resize did not present a frame")
	}
	if a.logicalW != 160 || !a.mapped {
		t.Fatalf("resize size %d mapped %v", a.logicalW, a.mapped)
	}
	fb.clip.pastes <- "yo"
	waitFor(t, "yo", got)
	fb.push(Event{Kind: EventClose})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not exit on close")
	}
	if !a.closed {
		t.Fatal("close was not recorded")
	}
}

func TestBlitSwizzlesBGRAAndRGBA(t *testing.T) {
	src := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	dst := make([]byte, 8)
	blit(dst, 8, FormatRGBA8888, src, 8, FormatBGRA8888, 2, 1)
	want := []byte{3, 2, 1, 4, 7, 6, 5, 8}
	if string(dst) != string(want) {
		t.Fatalf("swizzle %v", dst)
	}
	copy(dst, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	blit(dst, 8, FormatBGRA8888, src, 8, FormatBGRA8888, 2, 1)
	if string(dst) != string(src) {
		t.Fatalf("copy %v", dst)
	}
}

func TestSystemFontDirsUsesPrefix(t *testing.T) {
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	dirs := systemFontDirs()
	if !strings.Contains(strings.Join(dirs, "\n"), "/system/fonts") {
		t.Fatalf("dirs %v", dirs)
	}
	if !strings.Contains(strings.Join(dirs, "\n"), "/data/data/com.termux/files/usr/share/fonts") {
		t.Fatalf("dirs %v", dirs)
	}
}

func TestSystemFontDirsWithoutPrefix(t *testing.T) {
	t.Setenv("PREFIX", "")
	dirs := systemFontDirs()
	if len(dirs) != 1 || dirs[0] != "/system/fonts" {
		t.Fatalf("dirs %v", dirs)
	}
}

func TestFontDirFallbackFindsMonoThenEmbedded(t *testing.T) {
	dir := t.TempDir()
	regular := dir + "/NotoSansMono-Regular.ttf"
	plain := dir + "/NotoSans-Regular.ttf"
	if err := os.WriteFile(regular, gomono.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plain, gomono.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	sub := dir + "/truetype"
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	bold := sub + "/NotoSansMono-Bold.ttf"
	if err := os.WriteFile(bold, gomono.TTF, 0o644); err != nil {
		t.Fatal(err)
	}

	lib := newFontLibrary()
	lib.fcMatch = false
	lib.dirsSet = true
	lib.dirs = []string{dir}
	got := lib.family("monospace", styleRegular)
	if got == nil || got == lib.embedded(styleRegular) {
		t.Fatal("regular mono was not found in the search path")
	}
	if lib.family("monospace", styleBold) == nil {
		t.Fatal("bold mono in a subdirectory was not found")
	}
	if lib.family("monospace", styleItalic) != nil {
		t.Fatal("italic matched a file that is not italic")
	}
	if lib.family("Comic Sans", styleRegular) != nil {
		t.Fatal("unrelated family matched a mono file")
	}

	empty := newFontLibrary()
	empty.fcMatch = false
	empty.dirsSet = true
	fs, err := newFontSet(empty, "monospace", 12, 1)
	if err != nil {
		t.Fatal(err)
	}
	if fs.styles[styleRegular].file != empty.embedded(styleRegular) {
		t.Fatal("missing system font did not fall back to Go Mono")
	}
}
