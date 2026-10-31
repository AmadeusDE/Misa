package vi

import "testing"

func TestUndoF05DiscardedSnapshotReferences(t *testing.T) {
	e := newUndoF05Editor(t, "abc\n")
	backing := make([]undoEntry, maxUndo+2)
	e.undoStack = backing[:0]
	for i := 0; i <= maxUndo; i++ {
		e.beginEdit()
		e.lines[0] += "x"
		e.commitEdit()
	}
	if len(e.undoStack) != maxUndo {
		t.Fatalf("undo entries=%d, want %d", len(e.undoStack), maxUndo)
	}
	if backing[0].lines != nil {
		t.Error("evicted snapshot remains referenced by the undo backing array")
	}
	popped := &e.undoStack[len(e.undoStack)-1]
	e.undo()
	if popped.lines != nil {
		t.Error("popped undo snapshot remains referenced")
	}
	popped = &e.redoStack[len(e.redoStack)-1]
	e.redo()
	if popped.lines != nil {
		t.Error("popped redo snapshot remains referenced")
	}
}
