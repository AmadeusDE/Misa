package vi

import (
	"reflect"
	"strings"
	"testing"
)

func TestSubstituteConfirmAcceptedPreservesDot(t *testing.T) {
	e := newUndoF05Editor(t, "abc cat cat\n")
	sendUndoF05(t, e, "ylx:s/cat/dog/gc<CR>yq0.")
	assertUndoF05(t, e, []string{"c dog cat"}, true)
}

func TestSubstituteConfirmGlobalCommandRejected(t *testing.T) {
	// :global's nested-command execution is synchronous; confirmation cannot
	// suspend it. A c in the replacement is not the confirmation flag.
	for _, tc := range []struct{ name, command, want string }{
		{"confirmation flag", ":g/cat/s#cat#dog#c<CR>", "cat"},
		{"replacement only", ":g/cat/s#cat#c#<CR>", "c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newUndoF05Editor(t, "cat\ncat\n")
			sendUndoF05(t, e, tc.command)
			if !reflect.DeepEqual(e.lines, []string{tc.want, tc.want}) {
				t.Fatalf("buffer = %q", e.lines)
			}
			if tc.name == "confirmation flag" {
				if e.mode != ModeNormal || e.modified || len(e.undoStack) != 0 || e.row != 0 || e.col != 0 {
					t.Errorf("rejection changed state: mode=%v dirty=%v undo=%d cursor=(%d,%d)", e.mode, e.modified, len(e.undoStack), e.row, e.col)
				}
				if !strings.Contains(e.statusMsg, "not supported") {
					t.Errorf("missing explicit rejection: %q", e.statusMsg)
				}
			}
		})
	}
}

func TestSubstituteConfirmMacroPlaybackParity(t *testing.T) {
	keys := ":%s/cat/dog/gc<CR>ynl"
	for _, macro := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "macro"}[macro], func(t *testing.T) {
			e := newUndoF05Editor(t, "cat cat\ncat cat\n")
			if macro {
				r := &regContent{}
				for _, k := range replayKeys(t, keys) {
					r.keys = append(r.keys, typedKey(k))
				}
				e.regs['a'] = r
				sendUndoF05(t, e, "@a")
			} else {
				sendUndoF05(t, e, keys)
			}
			assertUndoF05(t, e, []string{"dog cat", "dog cat"}, true)
			if e.mode != ModeNormal || e.row != 1 || e.col != 0 {
				t.Errorf("mode=%v cursor=(%d,%d)", e.mode, e.row, e.col)
			}
			sendUndoF05(t, e, "u")
			assertUndoF05(t, e, []string{"cat cat", "cat cat"}, false)
		})
	}
}
