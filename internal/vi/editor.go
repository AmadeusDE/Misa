// Built-in vi editor. See NOTICE for licensing.
package vi

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/AmadeusDE/Misa/internal/builtins"

	tea "charm.land/bubbletea/v2"
)

//go:embed misatutor
var tutorText string

type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeReplace
	ModeReplaceChar
	ModeCommand
	ModeSearch
)

type visualKind int

const (
	visualNone visualKind = iota
	visualChar
	visualLine
)

type pos struct{ row, col int }

type regContent struct {
	text   []string
	isLine bool
	keys   []string
}

type Editor struct {
	lines    []string
	filename string
	modified bool

	row, col           int
	goalCol            int
	goalValid, goalEnd bool
	topLine            int
	width              int
	height             int

	mode      Mode
	cmdBuf    string
	statusMsg string

	undoStack                             []undoEntry
	redoStack                             []undoEntry
	revision, savedRevision, nextRevision uint64
	editDepth                             int
	editBefore                            undoEntry
	editSavedLines                        []string
	editSavedRevision                     uint64
	editPos                               pos
	editPosSet                            bool
	editAuto                              bool

	register []string
	regLine  bool

	regs       map[byte]*regContent
	pendingReg byte
	hasReg     bool

	recording  bool
	recordReg  byte
	recordBuf  []string
	lastMacro  byte
	macroDepth int

	visual visualKind
	visRow int
	visCol int

	pendingOp    byte
	pendingKey   string
	count        int
	countGiven   bool
	opCount      int
	opCountGiven bool

	repeatChange, repeatPending semanticChange
	replayingChange             bool
	replaceSteps                []replaceStep
	insertBuf                   strings.Builder
	insertEntry                 string
	insertCount                 int
	replaceCount                int

	searchPat      string
	searchDir      int
	searchInputDir int

	ignoreCase bool

	jumpList []pos
	jumpIdx  int

	lineUndoRow  int
	lineUndoText string
	lineUndoSet  bool

	lastFChar rune
	lastFCmd  byte

	marks   map[byte]pos
	abbrevs map[string]string

	Quit bool
}

func (e *Editor) Register() ([]string, bool) { return e.register, e.regLine }

func (e *Editor) SetRegister(reg []string, line bool) {
	e.register = reg
	e.regLine = line
}

func init() {
	builtins.Register(builtins.Command{Name: "vi", Run: runBuiltin})
}

func runBuiltin(ctx *builtins.Context) int {
	filename := ""
	if len(ctx.Args) > 0 {
		filename = ctx.Args[0]
	}
	if err := RunStandalone(filename, ctx.Dir); err != nil {
		fmt.Fprintln(ctx.Stderr, "vi:", err)
		return 1
	}
	return 0
}

func New(filename, dir string) *Editor {
	e := &Editor{
		lines:     []string{""},
		searchDir: 1,
		regs:      make(map[byte]*regContent),
		marks:     make(map[byte]pos),
		abbrevs:   make(map[string]string),
	}
	if filename != "" {
		if !filepath.IsAbs(filename) {
			filename = filepath.Join(dir, filename)
		}
		e.filename = filename
		if data, err := os.ReadFile(filename); err == nil {
			text := strings.ReplaceAll(string(data), "\r\n", "\n")
			e.lines = strings.Split(text, "\n")
			if len(e.lines) > 1 && e.lines[len(e.lines)-1] == "" {
				e.lines = e.lines[:len(e.lines)-1]
			}
			if len(e.lines) == 0 {
				e.lines = []string{""}
			}
		}
	}
	return e
}

func (e *Editor) Resize(w, h int) {
	e.width = w
	e.height = h - 2
	if e.height < 1 {
		e.height = 1
	}
	e.ensureVisible()
}

func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	defer e.finishEditEvent(e.row)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.Resize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		e.statusMsg = ""
		if e.visual != visualNone && e.mode == ModeNormal {
			e.updateVisual(msg)
			return nil
		}
		switch e.mode {
		case ModeNormal:
			return e.handleNormal(msg)
		case ModeInsert:
			return e.handleInsert(msg)
		case ModeReplace:
			return e.handleReplace(msg)
		case ModeReplaceChar:
			return e.handleReplaceChar(msg)
		case ModeCommand:
			return e.handleCommand(msg)
		case ModeSearch:
			return e.handleSearch(msg)
		}
	}
	return nil
}

func (e *Editor) addJump() {
	if len(e.jumpList) > 0 {
		last := e.jumpList[len(e.jumpList)-1]
		if last.row == e.row && last.col == e.col {
			return
		}
	}
	e.jumpList = append(e.jumpList, pos{e.row, e.col})
	if len(e.jumpList) > 100 {
		e.jumpList = e.jumpList[len(e.jumpList)-100:]
	}
	e.jumpIdx = len(e.jumpList)
}

func (e *Editor) jumpBack() {
	if e.jumpIdx <= 0 || len(e.jumpList) == 0 {
		e.statusMsg = "No older jump"
		return
	}
	if e.jumpIdx >= len(e.jumpList) {
		e.addJump()
		e.jumpIdx = len(e.jumpList) - 1
	}
	e.jumpIdx--
	j := e.jumpList[e.jumpIdx]
	e.row = j.row
	e.col = j.col
	e.clampCursor()
	e.ensureVisible()
}

func (e *Editor) jumpForward() {
	if e.jumpIdx >= len(e.jumpList)-1 {
		e.statusMsg = "No newer jump"
		return
	}
	e.jumpIdx++
	j := e.jumpList[e.jumpIdx]
	e.row = j.row
	e.col = j.col
	e.clampCursor()
	e.ensureVisible()
}

// --- Buffer Ops ---

func (e *Editor) currentLine() string {
	if e.row >= 0 && e.row < len(e.lines) {
		return e.lines[e.row]
	}
	return ""
}

func (e *Editor) lineRunes(row int) []rune {
	if row >= 0 && row < len(e.lines) {
		return []rune(e.lines[row])
	}
	return nil
}

func (e *Editor) setLine(row int, s string) {
	if row >= 0 && row < len(e.lines) {
		if e.lines[row] != s {
			_, col := changedPosition([]string{e.lines[row]}, []string{s})
			e.noteEdit(row, col)
		}
		e.lines[row] = s
		e.modified = true
	}
}

func (e *Editor) insertCharAt(row, col int, ch rune) {
	runes := e.lineRunes(row)
	if col > len(runes) {
		col = len(runes)
	}
	e.noteEdit(row, col)
	nr := make([]rune, 0, len(runes)+1)
	nr = append(nr, runes[:col]...)
	nr = append(nr, ch)
	nr = append(nr, runes[col:]...)
	e.setLine(row, string(nr))
}

func (e *Editor) deleteCharAt(row, col int) rune {
	runes := e.lineRunes(row)
	if col >= len(runes) {
		return 0
	}
	e.noteEdit(row, col)
	ch := runes[col]
	nr := make([]rune, 0, len(runes)-1)
	nr = append(nr, runes[:col]...)
	nr = append(nr, runes[col+1:]...)
	e.setLine(row, string(nr))
	return ch
}

func (e *Editor) deleteLine(row int) string {
	if row < 0 || row >= len(e.lines) {
		return ""
	}
	line := e.lines[row]
	e.noteEdit(row, 0)
	e.lines = append(e.lines[:row], e.lines[row+1:]...)
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	e.modified = true
	return line
}

func (e *Editor) insertLineAfter(row int, s string) {
	if row < 0 {
		row = 0
	}
	if row >= len(e.lines) {
		row = len(e.lines) - 1
	}
	e.noteEdit(row+1, 0)
	nl := make([]string, 0, len(e.lines)+1)
	nl = append(nl, e.lines[:row+1]...)
	nl = append(nl, s)
	nl = append(nl, e.lines[row+1:]...)
	e.lines = nl
	e.modified = true
}

func (e *Editor) insertLineBefore(row int, s string) {
	if row < 0 {
		row = 0
	}
	if row > len(e.lines) {
		row = len(e.lines)
	}
	e.noteEdit(row, 0)
	nl := make([]string, 0, len(e.lines)+1)
	nl = append(nl, e.lines[:row]...)
	nl = append(nl, s)
	nl = append(nl, e.lines[row:]...)
	e.lines = nl
	e.modified = true
}

func (e *Editor) splitLine(row, col int) {
	runes := e.lineRunes(row)
	if col > len(runes) {
		col = len(runes)
	}
	e.noteEdit(row, col)
	before := string(runes[:col])
	after := string(runes[col:])
	e.lines[row] = before
	e.insertLineAfter(row, after)
}

func (e *Editor) doJoinLine(row int) {
	if row < 0 || row >= len(e.lines)-1 {
		return
	}
	a := e.lines[row]
	e.noteEdit(row, len([]rune(a)))
	b := strings.TrimLeft(e.lines[row+1], " \t")
	e.col = len([]rune(a))
	if len(a) > 0 && len(b) > 0 {
		e.lines[row] = a + " " + b
	} else {
		e.lines[row] = a + b
	}
	e.lines = append(e.lines[:row+1], e.lines[row+2:]...)
	e.modified = true
}

func (e *Editor) deleteRange(fromRow, fromCol, toRow, toCol int) {
	if fromRow > toRow || (fromRow == toRow && fromCol > toCol) {
		fromRow, fromCol, toRow, toCol = toRow, toCol, fromRow, fromCol
	}
	e.noteEdit(fromRow, fromCol)
	if fromRow == toRow {
		runes := e.lineRunes(fromRow)
		if fromCol > len(runes) {
			fromCol = len(runes)
		}
		if toCol > len(runes) {
			toCol = len(runes)
		}
		nr := make([]rune, 0, len(runes)-(toCol-fromCol))
		nr = append(nr, runes[:fromCol]...)
		nr = append(nr, runes[toCol:]...)
		e.setLine(fromRow, string(nr))
		return
	}
	firstRunes := e.lineRunes(fromRow)
	lastRunes := e.lineRunes(toRow)
	if fromCol > len(firstRunes) {
		fromCol = len(firstRunes)
	}
	if toCol > len(lastRunes) {
		toCol = len(lastRunes)
	}
	e.lines[fromRow] = string(firstRunes[:fromCol]) + string(lastRunes[toCol:])
	e.lines = append(e.lines[:fromRow+1], e.lines[toRow+1:]...)
	e.modified = true
}

func (e *Editor) extractText(fromRow, fromCol, toRow, toCol int) string {
	if fromRow == toRow {
		runes := e.lineRunes(fromRow)
		if fromCol > len(runes) {
			fromCol = len(runes)
		}
		if toCol > len(runes) {
			toCol = len(runes)
		}
		if fromCol >= toCol {
			return ""
		}
		return string(runes[fromCol:toCol])
	}
	var parts []string
	r := e.lineRunes(fromRow)
	if fromCol > len(r) {
		fromCol = len(r)
	}
	parts = append(parts, string(r[fromCol:]))
	for i := fromRow + 1; i < toRow; i++ {
		parts = append(parts, e.lines[i])
	}
	r = e.lineRunes(toRow)
	if toCol > len(r) {
		toCol = len(r)
	}
	parts = append(parts, string(r[:toCol]))
	return strings.Join(parts, "\n")
}

// --- Cursor ---

func (e *Editor) clampCursor() {
	if e.row < 0 {
		e.row = 0
	}
	if e.row >= len(e.lines) {
		e.row = len(e.lines) - 1
	}
	runes := e.lineRunes(e.row)
	maxCol := len(runes)
	if e.mode != ModeInsert && e.mode != ModeReplace {
		if maxCol > 0 {
			maxCol--
		}
	}
	if e.col < 0 {
		e.col = 0
	}
	if e.col > maxCol {
		e.col = maxCol
	}
}

func (e *Editor) ensureVisible() {
	if e.height <= 0 {
		return
	}
	if e.row < e.topLine {
		e.topLine = e.row
	}
	if e.row >= e.topLine+e.height {
		e.topLine = e.row - e.height + 1
	}
	if e.topLine < 0 {
		e.topLine = 0
	}
}

func (e *Editor) firstNonBlank(row int) int {
	if row < 0 || row >= len(e.lines) {
		return 0
	}
	for i, r := range []rune(e.lines[row]) {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return 0
}

// --- Word / Sentence / Paragraph / Section motions ---

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isPunct(r rune) bool {
	return !unicode.IsSpace(r) && !isWordChar(r)
}

func (e *Editor) wordForward(count int) (int, int) {
	row, col := e.row, e.col
	for n := 0; n < count; n++ {
		runes := e.lineRunes(row)
		if col >= len(runes) {
			if row < len(e.lines)-1 {
				row++
				col = 0
			}
			continue
		}
		ch := runes[col]
		if isWordChar(ch) {
			for col < len(runes) && isWordChar(runes[col]) {
				col++
			}
		} else if isPunct(ch) {
			for col < len(runes) && isPunct(runes[col]) {
				col++
			}
		}
		for {
			runes = e.lineRunes(row)
			for col < len(runes) && unicode.IsSpace(runes[col]) {
				col++
			}
			if col < len(runes) {
				break
			}
			if row >= len(e.lines)-1 {
				if len(runes) > 0 {
					col = len(runes) - 1
				}
				return row, col
			}
			row++
			col = 0
		}
	}
	return row, col
}

func (e *Editor) wordBackward(count int) (int, int) {
	row, col := e.row, e.col
	for n := 0; n < count; n++ {
		if col == 0 {
			if row > 0 {
				row--
				r := e.lineRunes(row)
				col = len(r)
				if col > 0 {
					col--
				}
			}
			runes := e.lineRunes(row)
			for col > 0 && unicode.IsSpace(runes[col]) {
				col--
			}
			if col == 0 && len(runes) > 0 && unicode.IsSpace(runes[0]) {
				continue
			}
		} else {
			col--
		}
		runes := e.lineRunes(row)
		for col > 0 && unicode.IsSpace(runes[col]) {
			col--
		}
		if col == 0 && len(runes) > 0 && unicode.IsSpace(runes[0]) {
			if row > 0 {
				row--
				runes = e.lineRunes(row)
				col = len(runes) - 1
				if col < 0 {
					col = 0
					continue
				}
				for col > 0 && unicode.IsSpace(runes[col]) {
					col--
				}
			}
		}
		runes = e.lineRunes(row)
		if col < len(runes) {
			if isWordChar(runes[col]) {
				for col > 0 && isWordChar(runes[col-1]) {
					col--
				}
			} else if isPunct(runes[col]) {
				for col > 0 && isPunct(runes[col-1]) {
					col--
				}
			}
		}
	}
	return row, col
}

func (e *Editor) wordEnd(count int) (int, int) {
	row, col := e.row, e.col
	for n := 0; n < count; n++ {
		col++
		for {
			runes := e.lineRunes(row)
			for col < len(runes) && unicode.IsSpace(runes[col]) {
				col++
			}
			if col < len(runes) {
				break
			}
			if row >= len(e.lines)-1 {
				col = len(runes) - 1
				if col < 0 {
					col = 0
				}
				return row, col
			}
			row++
			col = 0
		}
		runes := e.lineRunes(row)
		if col < len(runes) {
			if isWordChar(runes[col]) {
				for col+1 < len(runes) && isWordChar(runes[col+1]) {
					col++
				}
			} else if isPunct(runes[col]) {
				for col+1 < len(runes) && isPunct(runes[col+1]) {
					col++
				}
			}
		}
	}
	return row, col
}

func (e *Editor) paragraphForward(count int) int {
	row := e.row
	for n := 0; n < count; n++ {
		for row < len(e.lines)-1 && strings.TrimSpace(e.lines[row]) != "" {
			row++
		}
		for row < len(e.lines)-1 && strings.TrimSpace(e.lines[row]) == "" {
			row++
		}
	}
	return row
}

func (e *Editor) paragraphBack(count int) int {
	row := e.row
	for n := 0; n < count; n++ {
		if row > 0 {
			row--
		}
		for row > 0 && strings.TrimSpace(e.lines[row]) == "" {
			row--
		}
		for row > 0 && strings.TrimSpace(e.lines[row-1]) != "" {
			row--
		}
	}
	return row
}

func (e *Editor) sentenceForward(count int) (int, int) {
	row, col := e.row, e.col
	for n := 0; n < count; n++ {
		found := false
		for !found {
			runes := e.lineRunes(row)
			for col < len(runes) {
				if runes[col] == '.' || runes[col] == '!' || runes[col] == '?' {
					if col+1 >= len(runes) || unicode.IsSpace(runes[col+1]) {
						col++
						for col < len(runes) && unicode.IsSpace(runes[col]) {
							col++
						}
						if col < len(runes) {
							found = true
							break
						}
					}
				}
				col++
			}
			if !found {
				if row >= len(e.lines)-1 {
					col = 0
					found = true
				} else {
					row++
					col = 0
					if strings.TrimSpace(e.lines[row]) == "" {
						for row < len(e.lines)-1 && strings.TrimSpace(e.lines[row]) == "" {
							row++
						}
						col = e.firstNonBlank(row)
						found = true
					}
				}
			}
		}
	}
	return row, col
}

func (e *Editor) sentenceBack(count int) (int, int) {
	row, col := e.row, e.col
	for n := 0; n < count; n++ {
		if col > 0 {
			col--
		} else if row > 0 {
			row--
			r := e.lineRunes(row)
			col = len(r) - 1
			if col < 0 {
				col = 0
			}
		}
		found := false
		for !found {
			runes := e.lineRunes(row)
			for col >= 0 {
				if col < len(runes) && (runes[col] == '.' || runes[col] == '!' || runes[col] == '?') {
					sc := col + 1
					for sc < len(runes) && unicode.IsSpace(runes[sc]) {
						sc++
					}
					if sc < len(runes) {
						col = sc
						found = true
						break
					}
				}
				col--
			}
			if !found {
				if row <= 0 {
					row, col = 0, 0
					found = true
				} else {
					row--
					if strings.TrimSpace(e.lines[row]) == "" {
						nr := row + 1
						if nr < len(e.lines) {
							col = e.firstNonBlank(nr)
							row = nr
						} else {
							col = 0
						}
						found = true
					} else {
						runes = e.lineRunes(row)
						col = len(runes) - 1
					}
				}
			}
		}
	}
	return row, col
}

func (e *Editor) sectionForward(count int) int {
	row := e.row
	for n := 0; n < count; n++ {
		row++
		for row < len(e.lines) {
			if strings.HasPrefix(e.lines[row], "{") {
				break
			}
			row++
		}
		if row >= len(e.lines) {
			row = len(e.lines) - 1
		}
	}
	return row
}

func (e *Editor) sectionBack(count int) int {
	row := e.row
	for n := 0; n < count; n++ {
		row--
		for row > 0 {
			if strings.HasPrefix(e.lines[row], "{") {
				break
			}
			row--
		}
		if row < 0 {
			row = 0
		}
	}
	return row
}

// --- Search ---

func byteToRuneIdx(s string, byteIdx int) int {
	return len([]rune(s[:byteIdx]))
}

func (e *Editor) shouldIgnoreCase(pat string) bool {
	if strings.HasSuffix(pat, `\c`) {
		return true
	}
	if strings.HasSuffix(pat, `\C`) {
		return false
	}
	return e.ignoreCase
}

func (e *Editor) cleanPattern(pat string) string {
	if strings.HasSuffix(pat, `\c`) {
		return pat[:len(pat)-2]
	}
	if strings.HasSuffix(pat, `\C`) {
		return pat[:len(pat)-2]
	}
	return pat
}

func (e *Editor) indexOf(haystack, needle string) int {
	if e.shouldIgnoreCase(e.searchPat) {
		i, _ := foldIndex(haystack, needle)
		return i
	}
	return strings.Index(haystack, needle)
}

func (e *Editor) lastIndexOf(haystack, needle string) int {
	if e.shouldIgnoreCase(e.searchPat) {
		i, _ := foldLastIndex(haystack, needle)
		return i
	}
	return strings.LastIndex(haystack, needle)
}

func (e *Editor) setReg(name byte, text []string, isLine bool) {
	if name >= 'A' && name <= 'Z' {
		lower := name - 'A' + 'a'
		existing, ok := e.regs[lower]
		if ok {
			existing.text = append(existing.text, text...)
			existing.isLine = existing.isLine || isLine
		} else {
			e.regs[lower] = &regContent{text: text, isLine: isLine}
		}
		return
	}
	e.regs[name] = &regContent{text: text, isLine: isLine}
}

func (e *Editor) getReg(name byte) ([]string, bool, bool) {
	if name >= 'A' && name <= 'Z' {
		name = name - 'A' + 'a'
	}
	r, ok := e.regs[name]
	if !ok {
		return nil, false, false
	}
	return r.text, r.isLine, true
}

func (e *Editor) yankToReg(text []string, isLine bool) {
	e.register = text
	e.regLine = isLine
	if e.hasReg {
		e.setReg(e.pendingReg, text, isLine)
		e.hasReg = false
		e.pendingReg = 0
	}
	e.setReg('0', text, isLine)
}

func (e *Editor) deleteToReg(text []string, isLine bool) {
	e.register = text
	e.regLine = isLine
	if e.hasReg {
		e.setReg(e.pendingReg, text, isLine)
		e.hasReg = false
		e.pendingReg = 0
	} else if isLine || (len(text) == 1 && len(text[0]) > 1) {
		for i := byte('9'); i > '1'; i-- {
			if r, ok := e.regs[i-1]; ok {
				e.regs[i] = r
			}
		}
		e.regs['1'] = &regContent{text: text, isLine: isLine}
	} else {
		e.regs['-'] = &regContent{text: text, isLine: isLine}
	}
}

// --- File I/O ---

func (e *Editor) save() error {
	if e.filename == "" {
		return fmt.Errorf("no filename")
	}
	content := strings.Join(e.lines, "\n") + "\n"
	dir := filepath.Dir(e.filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(e.filename, []byte(content), 0o644); err != nil {
		return err
	}
	e.savedEdit()
	return nil
}

// --- Command Mode ---

func (e *Editor) handleCommand(msg tea.KeyPressMsg) tea.Cmd {
	key := typedKey(msg)
	switch key {
	case "esc", "ctrl+[":
		e.mode = ModeNormal
		e.cmdBuf = ""
	case "enter":
		cmd := e.cmdBuf
		e.cmdBuf = ""
		e.mode = ModeNormal
		e.executeCommand(cmd)
	case "backspace", "ctrl+h":
		if len(e.cmdBuf) > 0 {
			e.cmdBuf = dropLastRune(e.cmdBuf)
		} else {
			e.mode = ModeNormal
		}
	default:
		if len([]rune(key)) == 1 {
			e.cmdBuf += key
		}
	}
	return nil
}

// --- Standalone ---

type standaloneModel struct {
	editor *Editor
}

func (m *standaloneModel) Init() tea.Cmd { return nil }

func (m *standaloneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.editor.Update(msg)
	if m.editor.Quit {
		return m, tea.Quit
	}
	return m, cmd
}

func (m *standaloneModel) View() tea.View {
	v := tea.NewView(m.editor.View())
	v.AltScreen = true
	return v
}

func RunStandalone(filename, dir string) error {
	ed := New(filename, dir)
	p := tea.NewProgram(&standaloneModel{editor: ed})
	_, err := p.Run()
	return err
}
