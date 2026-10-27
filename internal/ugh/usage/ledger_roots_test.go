package usage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/AmadeusDE/Misa/internal/ugh/roots"
)

func TestLedgersWithDifferentRootsAreSeparate(t *testing.T) {
	a := roots.Roots{State: filepath.Join(t.TempDir(), "a")}
	b := roots.Roots{State: filepath.Join(t.TempDir(), "b")}
	NewLedger(a).Record("openrouter", 0.5)
	NewLedger(a).RecordRoute("openrouter/m", "root", 10, 2, 0, 0.5)
	NewLedger(b).Record("openrouter", 0.25)

	if got, calls := NewLedger(a).Spent("openrouter"); got != 0.5 || calls != 1 {
		t.Fatalf("a spent %v/%d", got, calls)
	}
	if got, calls := NewLedger(b).Spent("openrouter"); got != 0.25 || calls != 1 {
		t.Fatalf("b spent %v/%d", got, calls)
	}
	if n := len(NewLedger(a).RouteStats(time.Time{})); n != 1 {
		t.Fatalf("a routes %d", n)
	}
	if n := len(NewLedger(b).RouteStats(time.Time{})); n != 0 {
		t.Fatalf("b saw a's routes: %d", n)
	}
}
