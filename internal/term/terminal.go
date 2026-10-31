package term

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

type ColorKind byte

const (
	ColorDefault ColorKind = iota
	ColorIndexed
	ColorRGB
)

type Color struct {
	Kind    ColorKind
	Index   byte
	R, G, B byte
}

func DefaultColor() Color { return Color{} }

func IndexedColor(idx byte) Color {
	return Color{Kind: ColorIndexed, Index: idx}
}

func RGBColor(r, g, b byte) Color {
	return Color{Kind: ColorRGB, R: r, G: g, B: b}
}

type Cell struct {
	Ch      rune
	marks   uint32 // combining marks drawn in the same cell, see Marks
	FG, BG  Color
	HasFG   bool
	HasBG   bool
	Bold    bool
	Dim     bool
	Italic  bool
	Under   bool
	Reverse bool
	Strike  bool
	Hidden  bool
	Wide    bool // left half of a double-width character
	cont    bool // right half of a double-width character
}

// savedCursor is the state captured by DECSC / mode 1048 / 1049.
type savedCursor struct {
	row, col int
	attr     Cell
	charsets charsets
}

func (c *Cell) ColorType() ColorKind {
	if !c.HasFG {
		return ColorDefault
	}
	return c.FG.Kind
}

func (c *Cell) FGIndex() byte {
	return c.FG.Index
}

const (
	maxScrollback = 10000
	// scrollbackSlack lets scrollback overrun its limit before a batch of the
	// oldest rows is dropped, so trimming isn't a copy per line.
	scrollbackSlack = 1024
	maxCachedWidths = 4096
)

type Terminal struct {
	Rows, Cols     int
	cells          [][]Cell
	altCells       [][]Cell
	scrollback     [][]Cell
	spare          [][]Cell // rows off the screen, reused by blankRow
	widths         map[rune]int8
	curRow, curCol int
	saved          savedCursor
	attr           Cell
	charsets       charsets
	lastChar       rune
	scrollTop      int
	scrollBot      int
	altScreen      bool
	wrapNext       bool
	noAutowrap     bool
	appCursorKeys  bool
	bracketedPaste bool
	cursorHidden   bool
	mouse          MouseMode

	state         int
	params        []int
	csiPrivate    byte
	csiSub        bool
	csiInter      byte
	subParams     [][]int
	osc           []byte
	oscHook       func(string)
	utf8Tail      []byte
	charsetTarget byte

	mu          sync.Mutex
	syncPending []byte
	synced      bool
	syncGen     uint64 // bumped per frame, so a stale timeout does nothing

	conn      conn     // the program's terminal, nil until a Start call
	proc      proc     // the started process, nil for StartFunc sessions
	fnDone    chan int // exit code of a StartFunc session
	fnAbort   atomic.Bool
	stopped   atomic.Bool
	input     inputQueue
	frameMu   sync.Mutex
	nextFrame time.Time
	exitCode  atomic.Int64
	done      atomic.Bool
	waitOnce  sync.Once
	exited    chan struct{}
	dirty     chan struct{}
}

const (
	stNormal = iota
	stEscape
	stCSI
	stOSC
	stCharset
	stEscInter
	stString // DCS, APC, PM, SOS: ignored up to ST
)

const maxOSC = 1 << 20 // room for OSC 52 clipboard payloads

func New(rows, cols int) *Terminal {
	if rows < 1 {
		rows = 24
	}
	if cols < 1 {
		cols = 80
	}
	t := &Terminal{
		Rows:      rows,
		Cols:      cols,
		scrollBot: rows - 1,
		exited:    make(chan struct{}),
		dirty:     make(chan struct{}, 1),
	}
	t.cells = makeGrid(rows, cols)
	t.altCells = makeGrid(rows, cols)
	return t
}

func makeGrid(rows, cols int) [][]Cell {
	g := make([][]Cell, rows)
	for i := range g {
		g[i] = make([]Cell, cols)
		for j := range g[i] {
			g[i][j].Ch = ' '
		}
	}
	return g
}

func (t *Terminal) StartCommand(dir, command string) error {
	return t.StartArgv(dir, "sh", "-c", command)
}

func (t *Terminal) StartArgv(dir string, argv ...string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	c, p, err := startProcess(startSpec{
		Argv: argv,
		Dir:  dir,
		Env:  append(os.Environ(), "TERM=xterm-256color"),
		Rows: t.Rows,
		Cols: t.Cols,
		OnStop: func() {
			t.stopped.Store(true)
			t.markDirty()
		},
	})
	if err != nil {
		return err
	}
	t.conn, t.proc = c, p
	return nil
}

// StartFunc runs fn as the session's program on a goroutine of this process.
// fn gets the PTY's terminal side as stdin/stdout, so it sees a real tty.
// Interrupt abandons it: the session ends at once with status 130 while fn
// finishes in the background with its terminal gone. Platforms without PTYs
// return ErrUnsupported; callers run the builtin as a multicall process.
func (t *Terminal) StartFunc(fn func(tty *os.File) int) error {
	c, tty, err := openFuncTTY(t.Rows, t.Cols)
	if err != nil {
		return err
	}
	t.conn = c
	t.fnDone = make(chan int, 1)
	go func() {
		t.fnDone <- fn(tty)
		tty.Close()
	}()
	return nil
}

// InProcess reports whether the session runs a StartFunc program.
func (t *Terminal) InProcess() bool { return t.fnDone != nil }

// Interrupt ends an in-process session, which no signal can reach. It reports
// false for a real process, whose interrupt is ^C on its terminal.
func (t *Terminal) Interrupt() bool {
	if !t.InProcess() {
		return false
	}
	t.fnAbort.Store(true)
	t.Close()
	return true
}

// StartReader drains the PTY on its own goroutine so rendering never stalls the
// child. Dirty is signalled (coalesced) after screen-changing output and once
// more when the child has exited and its final output has been applied.
func (t *Terminal) StartReader() {
	c := t.conn
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := c.Read(buf)
			if n > 0 && t.processChunk(buf[:n]) {
				t.markDirty()
			}
			if err != nil {
				break
			}
		}
		t.Finish()
		t.reap()
		close(t.exited)
		t.markDirty()
	}()
}

func (t *Terminal) markDirty() {
	select {
	case t.dirty <- struct{}{}:
	default:
	}
}

func (t *Terminal) Dirty() <-chan struct{} { return t.dirty }

// FrameDelay paces repaints: it returns how long to wait so successive frames
// are at least interval apart, and books that frame. An idle terminal's first
// change is never delayed.
func (t *Terminal) FrameDelay(interval time.Duration) time.Duration {
	t.frameMu.Lock()
	defer t.frameMu.Unlock()
	now := time.Now()
	at := t.nextFrame
	if now.After(at) {
		at = now
	}
	t.nextFrame = at.Add(interval)
	return at.Sub(now)
}

func (t *Terminal) Exited() <-chan struct{} { return t.exited }

func (t *Terminal) reap() {
	t.waitOnce.Do(func() {
		if t.fnDone != nil {
			code := 130
			if !t.fnAbort.Load() {
				select {
				case code = <-t.fnDone:
				default:
				}
			}
			t.finish(code)
			return
		}
		if t.proc == nil {
			return
		}
		t.finish(<-t.proc.Status())
	})
}

func (t *Terminal) InAltScreen() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.altScreen
}

// SendBytes queues data for the program's stdin without waiting for it.
func (t *Terminal) SendBytes(data []byte) {
	if t.conn != nil {
		t.enqueue(data)
	}
}

func (t *Terminal) Resize(rows, cols int) {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	t.mu.Lock()
	if !t.altScreen && t.curRow >= rows {
		t.pushTopRows(t.curRow - rows + 1)
	}
	nc := makeGrid(rows, cols)
	copyGrid(nc, t.cells, rows, cols, t.Rows, t.Cols)
	na := makeGrid(rows, cols)
	copyGrid(na, t.altCells, rows, cols, t.Rows, t.Cols)
	t.cells = nc
	t.altCells = na
	t.spare = nil
	t.Rows = rows
	t.Cols = cols
	t.scrollTop = 0
	t.scrollBot = rows - 1
	t.wrapNext = false
	t.curRow, t.curCol = t.clampPos(t.curRow, t.curCol)
	t.saved.row, t.saved.col = t.clampPos(t.saved.row, t.saved.col)
	t.mu.Unlock()
	if t.conn != nil {
		t.conn.Resize(rows, cols)
	}
}

// pushTopRows moves the top n main-screen rows into scrollback so the cursor
// line survives a shrink, as xterm does.
func (t *Terminal) pushTopRows(n int) {
	for i := 0; i < n; i++ {
		t.appendScrollback(t.cells[i])
	}
	t.cells = append(t.cells[n:], makeGrid(n, t.Cols)...)
	t.curRow -= n
	t.saved.row -= n
}

// appendScrollback takes ownership of row. Rows past the limit are dropped
// in batches and recycled, so steady scrolling neither allocates nor copies.
func (t *Terminal) appendScrollback(row []Cell) {
	t.scrollback = append(t.scrollback, row)
	if len(t.scrollback) < maxScrollback+scrollbackSlack {
		return
	}
	drop := len(t.scrollback) - maxScrollback
	for _, old := range t.scrollback[:drop] {
		t.recycle(old)
	}
	n := copy(t.scrollback, t.scrollback[drop:])
	clear(t.scrollback[n:])
	t.scrollback = t.scrollback[:n]
}

// recycle keeps a row that left the screen for blankRow: up to a trimmed
// batch of scrollback plus a screenful, rows that were just held anyway.
func (t *Terminal) recycle(row []Cell) {
	if len(row) == t.Cols && len(t.spare) < scrollbackSlack+t.Rows {
		t.spare = append(t.spare, row)
	}
}

func (t *Terminal) clampPos(row, col int) (int, int) {
	return max(0, min(row, t.Rows-1)), max(0, min(col, t.Cols-1))
}

func copyGrid(dst, src [][]Cell, dr, dc, sr, sc int) {
	for r := 0; r < dr && r < sr; r++ {
		for c := 0; c < dc && c < sc; c++ {
			dst[r][c] = src[r][c]
		}
	}
}

func (t *Terminal) Close() {
	if t.proc != nil {
		t.proc.Kill()
	}
	if t.conn != nil {
		t.conn.Close()
	}
	t.reap()
}

// finish records the exit code, then marks the terminal done, so a reader
// that sees Done also sees the code.
func (t *Terminal) finish(code int) {
	t.exitCode.Store(int64(code))
	t.done.Store(true)
}

func (t *Terminal) WaitExit() int {
	t.reap()
	return t.ExitCode()
}

func (t *Terminal) Done() bool    { return t.done.Load() }
func (t *Terminal) ExitCode() int { return int(t.exitCode.Load()) }

func (t *Terminal) AppCursorKeys() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.appCursorKeys
}

func (t *Terminal) BracketedPaste() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.bracketedPaste
}

func (t *Terminal) CursorVisible() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.cursorHidden
}

func (t *Terminal) CursorRow() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.curRow
}

func (t *Terminal) CursorCol() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.curCol
}

func (t *Terminal) ProcessOutput(data []byte) {
	t.processChunk(data)
}

// processChunk reports whether the visible screen may have changed.
func (t *Terminal) processChunk(data []byte) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.feedSync(data)
}

// Finish applies any output still held back by an unterminated sync block.
func (t *Terminal) Finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	pending := append(t.utf8Tail, t.syncPending...)
	t.syncPending = nil
	t.utf8Tail = nil
	t.synced = false
	t.processVT(pending)
	if len(t.utf8Tail) > 0 {
		t.utf8Tail = nil
		t.processRune(utf8.RuneError)
	}
}

// processVT interprets data, holding back a UTF-8 sequence split across reads
// until the rest of it arrives.
func (t *Terminal) processVT(data []byte) {
	if len(t.utf8Tail) > 0 {
		data = append(t.utf8Tail, data...)
		t.utf8Tail = nil
	}
	for len(data) > 0 {
		if t.state == stNormal && data[0] >= 0x20 && data[0] < 0x7f {
			if n := t.putASCII(data); n > 0 {
				data = data[n:]
				continue
			}
		}
		if t.state != stNormal || data[0] < utf8.RuneSelf {
			t.processByte(data[0])
			data = data[1:]
			continue
		}
		if !utf8.FullRune(data) {
			t.utf8Tail = append([]byte(nil), data...)
			return
		}
		r, sz := utf8.DecodeRune(data)
		t.processRune(r)
		data = data[sz:]
	}
}
