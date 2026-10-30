package builtins

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/battleship"
)

func TestBattleshipGame(t *testing.T) {
	for _, args := range [][]string{nil, {"-r"}, {"--random"}} {
		g, err := BattleshipGame(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if g == nil {
			t.Fatalf("%v: nil game", args)
		}
	}
}

// TestBattleshipGamePlacement checks that -r really lays the fleet out instead
// of starting a hand placement.
func TestBattleshipGamePlacement(t *testing.T) {
	g, err := BattleshipGame(nil)
	if err != nil {
		t.Fatal(err)
	}
	if g.Phase() != battleship.Placing {
		t.Errorf("phase = %v, want placing", g.Phase())
	}
	if n := len(g.Player().Ships()); n != 0 {
		t.Errorf("%d ships placed before the player had a turn", n)
	}
	for _, args := range [][]string{{"-r"}, {"--random"}} {
		g, err := BattleshipGame(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if g.Phase() != battleship.Firing {
			t.Errorf("%v: phase = %v, want firing", args, g.Phase())
		}
		if n := len(g.Player().Ships()); n != len(battleship.Fleet) {
			t.Errorf("%v: %d ships placed, want %d", args, n, len(battleship.Fleet))
		}
	}
}

func TestBattleshipGameRejects(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"-x"}, {"stray"}} {
		if _, err := BattleshipGame(args); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}
