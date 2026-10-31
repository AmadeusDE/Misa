package vi

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestEditT05Expansion(t *testing.T) {
	for _, tc := range []struct{ name, arg string }{{"home", "~/target"}, {"env", "$T05_DIR/target"}, {"braces", "${T05_DIR}/target"}, {"percent suffix", "% .copy"}} {
		t.Run(tc.name, func(t *testing.T) {
			e, first := f07Editor(t, "first\n")
			dir := filepath.Dir(first)
			e.shellEnv = []string{"HOME=" + dir, "T05_DIR=" + dir}
			target := filepath.Join(dir, "target")
			if tc.name == "percent suffix" {
				target = first + " .copy"
			}
			t05File(t, target, "expanded\n")
			f07Keys(t, e, ":e "+tc.arg+"<CR>")
			if e.filename != target || !reflect.DeepEqual(e.lines, []string{"expanded"}) {
				t.Errorf("expansion: %q %q status=%q", e.filename, e.lines, e.statusMsg)
			}
		})
	}
}

func TestEditT05ReadFailurePreservesState(t *testing.T) {
	e, first := f07Editor(t, "abc\nsecond\n")
	other := filepath.Join(filepath.Dir(first), "other")
	t05File(t, other, "other\n")
	f07Keys(t, e, ":e "+other+"<CR>:e #<CR>jx")
	beforeLines := append([]string(nil), e.lines...)
	beforeFile, beforeName, beforeAlt := e.file, e.filename, e.alternateFilename
	beforeRow, beforeCol := e.row, e.col
	beforeUndo, beforeRedo := len(e.undoStack), len(e.redoStack)
	f07Keys(t, e, ":e! "+filepath.Dir(first)+"<CR>")
	if !reflect.DeepEqual(e.lines, beforeLines) || !reflect.DeepEqual(e.file, beforeFile) || e.filename != beforeName || e.alternateFilename != beforeAlt || e.row != beforeRow || e.col != beforeCol || len(e.undoStack) != beforeUndo || len(e.redoStack) != beforeRedo || !e.modified || e.statusMsg == "" {
		t.Errorf("failure changed state: name=%q alt=%q lines=%q cursor=%d,%d status=%q", e.filename, e.alternateFilename, e.lines, e.row, e.col, e.statusMsg)
	}
	f07Keys(t, e, "u")
	if !reflect.DeepEqual(e.lines, []string{"abc", "second"}) {
		t.Errorf("undo lost on failure: %q", e.lines)
	}
}

func TestEditT05SuccessClearsHistoryPreservesRegisterAndDot(t *testing.T) {
	e, first := f07Editor(t, "abcd\n")
	other := filepath.Join(filepath.Dir(first), "other")
	t05File(t, other, "wxyz\n")
	f07Keys(t, e, "xxu:e! "+other+"<CR>u<C-r>")
	if !reflect.DeepEqual(e.lines, []string{"wxyz"}) || e.modified {
		t.Errorf("history leaked: %q dirty=%v", e.lines, e.modified)
	}
	reg, _ := e.Register()
	if !reflect.DeepEqual(reg, []string{"b"}) {
		t.Errorf("register = %q", reg)
	}
	f07Keys(t, e, ".")
	if !reflect.DeepEqual(e.lines, []string{"xyz"}) {
		t.Errorf("dot not preserved: %q", e.lines)
	}
}

func TestEditT05MarksAndJumpsCleared(t *testing.T) {
	e, first := f07Editor(t, "abc\ndef\n")
	other := filepath.Join(filepath.Dir(first), "other")
	t05File(t, other, "xyz\n")
	f07Keys(t, e, "majG:e "+other+"<CR>")
	if len(e.marks) != 0 || len(e.jumpList) != 0 {
		t.Errorf("marks/jumps retained: %v %v", e.marks, e.jumpList)
	}
}
