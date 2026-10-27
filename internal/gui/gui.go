// Package gui is an optional Wayland window for Misa: a software-rendered
// terminal, in pure Go, that runs Misa (or another command) on a PTY for
// when the host terminal lacks features or isn't wanted.
package gui

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
	"github.com/AmadeusDE/Misa/internal/ui"
	"github.com/AmadeusDE/Misa/internal/wayland"
)

const (
	defaultFontSize = 11.0
	defaultPadding  = 4
	defaultColumns  = 110
	defaultRows     = 32
	minFontSize     = 4.0
	maxOSCQueue     = 64
)

type app struct {
	conn     *wayland.Conn
	win      *window
	seat     seat
	clip     clipboard
	cfg      config.GUIConfig
	lib      *fontLibrary
	rend     renderer
	fontSize float64

	argv          []string
	dir           string
	term          *term.Terminal
	snap          term.Snapshot
	shown         frame
	shownW        int
	shownH        int
	logicalW      int
	logicalH      int
	drawPending   bool
	commitPending bool
	osc           oscQueue
	holding       bool // the program failed; its output stays until a key
	exitCode      int
}

// Run opens a window titled title and runs argv in it until the program
// exits or the window closes, returning the program's exit status. A program
// that fails leaves its last screen up until a key is pressed.
func Run(dir string, argv []string, title string) int {
	code, err := run(dir, argv, title)
	if err != nil {
		fmt.Fprintln(os.Stderr, "misa: gui:", err)
		return 1
	}
	return code
}

func run(dir string, argv []string, title string) (int, error) {
	conn, err := wayland.Connect()
	if err != nil {
		return 1, err
	}
	defer conn.Close()
	reg := conn.GetRegistry()
	if err := conn.Roundtrip(); err != nil {
		return 1, err
	}
	cfg := config.LoadGUI()
	a := &app{conn: conn, cfg: cfg, argv: argv, dir: dir, lib: newFontLibrary(), fontSize: cfg.FontSize}
	if a.fontSize <= 0 {
		a.fontSize = defaultFontSize
	}
	a.osc.ready = make(chan struct{}, 1)
	a.rend.pal = newPalette(cfg)
	if a.win, err = newWindow(conn, reg, title, a.rend.pal.translucent()); err != nil {
		return 1, err
	}
	a.win.onConfigure = a.configured
	a.win.onScale = a.rescale
	a.win.onFrame = a.redrawIfPending
	a.win.onBufferRelease = a.redrawIfPending
	a.bindSeat(reg)
	a.bindClipboard(reg)
	if err := a.loadFonts(); err != nil {
		return 1, err
	}
	return a.loop()
}

func (a *app) loadFonts() error {
	fs, err := newFontSet(a.lib, a.cfg.Font, a.fontSize, a.win.scale)
	if err != nil {
		return err
	}
	a.rend.fonts = fs
	pad := defaultPadding
	if a.cfg.Padding != nil {
		pad = max(0, *a.cfg.Padding)
	}
	a.rend.pad = pad * a.win.scale
	a.shown.valid = false
	return nil
}

func (a *app) loop() (int, error) {
	for {
		select {
		case ev, ok := <-a.conn.Events():
			if !ok {
				a.closeTerminal()
				return 1, a.conn.Err()
			}
			a.conn.Dispatch(ev)
		case <-a.termDirty():
			a.requestDraw()
		case <-a.termExited():
			if code := a.term.WaitExit(); code == 0 || !a.hold(code) {
				return code, nil
			}
		case <-a.seat.repeatC():
			a.repeatKey()
		case text := <-a.clip.pasted:
			if a.term != nil {
				a.term.SendBytes(ui.PasteBytes(text, a.term))
			}
		case <-a.osc.ready:
			for _, s := range a.osc.take() {
				a.handleOSC(s)
			}
		}
		if err := a.conn.Err(); err != nil {
			a.closeTerminal()
			return 1, err
		}
		if a.win.closeRequested {
			a.closeTerminal()
			return a.exitCode, nil
		}
	}
}

func (a *app) termDirty() <-chan struct{} {
	if a.term == nil {
		return nil
	}
	return a.term.Dirty()
}

func (a *app) termExited() <-chan struct{} {
	if a.term == nil || a.holding {
		return nil
	}
	return a.term.Exited()
}

// hold keeps a failed program's screen up with a note, so errors (an ssh
// connection refused) can be read. It reports false if nothing can be shown.
func (a *app) hold(code int) bool {
	if !a.win.configured {
		return false
	}
	a.holding, a.exitCode = true, code
	a.term.ProcessOutput(fmt.Appendf(nil, "\r\n\x1b[0;2m[exited with status %d — press any key to close]\x1b[m", code))
	a.requestDraw()
	return true
}

func (a *app) closeTerminal() {
	if a.term != nil {
		a.term.Close()
	}
}

func (a *app) configured() {
	if a.win.width > 0 && a.win.height > 0 {
		a.logicalW, a.logicalH = a.win.width, a.win.height
	} else if a.logicalW == 0 {
		a.logicalW, a.logicalH = a.defaultSize()
	}
	a.commitPending = true
	a.resizeTerminal()
	a.requestDraw()
}

// defaultSize is the logical window size for the configured grid.
func (a *app) defaultSize() (int, int) {
	cols, rows := a.cfg.Columns, a.cfg.Rows
	if cols <= 0 {
		cols = defaultColumns
	}
	if rows <= 0 {
		rows = defaultRows
	}
	fs, s := a.rend.fonts, a.win.scale
	return (cols*fs.cellW + 2*a.rend.pad + s - 1) / s, (rows*fs.cellH + 2*a.rend.pad + s - 1) / s
}

func (a *app) pixelSize() (int, int) {
	return a.logicalW * a.win.scale, a.logicalH * a.win.scale
}

func (a *app) gridSize() (rows, cols int) {
	pw, ph := a.pixelSize()
	fs := a.rend.fonts
	return max(1, (ph-2*a.rend.pad)/fs.cellH), max(1, (pw-2*a.rend.pad)/fs.cellW)
}

func (a *app) resizeTerminal() {
	rows, cols := a.gridSize()
	if a.term == nil {
		a.startTerminal(rows, cols)
		return
	}
	if rows != a.term.Rows || cols != a.term.Cols {
		a.term.Resize(rows, cols)
	}
}

func (a *app) startTerminal(rows, cols int) {
	t := term.New(rows, cols)
	t.SetOSCHook(a.osc.push)
	pal := a.rend.pal
	term.SetHostForeground(color.RGBA{pal.fg.r, pal.fg.g, pal.fg.b, 0xff})
	term.SetHostBackground(color.RGBA{pal.bg.r, pal.bg.g, pal.bg.b, 0xff})
	os.Setenv("COLORTERM", "truecolor")
	if err := t.StartArgv(a.dir, a.argv...); err != nil {
		fmt.Fprintln(os.Stderr, "misa: gui:", err)
		a.win.closeRequested = true
		return
	}
	t.StartReader()
	a.term = t
}

// rescale follows a new buffer scale: glyphs re-rasterize at the new
// density, keeping the logical size.
func (a *app) rescale() {
	if err := a.loadFonts(); err != nil {
		return
	}
	if a.logicalW > 0 {
		a.resizeTerminal()
	}
	a.requestDraw()
}

// zoom changes the font size by step points, or restores it for 0.
func (a *app) zoom(step int) {
	size := a.cfg.FontSize
	if size <= 0 {
		size = defaultFontSize
	}
	if step != 0 {
		size = max(minFontSize, a.fontSize+float64(step))
	}
	if size == a.fontSize {
		return
	}
	a.fontSize = size
	if err := a.loadFonts(); err != nil {
		return
	}
	a.win.dropBuffers()
	a.resizeTerminal()
	a.requestDraw()
}

func (a *app) redrawIfPending() {
	if a.drawPending {
		a.requestDraw()
	}
}

// requestDraw paints the terminal now, or once the compositor wants the next
// frame or frees a buffer.
func (a *app) requestDraw() {
	a.drawPending = true
	if !a.win.configured || a.win.frameCB != 0 || a.term == nil || a.logicalW == 0 {
		return
	}
	pw, ph := a.pixelSize()
	a.term.Snapshot(&a.snap)
	cur := cursorState{row: a.snap.CursorRow, col: a.snap.CursorCol, visible: a.snap.CursorVisible, focused: a.seat.focused}
	rows, full := changedRows(&a.snap, cur, &a.shown)
	full = full || a.shownW != pw || a.shownH != ph
	if !full && len(rows) == 0 {
		a.drawPending = false
		if a.commitPending {
			a.win.commit()
			a.commitPending = false
		}
		return
	}
	b := a.win.bufferFor(pw, ph)
	if b == nil {
		return
	}
	a.rend.draw(&b.c, &a.snap, cur, &b.content)
	a.shown.record(&a.snap, cur)
	a.shownW, a.shownH = pw, ph
	a.win.present(b, a.rend.damage(rows, full, b.c.bounds()))
	a.drawPending, a.commitPending = false, false
}

func (a *app) handleOSC(s string) {
	code, rest, _ := strings.Cut(s, ";")
	switch code {
	case "0", "2":
		a.win.setTitle(rest)
	case "52":
		if !config.BoolDefault(a.cfg.ClipboardWrite, true) {
			return
		}
		_, data, ok := strings.Cut(rest, ";")
		if !ok || data == "?" {
			return
		}
		if text, err := base64.StdEncoding.DecodeString(data); err == nil {
			a.clip.copy(a, string(text))
		}
	}
}

// oscQueue carries OSC strings from the terminal's reader goroutine, where
// the hook must not block, to the event loop.
type oscQueue struct {
	mu    sync.Mutex
	items []string
	ready chan struct{}
}

func (q *oscQueue) push(s string) {
	q.mu.Lock()
	if len(q.items) < maxOSCQueue {
		q.items = append(q.items, s)
	}
	q.mu.Unlock()
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

func (q *oscQueue) take() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.items
	q.items = nil
	return items
}
