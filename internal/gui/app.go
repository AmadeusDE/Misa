package gui

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"math"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/AmadeusDE/Misa/internal/config"
	"github.com/AmadeusDE/Misa/internal/term"
	"github.com/AmadeusDE/Misa/internal/ui"
)

const (
	defaultFontSize = 11.0
	defaultPadding  = 4
	defaultColumns  = 110
	defaultRows     = 32
	minFontSize     = 4.0
	maxOSCQueue     = 64
)

// app is the OS-neutral window session: the terminal, the renderer, damage
// tracking and the event loop. Platform input and presentation go through be.
type app struct {
	be       Backend
	cfg      config.GUIConfig
	lib      *fontLibrary
	rend     renderer
	fontSize float64
	scale    float64

	argv []string
	dir  string

	term          *term.Terminal
	snap          term.Snapshot
	shown         frame
	frames        map[uint64]*frame
	shownW        int
	shownH        int
	logicalW      int
	logicalH      int
	drawPending   bool
	commitPending bool
	osc           oscQueue
	holding       bool
	exitCode      int
	mapped        bool
	focused       bool
	closed        bool
	lastCell      [2]int
}

func newApp(be Backend, cfg config.GUIConfig, dir string, argv []string) *app {
	fontSize := cfg.FontSize
	if fontSize <= 0 {
		fontSize = defaultFontSize
	}
	a := &app{
		be: be, cfg: cfg, argv: argv, dir: dir,
		lib: newFontLibrary(), fontSize: fontSize, scale: 1,
	}
	a.osc.ready = make(chan struct{}, 1)
	a.rend.pal = newPalette(cfg)
	return a
}

func (a *app) loadFonts() error {
	fs, err := newFontSet(a.lib, a.cfg.Font, a.fontSize, a.scale)
	if err != nil {
		return err
	}
	a.rend.fonts = fs
	pad := defaultPadding
	if a.cfg.Padding != nil {
		pad = max(0, *a.cfg.Padding)
	}
	a.rend.pad = scaleLen(pad, a.scale)
	a.shown.valid = false
	return nil
}

func (a *app) loop() (int, error) {
	for {
		var pastes <-chan string
		if c := a.be.Clipboard(); c != nil {
			pastes = c.Pastes()
		}
		select {
		case _, ok := <-a.be.Ready():
			if !ok {
				a.closeTerminal()
				return 1, a.be.Err()
			}
			a.be.Dispatch(a)
		case ev, ok := <-a.be.Events():
			if !ok {
				a.closeTerminal()
				return 1, a.be.Err()
			}
			a.Handle(ev)
		case <-a.termDirty():
			a.requestDraw()
		case <-a.termExited():
			if code := a.term.WaitExit(); code == 0 || !a.hold(code) {
				return code, nil
			}
		case <-a.be.Repeat():
			a.be.DispatchRepeat(a)
		case text := <-pastes:
			a.pasteText(text)
		case <-a.osc.ready:
			for _, s := range a.osc.take() {
				a.handleOSC(s)
			}
		}
		if err := a.be.Err(); err != nil {
			a.closeTerminal()
			return 1, err
		}
		if a.closed {
			a.closeTerminal()
			return a.exitCode, nil
		}
	}
}

func (a *app) Handle(ev Event) {
	switch ev.Kind {
	case EventResize:
		a.onResize(ev)
	case EventScale:
		a.onScale(ev.Scale)
	case EventFrame:
		a.redrawIfPending()
	case EventClose:
		a.closed = true
	case EventFocus:
		a.focused = ev.Focused
		a.requestDraw()
	case EventKey:
		a.onKey(ev)
	case EventText:
		a.onText(ev.Text)
	case EventPointer:
		a.onPointer(ev)
	case EventScroll:
		a.onScroll(ev)
	case EventFont:
		a.onFont(ev)
	}
}

func (a *app) onResize(ev Event) {
	if ev.Scale > 0 && ev.Scale != a.scale {
		a.scale = ev.Scale
		if err := a.loadFonts(); err != nil {
			return
		}
	}
	if ev.Width > 0 && ev.Height > 0 {
		a.logicalW, a.logicalH = ev.Width, ev.Height
	} else if a.logicalW == 0 {
		a.logicalW, a.logicalH = a.defaultSize()
	}
	a.mapped = true
	a.commitPending = true
	a.resizeTerminal()
	a.requestDraw()
}

func (a *app) onScale(scale float64) {
	if scale <= 0 || scale == a.scale {
		return
	}
	a.scale = scale
	if err := a.loadFonts(); err != nil {
		return
	}
	if a.logicalW > 0 {
		a.resizeTerminal()
	}
	a.requestDraw()
}

func (a *app) onKey(ev Event) {
	if !a.handleShortcut(ev.Key) && a.term != nil && (ev.Key.Code != 0 || ev.Key.Text != "") {
		a.term.SendBytes(ui.KeyBytes(ev.Key, a.term.AppCursorKeys()))
	}
	if a.holding && ev.DismissHold {
		a.closed = true
	}
}

func (a *app) handleShortcut(k tea.Key) bool {
	switch shortcutOf(k) {
	case pasteShortcut:
		if c := a.be.Clipboard(); c != nil {
			if text, err := c.Read(); err == nil && text != "" {
				a.pasteText(text)
			}
		}
		return true
	case zoomInShortcut:
		a.zoom(1)
		return true
	case zoomOutShortcut:
		a.zoom(-1)
		return true
	case zoomResetShortcut:
		a.zoom(0)
		return true
	default:
		return false
	}
}

func (a *app) onText(text string) {
	if a.term != nil && text != "" {
		a.term.SendBytes([]byte(text))
	}
}

func (a *app) pasteText(text string) {
	if a.term != nil && text != "" {
		a.term.SendBytes(ui.PasteBytes(text, a.term))
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
	if !a.mapped {
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

// RunSession drives one terminal session against be until it ends. dir is
// the child's working directory and argv is the program — on Android, the
// libmisa.so path. Wayland keeps its own setup and does not call this.
func RunSession(be Backend, dir string, argv []string) (int, error) {
	if be == nil {
		return 1, fmt.Errorf("gui: no backend")
	}
	a := newApp(be, config.LoadGUI(), dir, argv)
	if err := a.loadFonts(); err != nil {
		return 1, err
	}
	return a.loop()
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
	fs := a.rend.fonts
	pw := cols*fs.cellW + 2*a.rend.pad
	ph := rows*fs.cellH + 2*a.rend.pad
	return ceilDiv(pw, a.scale), ceilDiv(ph, a.scale)
}

func (a *app) pixelSize() (int, int) {
	return scaleLen(a.logicalW, a.scale), scaleLen(a.logicalH, a.scale)
}

func scaleLen(logical int, scale float64) int {
	if scale <= 0 {
		scale = 1
	}
	if scale == float64(int(scale)) {
		return logical * int(scale)
	}
	return int(math.Round(float64(logical) * scale))
}

func ceilDiv(px int, scale float64) int {
	if scale <= 0 {
		scale = 1
	}
	if scale == float64(int(scale)) {
		s := int(scale)
		return (px + s - 1) / s
	}
	return int(math.Ceil(float64(px) / scale))
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
		a.closed = true
		return
	}
	t.StartReader()
	a.term = t
}

// maxPinchFont caps a pinch factor. Zoom from the keyboard is unbounded
// aside from minFontSize, so a Wayland ctrl+shift+= still climbs past this.
const maxPinchFont = 72.0

// onFont applies EventFont. An absolute FontSize wins; otherwise Scale is
// a factor on the current size.
func (a *app) onFont(ev Event) {
	size := ev.FontSize
	if size <= 0 {
		if ev.Scale <= 0 || a.fontSize <= 0 {
			return
		}
		size = a.fontSize * ev.Scale
		if size > maxPinchFont {
			size = maxPinchFont
		}
	}
	if size < minFontSize {
		size = minFontSize
	}
	if size == a.fontSize {
		return
	}
	a.fontSize = size
	if err := a.loadFonts(); err != nil {
		return
	}
	a.be.DropBuffers()
	if a.logicalW > 0 || a.term != nil {
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
	a.onFont(Event{FontSize: size})
}

func (a *app) redrawIfPending() {
	if a.drawPending {
		a.requestDraw()
	}
}

// requestDraw paints the terminal now, or once the backend can take a frame.
func (a *app) requestDraw() {
	a.drawPending = true
	if a.be == nil || !a.be.CanPresent() || a.term == nil || a.logicalW == 0 {
		return
	}
	pw, ph := a.pixelSize()
	a.term.Snapshot(&a.snap)
	cur := cursorState{row: a.snap.CursorRow, col: a.snap.CursorCol, visible: a.snap.CursorVisible, focused: a.focused}
	rows, full := changedRows(&a.snap, cur, &a.shown)
	full = full || a.shownW != pw || a.shownH != ph
	if !full && len(rows) == 0 {
		a.drawPending = false
		if a.commitPending {
			a.be.Commit()
			a.commitPending = false
		}
		return
	}
	t := a.be.Buffer(pw, ph)
	if t == nil {
		return
	}
	c := canvas{pix: t.Pix, w: pw, h: ph, stride: t.Stride, order: orderOf(a.be.Format())}
	prev := t.content
	if prev == nil {
		prev = a.frameFor(t.ID)
	}
	a.rend.draw(&c, &a.snap, cur, prev)
	a.shown.record(&a.snap, cur)
	a.shownW, a.shownH = pw, ph
	a.be.Present(t.Pix, t.Stride, a.be.Format(), a.rend.damage(rows, full, c.bounds()))
	a.drawPending, a.commitPending = false, false
}

func (a *app) frameFor(id uint64) *frame {
	if a.frames == nil {
		a.frames = map[uint64]*frame{}
	}
	f := a.frames[id]
	if f == nil {
		f = &frame{}
		a.frames[id] = f
	}
	return f
}

func (a *app) handleOSC(s string) {
	code, rest, _ := strings.Cut(s, ";")
	switch code {
	case "0", "2":
		if a.be != nil {
			a.be.SetTitle(rest)
		}
	case "52":
		if !config.BoolDefault(a.cfg.ClipboardWrite, true) {
			return
		}
		_, data, ok := strings.Cut(rest, ";")
		if !ok || data == "?" {
			return
		}
		text, err := base64.StdEncoding.DecodeString(data)
		if err != nil || a.be == nil || a.be.Clipboard() == nil {
			return
		}
		a.be.Clipboard().Write(string(text))
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

func orderOf(f PixelFormat) pixelOrder {
	if f == FormatRGBA8888 {
		return orderRGBA
	}
	return orderBGRA
}
