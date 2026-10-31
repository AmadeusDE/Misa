package vi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUndoF05Transactions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transaction.txt")
	if err := os.WriteFile(path, []byte("abc\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.beginEdit()
	e.lines[0] = "Abc"
	e.beginEdit()
	e.lines[0] = "ABc"
	e.commitEdit()
	e.commitEdit()
	e.undo()
	if e.lines[0] != "abc" || e.modified {
		t.Fatalf("nested edit undo: %q dirty=%v", e.lines, e.modified)
	}
	e.beginEdit()
	e.commitEdit()
	e.redo()
	if e.lines[0] != "ABc" || !e.modified {
		t.Fatalf("no-op commit must preserve redo: %q dirty=%v", e.lines, e.modified)
	}
	e.beginEdit()
	e.lines[0] = "discard"
	e.abortEdit()
	if e.lines[0] != "ABc" || !e.modified {
		t.Fatalf("abort: %q dirty=%v", e.lines, e.modified)
	}
	e.undo()
	if e.lines[0] != "abc" || e.modified {
		t.Fatalf("abort must not add history: %q dirty=%v", e.lines, e.modified)
	}
}

// Loading replacement text must start a fresh, clean history; :e! itself is T05.
func TestUndoF05ResetHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reset.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New(path, dir)
	e.beginEdit()
	e.lines[0] = "edited"
	e.commitEdit()
	e.beginEdit()
	e.lines[0] = "later"
	e.commitEdit()
	e.undo() // Both undo and redo histories now exist.
	previousRevision := e.revision
	e.lines = []string{"loaded"}
	e.resetHistory()
	if e.modified || e.revision != e.savedRevision || e.revision == previousRevision {
		t.Fatalf("reset identity: dirty=%v revision=%d saved=%d previous=%d", e.modified, e.revision, e.savedRevision, previousRevision)
	}
	e.undo()
	e.redo()
	if len(e.lines) != 1 || e.lines[0] != "loaded" || e.modified {
		t.Fatalf("reset must discard undo and redo: lines=%q dirty=%v", e.lines, e.modified)
	}
	e.beginEdit()
	e.lines[0] = "new"
	e.commitEdit()
	e.undo()
	if len(e.lines) != 1 || e.lines[0] != "loaded" || e.modified {
		t.Fatalf("undo after reset must return to clean loaded state: lines=%q dirty=%v", e.lines, e.modified)
	}
}
