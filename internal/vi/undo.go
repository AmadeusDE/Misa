package vi

import "slices"

const maxUndo = 200

type undoEntry struct {
	lines    []string
	row, col int
	revision uint64
}

func sameLines(a, b []string) bool { return slices.Equal(a, b) }

func (e *Editor) snapshot() undoEntry {
	return undoEntry{lines: slices.Clone(e.lines), row: e.row, col: e.col, revision: e.revision}
}

// beginEdit and commitEdit nest; only the outermost commit publishes a change.
func (e *Editor) beginEdit() {
	if e.editDepth == 0 {
		e.editBefore = e.snapshot()
		e.editPosSet = false
	}
	e.editDepth++
}

// A successful write can expose an intermediate transaction state. Give it
// an identity without splitting the transaction's undo unit.
func (e *Editor) materializeEdit() {
	if e.editDepth == 0 {
		return
	}
	if sameLines(e.editBefore.lines, e.lines) {
		e.revision = e.editBefore.revision
	} else if e.revision == e.editBefore.revision ||
		(e.editSavedLines != nil && e.revision == e.editSavedRevision && !sameLines(e.editSavedLines, e.lines)) {
		e.nextRevision++
		e.revision = e.nextRevision
	}
}

func (e *Editor) savedEdit() {
	e.materializeEdit()
	e.savedRevision = e.revision
	e.modified = false
	if e.editDepth != 0 {
		e.editSavedLines = slices.Clone(e.lines)
		e.editSavedRevision = e.revision
	}
}

func (e *Editor) noteEdit(row, col int) {
	if e.editDepth == 0 {
		return
	}
	if !e.editPosSet || row < e.editPos.row || (row == e.editPos.row && col < e.editPos.col) {
		e.editPos, e.editPosSet = pos{row, col}, true
	}
}

func (e *Editor) saveUndo() {
	if e.editDepth == 0 {
		e.beginEdit()
		e.editAuto = true
	}
}

func changedPosition(a, b []string) (int, int) {
	n := min(len(a), len(b))
	for row := 0; row < n; row++ {
		if a[row] != b[row] {
			x, y := []rune(a[row]), []rune(b[row])
			col := 0
			for col < len(x) && col < len(y) && x[col] == y[col] {
				col++
			}
			return row, col
		}
	}
	if n > 0 {
		return min(n, len(a)-1), 0
	}
	return 0, 0
}

func (e *Editor) commitEdit() {
	if e.editDepth == 0 {
		return
	}
	if e.editDepth > 1 {
		e.editDepth--
		return
	}
	e.materializeEdit()
	e.editDepth = 0
	before := e.editBefore
	e.editBefore = undoEntry{}
	e.editSavedLines = nil
	if sameLines(before.lines, e.lines) {
		e.revision = before.revision
		e.modified = e.revision != e.savedRevision
		return
	}
	row, col := changedPosition(before.lines, e.lines)
	if e.editPosSet {
		row, col = e.editPos.row, e.editPos.col
	}
	before.row, before.col = row, col
	if !e.lineUndoSet || e.lineUndoRow != row {
		e.lineUndoSet = row < len(before.lines)
		e.lineUndoRow = row
		if e.lineUndoSet {
			e.lineUndoText = before.lines[row]
		}
	}
	e.undoStack = append(e.undoStack, before)
	if len(e.undoStack) > maxUndo {
		e.undoStack[0] = undoEntry{}
		e.undoStack = e.undoStack[1:]
	}
	e.redoStack = nil
	e.modified = e.revision != e.savedRevision
}

func (e *Editor) abortEdit() {
	if e.editDepth == 0 {
		return
	}
	b := e.editBefore
	e.lines, e.row, e.col, e.revision = b.lines, b.row, b.col, b.revision
	e.editDepth, e.editAuto = 0, false
	e.editBefore = undoEntry{}
	e.editSavedLines = nil
	e.modified = e.revision != e.savedRevision
	e.clampCursor()
}

func (e *Editor) resetHistory() {
	e.undoStack, e.redoStack = nil, nil
	e.editDepth, e.editAuto = 0, false
	e.editBefore = undoEntry{}
	e.editSavedLines = nil
	e.nextRevision++
	e.revision, e.savedRevision = e.nextRevision, e.nextRevision
	e.modified, e.lineUndoSet = false, false
}

func (e *Editor) undo() {
	if len(e.undoStack) == 0 {
		e.statusMsg = "Already at oldest change"
		return
	}
	u := e.undoStack[len(e.undoStack)-1]
	e.undoStack[len(e.undoStack)-1] = undoEntry{}
	e.undoStack = e.undoStack[:len(e.undoStack)-1]
	cur := e.snapshot()
	cur.row, cur.col = u.row, u.col
	e.redoStack = append(e.redoStack, cur)
	e.lines, e.row, e.col, e.revision = u.lines, u.row, u.col, u.revision
	e.modified = e.revision != e.savedRevision
	e.clampCursor()
}

func (e *Editor) redo() {
	if len(e.redoStack) == 0 {
		e.statusMsg = "Already at newest change"
		return
	}
	r := e.redoStack[len(e.redoStack)-1]
	e.redoStack[len(e.redoStack)-1] = undoEntry{}
	e.redoStack = e.redoStack[:len(e.redoStack)-1]
	cur := e.snapshot()
	cur.row, cur.col = r.row, r.col
	e.undoStack = append(e.undoStack, cur)
	e.lines, e.row, e.col, e.revision = r.lines, r.row, r.col, r.revision
	e.modified = e.revision != e.savedRevision
	e.clampCursor()
}

func (e *Editor) undoLine() {
	if !e.lineUndoSet || e.row != e.lineUndoRow || e.row >= len(e.lines) {
		e.statusMsg = "No line changes to undo"
		return
	}
	e.saveUndo()
	e.setLine(e.row, e.lineUndoText)
	e.col = 0
	e.clampCursor()
}

// finishEditEvent closes implicit edits but leaves insert sessions and explicit
// outer transactions open. Macro replay invokes it for each replayed key too.
func (e *Editor) finishEditEvent(oldRow int) {
	if e.editAuto && e.mode != ModeInsert && e.mode != ModeReplace {
		e.editAuto = false
		e.commitEdit()
	} else if e.editAuto {
		e.materializeEdit()
		e.modified = e.revision != e.savedRevision
	}
	if e.lineUndoSet && oldRow != e.row && e.row != e.lineUndoRow && e.editDepth == 0 {
		e.lineUndoSet = false
	}
}
