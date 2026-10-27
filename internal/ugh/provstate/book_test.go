package provstate

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/roots"
)

func TestBooksWithDifferentRootsAreSeparate(t *testing.T) {
	a := For(roots.Roots{State: filepath.Join(t.TempDir(), "a")})
	b := For(roots.Roots{State: filepath.Join(t.TempDir(), "b")})
	a.Mark("codex", State{Kind: "exhausted", Until: time.Now().Add(time.Hour), Reason: "limit"})
	if !a.Exhausted("codex") {
		t.Fatal("a lost its mark")
	}
	if b.Exhausted("codex") {
		t.Fatal("b saw a's mark")
	}
}
