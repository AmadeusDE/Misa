package builtins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChessGame(t *testing.T) {
	tests := []struct {
		args []string
		id   string
	}{
		{nil, "chess"},
		{[]string{"double"}, "double"},
		{[]string{"big"}, "bigboard"},
		{[]string{"--fen", "k7/8/8/8/8/8/8/K7 b"}, "chess"},
	}
	for _, tt := range tests {
		g, err := ChessGame(tt.args)
		if err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		if got := g.Match().Variant().ID; got != tt.id {
			t.Errorf("%v: variant %s, want %s", tt.args, got, tt.id)
		}
	}
}

func TestChessGameFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mini.yaml")
	src := "id: mini\nname: Mini\nextends: chess\nfiles: 5\nranks: 5\nsetup: rnbqk/ppppp/5/PPPPP/RNBQK\ncastling: []\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := ChessGame([]string{"-f", path})
	if err != nil {
		t.Fatal(err)
	}
	if v := g.Match().Variant(); v.ID != "mini" || v.Files != 5 {
		t.Errorf("loaded %s %d×%d", v.ID, v.Files, v.Ranks)
	}
}

func TestChessGameRejects(t *testing.T) {
	for _, args := range [][]string{{"nope"}, {"a", "b"}, {"--list"}, {"--help"}, {"--fen", "zz"}, {"-x"}, {"-f", "/nonexistent.yaml"}, {"-c", "red"}, {"--think", "0"}} {
		if _, err := ChessGame(args); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}

func TestChessGameComputerSide(t *testing.T) {
	for _, args := range [][]string{{"-c", "none"}, {"--computer=white", "--think", "1"}, {"double", "-c", "both"}} {
		if _, err := ChessGame(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
}
