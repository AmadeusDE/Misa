package vi

import "testing"

func TestUndoF05SaveInOuterTransaction(t *testing.T) {
	t.Run("save final edit", func(t *testing.T) {
		e := newUndoF05Editor(t, "abcd\n")
		e.beginEdit()
		sendUndoF05(t, e, "x:w<CR>")
		e.commitEdit()
		assertUndoF05(t, e, []string{"bcd"}, false)
		sendUndoF05(t, e, "u")
		assertUndoF05(t, e, []string{"abcd"}, true)
		sendUndoF05(t, e, "<C-r>")
		assertUndoF05(t, e, []string{"bcd"}, false)
	})
	t.Run("edit after save", func(t *testing.T) {
		e := newUndoF05Editor(t, "abcd\n")
		e.beginEdit()
		sendUndoF05(t, e, "x:w<CR>x")
		e.commitEdit()
		assertUndoF05(t, e, []string{"cd"}, true)
		sendUndoF05(t, e, "u")
		assertUndoF05(t, e, []string{"abcd"}, true)
		sendUndoF05(t, e, "<C-r>")
		assertUndoF05(t, e, []string{"cd"}, true)
	})
	t.Run("abort after save", func(t *testing.T) {
		e := newUndoF05Editor(t, "abcd\n")
		e.beginEdit()
		sendUndoF05(t, e, "x:w<CR>")
		e.abortEdit()
		assertUndoF05(t, e, []string{"abcd"}, true)
		sendUndoF05(t, e, "u<C-r>")
		assertUndoF05(t, e, []string{"abcd"}, true)
	})
}
