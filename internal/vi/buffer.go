package vi

// bufferIdentity groups one file's text, absolute name, disk state and history.
// Switching files replaces this boundary only after the candidate read succeeds.
type bufferIdentity struct {
	lines    []string
	filename string
	file     fileState
	bufferHistory
}

type bufferHistory struct {
	modified                              bool
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
	lineUndoRow                           int
	lineUndoText                          string
	lineUndoSet                           bool
}

func (e *Editor) replaceBuffer(b bufferIdentity) {
	previous, nextRevision := e.filename, e.nextRevision
	e.bufferIdentity = b
	if previous != e.filename {
		e.alternateFilename = previous
	}
	e.nextRevision = nextRevision
	e.resetHistory()
	e.row, e.col, e.topLine = 0, 0, 0
	e.goalCol, e.goalValid, e.goalEnd = 0, false, false
	e.marks = make(map[byte]pos)
	e.jumpList, e.jumpIdx = nil, 0
	e.visual = visualNone
}
