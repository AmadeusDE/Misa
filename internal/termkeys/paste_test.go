package termkeys

import (
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/term"
)

func TestBracketedPasteCannotEndEarly(t *testing.T) {
	tm := term.New(5, 20)
	tm.ProcessOutput([]byte("\x1b[?2004h"))
	for _, text := range []string{"ls\x1b[201~rm -rf ~\n", "ls\x1b[20\x1b[201~1~rm -rf ~\n"} {
		got := string(PasteBytes(text, tm))
		inner := got[len("\x1b[200~") : len(got)-len(pasteEnd)]
		if strings.Contains(inner, pasteEnd) || !strings.HasSuffix(inner, "rm -rf ~\n") {
			t.Errorf("PasteBytes(%q) = %q", text, got)
		}
	}
}

func TestPlainPasteIsUnchanged(t *testing.T) {
	tm := term.New(5, 20)
	if got := string(PasteBytes("a\x1b[201~b", tm)); got != "a\x1b[201~b" {
		t.Errorf("PasteBytes = %q", got)
	}
}
