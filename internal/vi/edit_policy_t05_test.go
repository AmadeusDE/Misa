package vi

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEditPolicyT05CancelAndUnnamed(t *testing.T) {
	e, first := f07Editor(t, "abc\n")
	f07Keys(t, e, ":e another<Esc>")
	if e.filename != first || e.alternateFilename != "" || !reflect.DeepEqual(e.lines, []string{"abc"}) {
		t.Errorf("cancel changed buffer: %q %q %q", e.filename, e.alternateFilename, e.lines)
	}
	for _, cmd := range []string{":e", ":e!", ":e %", ":e #"} {
		t.Run(cmd, func(t *testing.T) {
			e := New("", t.TempDir())
			e.Resize(80, 24)
			f07Keys(t, e, cmd+"<CR>")
			if e.filename != "" || e.alternateFilename != "" || !reflect.DeepEqual(e.lines, []string{""}) || e.statusMsg == "" {
				t.Errorf("unnamed: name=%q alt=%q lines=%q status=%q", e.filename, e.alternateFilename, e.lines, e.statusMsg)
			}
		})
	}
}

func TestEditPolicyT05ReloadAndEmptyWrites(t *testing.T) {
	e, first := f07Editor(t, "first\n")
	other := filepath.Join(filepath.Dir(first), "other")
	t05File(t, other, "other\n")
	f07Keys(t, e, ":e "+other+"<CR>:e #<CR>")
	t05File(t, first, "new\r\nbytes")
	f07Keys(t, e, ":e<CR>:w<CR>")
	if e.filename != first || e.alternateFilename != other || !reflect.DeepEqual(e.lines, []string{"new", "bytes"}) || f07Read(t, first) != "new\r\nbytes" {
		t.Errorf("reload: %q alt=%q lines=%q status=%q", e.filename, e.alternateFilename, e.lines, e.statusMsg)
	}
	for _, existing := range []bool{true, false} {
		t.Run(map[bool]string{true: "empty existing", false: "missing"}[existing], func(t *testing.T) {
			e, first := f07Editor(t, "old\n")
			target := filepath.Join(filepath.Dir(first), "empty")
			if existing {
				t05File(t, target, "")
			}
			f07Keys(t, e, ":e "+target+"<CR>:w<CR>")
			b, err := os.ReadFile(target)
			if err != nil || len(b) != 0 {
				t.Errorf("zero bytes: %q err=%v status=%q", b, err, e.statusMsg)
			}
		})
	}
}

func TestEditPolicyT05RejectedForms(t *testing.T) {
	for _, cmd := range []string{":1e other", ":e +2 other", ":e ++enc=utf8 other", ":e %:h", ":e ~someone/file", ":e $MISSING/child", ":e ${EMPTY}/child", ":e a${oops", ":e $5", ":e $-", ":e ${bad-name}", ":e $$", ":e dangling$"} {
		t.Run(cmd, func(t *testing.T) {
			e, first := f07Editor(t, "abc\n")
			e.shellEnv = []string{"HOME=" + filepath.Dir(first), "EMPTY="}
			f07Keys(t, e, cmd+"<CR>")
			if e.filename != first || e.alternateFilename != "" || !reflect.DeepEqual(e.lines, []string{"abc"}) || e.statusMsg == "" {
				t.Errorf("rejected %q: name=%q alt=%q lines=%q status=%q", cmd, e.filename, e.alternateFilename, e.lines, e.statusMsg)
			}
		})
	}
}

func TestEditPolicyT05GlobalRejectsEdit(t *testing.T) {
	e, first := f07Editor(t, "xpat abc\n")
	f07Keys(t, e, "x:g/pat/e!<CR>")
	if e.filename != first || !reflect.DeepEqual(e.lines, []string{"pat abc"}) || !e.modified || len(e.undoStack) != 1 {
		t.Errorf("global edit: %q %q undo=%d status=%q", e.filename, e.lines, len(e.undoStack), e.statusMsg)
	}
	f07Keys(t, e, "u")
	if !reflect.DeepEqual(e.lines, []string{"xpat abc"}) {
		t.Errorf("undo changed: %q", e.lines)
	}
}

func TestEditPolicyT05AlternateFromFileCommands(t *testing.T) {
	for _, tc := range []struct{ command, alt string }{{":file renamed<CR>", "original"}, {":w written<CR>", "written"}, {":saveas saved<CR>", "original"}} {
		t.Run(tc.command, func(t *testing.T) {
			dir := t.TempDir()
			first := filepath.Join(dir, "original")
			t05File(t, first, "abc\n")
			e := New(first, dir)
			e.Resize(80, 24)
			f07Keys(t, e, tc.command+":e #<CR>")
			want := filepath.Join(dir, tc.alt)
			if e.filename != want || !reflect.DeepEqual(e.lines, []string{"abc"}) {
				t.Errorf("alternate: got=%q want=%q lines=%q status=%q", e.filename, want, e.lines, e.statusMsg)
			}
		})
	}
}

func TestEditPolicyT05NoRecursiveExpansion(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "name$UNSET#%.txt")
	other := filepath.Join(dir, "other")
	t05File(t, first, "first\n")
	t05File(t, other, "other\n")
	e := New(first, dir)
	e.Resize(80, 24)
	e.shellEnv = []string{}
	f07Keys(t, e, ":e %<CR>:e "+other+"<CR>:e #<CR>")
	if e.filename != first || !reflect.DeepEqual(e.lines, []string{"first"}) {
		t.Errorf("recursive expansion: name=%q lines=%q status=%q", e.filename, e.lines, e.statusMsg)
	}
}

func TestEditPolicyT05MacroReplay(t *testing.T) {
	e, first := f07Editor(t, "first\n")
	other := filepath.Join(filepath.Dir(first), "other")
	t05File(t, other, "second\n")
	// Preloaded keys exercise replay without relying on cross-mode macro recording (H09).
	var keys []string
	for _, k := range replayKeys(t, ":e other<CR>") {
		keys = append(keys, typedKey(k))
	}
	e.regs['a'] = &regContent{keys: keys}
	f07Keys(t, e, "@a")
	if e.filename != other || !reflect.DeepEqual(e.lines, []string{"second"}) || strings.Contains(e.statusMsg, "Not an editor") {
		t.Errorf("macro edit: name=%q lines=%q status=%q", e.filename, e.lines, e.statusMsg)
	}
}

func TestEditPolicyT05ReadFailureHistory(t *testing.T) {
	for _, kind := range []string{"directory", "not-directory", "permission"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "permission" && os.Geteuid() == 0 {
				t.Skip("root bypasses read permissions")
			}
			e, first := f07Editor(t, "abcd\nsecond\n")
			other := filepath.Join(filepath.Dir(first), "other")
			t05File(t, other, "other\n")
			f07Keys(t, e, ":e "+other+"<CR>:e #<CR>majGggxxu")
			target := filepath.Dir(first)
			switch kind {
			case "not-directory":
				target = filepath.Join(first, "child")
			case "permission":
				target = other
				if err := os.Chmod(target, 0000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(target, 0600); err != nil {
						t.Error(err)
					}
				})
			}
			beforeUndo := append([]undoEntry(nil), e.undoStack...)
			beforeRedo := append([]undoEntry(nil), e.redoStack...)
			beforeMarks := make(map[byte]pos, len(e.marks))
			for k, v := range e.marks {
				beforeMarks[k] = v
			}
			beforeJumps := append([]pos(nil), e.jumpList...)
			beforeFile, beforeName, beforeAlt := e.file, e.filename, e.alternateFilename
			beforeLines := append([]string(nil), e.lines...)
			row, col, jumpIdx := e.row, e.col, e.jumpIdx
			rev, saved, next := e.revision, e.savedRevision, e.nextRevision
			if len(beforeUndo) == 0 || len(beforeRedo) == 0 || len(beforeMarks) == 0 || len(beforeJumps) == 0 {
				t.Fatal("fixture lacks undo, redo, marks or jumps")
			}
			f07Keys(t, e, ":e! "+target+"<CR>")
			if !reflect.DeepEqual(e.undoStack, beforeUndo) || !reflect.DeepEqual(e.redoStack, beforeRedo) || !reflect.DeepEqual(e.marks, beforeMarks) || !reflect.DeepEqual(e.jumpList, beforeJumps) || e.jumpIdx != jumpIdx || e.revision != rev || e.savedRevision != saved || e.nextRevision != next || !reflect.DeepEqual(e.file, beforeFile) || e.filename != beforeName || e.alternateFilename != beforeAlt || !reflect.DeepEqual(e.lines, beforeLines) || e.row != row || e.col != col || !e.modified || e.statusMsg == "" {
				t.Errorf("failed read changed state: file=%q alt=%q lines=%q cursor=%d,%d undo=%v redo=%v marks=%v jumps=%v revisions=%d/%d/%d status=%q", e.filename, e.alternateFilename, e.lines, e.row, e.col, e.undoStack, e.redoStack, e.marks, e.jumpList, e.revision, e.savedRevision, e.nextRevision, e.statusMsg)
			}
		})
	}
}
