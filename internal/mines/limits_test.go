package mines

import (
	"strings"
	"testing"
)

func TestValidateLevel(t *testing.T) {
	for _, tt := range []struct {
		name  string
		level Level
		want  string
	}{
		{"minimum", Level{W: MinWidth, H: MinHeight, Mines: MinMines}, ""},
		{"maximum", Level{W: MaxWidth, H: MaxHeight, Mines: MaxMines}, ""},
		// A 999×999 board is legal: it is played through the viewport.
		{"largest board", Level{W: 999, H: 999, Mines: 1000}, ""},
		// Counts exceeding board capacity remain legal; NewBoard clamps them.
		{"crowded", Level{W: 2, H: 2, Mines: 9999}, ""},
		{"narrow", Level{W: 1, H: 2, Mines: 1}, "--width must be 2-999"},
		{"wide", Level{W: 1000, H: 2, Mines: 1}, "--width must be 2-999"},
		{"flat", Level{W: 2, H: 1, Mines: 1}, "--height must be 2-999"},
		{"tall", Level{W: 2, H: 1000, Mines: 1}, "--height must be 2-999"},
		{"no mines", Level{W: 2, H: 2, Mines: 0}, "--mines must be 1-998000"},
		{"too many mines", Level{W: 2, H: 2, Mines: MaxMines + 1}, "--mines must be 1-998000"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLevel(tt.level)
			if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Errorf("ValidateLevel(%+v) = %v, want %q", tt.level, err, tt.want)
			}
		})
	}
	if MaxMines != MaxWidth*MaxHeight-1 {
		t.Errorf("MaxMines = %d, want one safe cell of %d×%d", MaxMines, MaxWidth, MaxHeight)
	}
	if got := NewBoard(2, 2, 9999, nil).Mines(); got != 3 {
		t.Errorf("NewBoard clamped mines = %d, want 3", got)
	}
}

func TestPercentMines(t *testing.T) {
	for _, tt := range []struct {
		name      string
		w, h, pct int
		want      int
	}{
		{"exact", 20, 10, 15, 30},
		{"rounds up", 9, 9, 12, 10},          // 9.72 mines
		{"rounds down", 30, 16, 21, 101},     // 100.8 mines
		{"half rounds up", 2, 5, 15, 2},      // 1.5 mines
		{"huge board", 999, 999, 90, 898201}, // 898200.9 mines, one safe cell left
		{"one mine floor", 2, 2, MinPercent, MinMines},
		{"safe cell ceiling", 2, 2, MaxPercent, 3},
		{"single cell", 1, 1, 15, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := PercentMines(tt.w, tt.h, tt.pct); got != tt.want {
				t.Errorf("PercentMines(%d, %d, %d) = %d, want %d", tt.w, tt.h, tt.pct, got, tt.want)
			}
		})
	}
	n := 0
	for _, w := range []int{2, 3, 9, 16, 30, 99, 999} {
		for _, h := range []int{2, 3, 9, 16, 30, 99, 999} {
			for pct := MinPercent; pct <= MaxPercent; pct++ {
				got := PercentMines(w, h, pct)
				if got < MinMines || got > w*h-1 {
					t.Fatalf("PercentMines(%d, %d, %d) = %d, outside 1..%d", w, h, pct, got, w*h-1)
				}
				n++
			}
		}
	}
	if n == 0 {
		t.Fatal("bounds check covered no boards")
	}
}

func TestDensityPercent(t *testing.T) {
	for _, tt := range []struct {
		name        string
		w, h, mines int
		want        int
	}{
		{"expert", 30, 16, 99, 21}, // 20.6% rounds up
		{"exact", 20, 10, 30, 15},  // 15%
		{"beginner", 9, 9, 10, 12}, // 12.3% rounds down
		{"sparse floors", 999, 999, 1, MinPercent},
		{"dense clamps", 2, 2, 9999, MaxPercent},
		{"three of four", 2, 2, 3, 75},
		{"no cells", 0, 0, 5, DefaultPercent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := DensityPercent(tt.w, tt.h, tt.mines); got != tt.want {
				t.Errorf("DensityPercent(%d, %d, %d) = %d, want %d", tt.w, tt.h, tt.mines, got, tt.want)
			}
		})
	}
	for _, w := range []int{2, 3, 9, 16, 30, 999} {
		for _, h := range []int{2, 3, 9, 16, 30, 999} {
			for _, mines := range []int{0, 1, 10, w * h / 2, w * h, MaxMines} {
				got := DensityPercent(w, h, mines)
				if got < MinPercent || got > MaxPercent {
					t.Fatalf("DensityPercent(%d, %d, %d) = %d, outside %d..%d", w, h, mines, got, MinPercent, MaxPercent)
				}
			}
		}
	}
	if DefaultPercent < MinPercent || DefaultPercent > MaxPercent {
		t.Errorf("DefaultPercent = %d, outside %d..%d", DefaultPercent, MinPercent, MaxPercent)
	}
}

func TestValidatePercent(t *testing.T) {
	for _, tt := range []struct {
		pct  int
		want string
	}{
		{0, "--percent must be 1-90"},
		{MinPercent, ""},
		{45, ""},
		{MaxPercent, ""},
		{91, "--percent must be 1-90"},
		{-5, "--percent must be 1-90"},
		{100, "--percent must be 1-90"},
	} {
		err := ValidatePercent(tt.pct)
		if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
			t.Errorf("ValidatePercent(%d) = %v, want %q", tt.pct, err, tt.want)
		}
	}
	if MinPercent != 1 || MaxPercent != 90 {
		t.Errorf("percent range = %d..%d, want 1..90", MinPercent, MaxPercent)
	}
}
