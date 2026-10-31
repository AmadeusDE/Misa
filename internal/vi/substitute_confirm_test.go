package vi

import (
	"reflect"
	"testing"
)

// All input goes through Editor.Update via the F05 replay helpers. Match
// positions are deliberately not asserted after completion: these tests focus
// on the decisions, ranges, and transaction boundaries of confirmation.
func TestSubstituteConfirmDecisions(t *testing.T) {
	cases := []struct {
		name, input, keys string
		want              []string
	}{
		{"mixed", "cat cat cat\n", ":s/cat/dog/gc<CR>nyn", []string{"cat dog cat"}},
		{"accept rest", "cat cat\ncat cat\n", ":%s/cat/dog/gc<CR>na", []string{"cat dog", "dog dog"}},
		{"quit after acceptance", "cat cat cat\n", ":s/cat/dog/gc<CR>yq", []string{"dog cat cat"}},
		{"last", "cat cat cat\n", ":s/cat/dog/gc<CR>nl", []string{"cat dog cat"}},
		{"escape", "cat cat\n", ":s/cat/dog/gc<CR>y<Esc>", []string{"dog cat"}},
		{"longer replacement", "aa aa aa\n", ":s/aa/aaaa/gc<CR>yny", []string{"aaaa aa aaaa"}},
		{"shorter replacement", "aaa aaa aaa\n", ":s/aaa/a/gc<CR>yny", []string{"a aaa a"}},
		{"empty replacement", "ab ab ab\n", ":s/ab//gc<CR>yny", []string{" ab "}},
		{"range first per line", "cat cat\ncat cat\ncat cat\ncat cat\n", ":2,3s/cat/dog/c<CR>yy", []string{"cat cat", "dog cat", "dog cat", "cat cat"}},
		{"visual range", "cat\ncat\ncat\ncat\n", "jVj:s/cat/dog/c<CR>yn", []string{"cat", "dog", "cat", "cat"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newUndoF05Editor(t, tc.input)
			sendUndoF05(t, e, tc.keys)
			assertUndoF05(t, e, tc.want, true)
		})
	}
}

func TestSubstituteConfirmTransaction(t *testing.T) {
	e := newUndoF05Editor(t, "cat cat\ncat cat\n")
	sendUndoF05(t, e, ":%s/cat/dog/gc<CR>yna")
	assertUndoF05(t, e, []string{"dog cat", "dog dog"}, true)
	// Undo the entire confirmed command.
	sendUndoF05(t, e, "u")
	assertUndoF05(t, e, []string{"cat cat", "cat cat"}, false)
	sendUndoF05(t, e, "<C-r>")
	assertUndoF05(t, e, []string{"dog cat", "dog dog"}, true)
}

func TestSubstituteConfirmNoChangePreservesHistory(t *testing.T) {
	for _, tc := range []struct{ name, command string }{
		{"reject all", ":s/a/z/gc<CR>nnn"},
		{"quit", ":s/a/z/gc<CR>q"},
		{"escape", ":s/a/z/gc<CR><Esc>"},
		{"no match", ":s/Q/z/gc<CR>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newUndoF05Editor(t, "aaa\n")
			sendUndoF05(t, e, "iX<Esc>u"+tc.command)
			assertUndoF05(t, e, []string{"aaa"}, false)
			sendUndoF05(t, e, "<C-r>")
			assertUndoF05(t, e, []string{"Xaaa"}, true)
		})
	}
}

func TestSubstituteConfirmModalIsolation(t *testing.T) {
	e := newUndoF05Editor(t, "cat cat\n")
	sendUndoF05(t, e, ":s/cat/dog/gc<CR>")
	assertUndoF05(t, e, []string{"cat cat"}, false)
	if e.mode == ModeNormal {
		t.Error("confirmation must own input until answered")
	}
	// x is not a confirmation answer and must not delete text or dismiss it.
	sendUndoF05(t, e, "x")
	assertUndoF05(t, e, []string{"cat cat"}, false)
	sendUndoF05(t, e, "ny")
	assertUndoF05(t, e, []string{"cat dog"}, true)
}

func TestSubstituteConfirmPreservesRegisterAndDot(t *testing.T) {
	e := newUndoF05Editor(t, "abc cat cat\n")
	sendUndoF05(t, e, "ylx") // yank a, then delete a; x is the repeatable edit
	reg, linewise := e.Register()
	reg = append([]string(nil), reg...)
	sendUndoF05(t, e, ":s/cat/dog/gc<CR>nq")
	got, gotLinewise := e.Register()
	if !reflect.DeepEqual(got, reg) || gotLinewise != linewise {
		t.Errorf("register = %q linewise=%v; want %q linewise=%v", got, gotLinewise, reg, linewise)
	}
	sendUndoF05(t, e, "0.")
	assertUndoF05(t, e, []string{"c cat cat"}, true)
}

func TestSubstituteOptionalTrailingDelimiter(t *testing.T) {
	e := newUndoF05Editor(t, "cat\n")
	sendUndoF05(t, e, ":s/cat/dog<CR>")
	assertUndoF05(t, e, []string{"dog"}, true)
}

func TestSubstituteConfirmCursorAndViewport(t *testing.T) {
	e := newUndoF05Editor(t, "  cat\n  cat\n  cat\n  cat\n  cat\n  cat\n")
	e.Resize(40, 4)
	sendUndoF05(t, e, ":%s/cat/dog/c<CR>")
	before := e.topLine
	e.Update(keyFromName("ctrl+e"))
	if e.topLine <= before {
		t.Error("Ctrl-E must scroll while confirmation remains active")
	}
	e.Update(keyFromName("ctrl+y"))
	if e.topLine != before {
		t.Error("Ctrl-Y must scroll back while confirmation remains active")
	}
	sendUndoF05(t, e, "nnyq")
	assertUndoF05(t, e, []string{"  cat", "  cat", "  dog", "  cat", "  cat", "  cat"}, true)
	if e.row != 2 || e.col != 2 {
		t.Errorf("cursor = (%d,%d); want first nonblank of last changed line (2,2)", e.row, e.col)
	}
}

func TestSubstituteConfirmRejectsNewlineReplacement(t *testing.T) {
	e := newUndoF05Editor(t, "cat cat\n")
	sendUndoF05(t, e, ":s/cat/\\n/gc<CR>")
	assertUndoF05(t, e, []string{"cat cat"}, false)
	if e.mode != ModeNormal {
		t.Error("unsupported newline replacement should be rejected, not prompt")
	}
}
