package mines

import (
	"strings"
	"testing"
)

func TestValidateLevel(t *testing.T) {
	for _, tt := range []struct {
		level Level
		want  string
	}{
		{Level{W: MinWidth, H: MinHeight, Mines: MinMines}, ""},
		{Level{W: MaxWidth, H: MaxHeight, Mines: MaxMines}, ""},
		// Counts exceeding board capacity remain legal; NewBoard clamps them.
		{Level{W: 2, H: 2, Mines: 9999}, ""},
		{Level{W: 1, H: 2, Mines: 1}, "--width must be 2-99"},
		{Level{W: 100, H: 2, Mines: 1}, "--width must be 2-99"},
		{Level{W: 2, H: 1, Mines: 1}, "--height must be 2-99"},
		{Level{W: 2, H: 100, Mines: 1}, "--height must be 2-99"},
		{Level{W: 2, H: 2, Mines: 0}, "--mines must be 1-9999"},
		{Level{W: 2, H: 2, Mines: 10000}, "--mines must be 1-9999"},
	} {
		err := ValidateLevel(tt.level)
		if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
			t.Errorf("ValidateLevel(%+v) = %v, want %q", tt.level, err, tt.want)
		}
	}
	if got := NewBoard(2, 2, 9999, nil).Mines(); got != 3 {
		t.Errorf("NewBoard clamped mines = %d, want 3", got)
	}
}
