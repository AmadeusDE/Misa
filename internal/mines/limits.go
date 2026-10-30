package mines

import "fmt"

// Custom level limits shared by the command line and interactive settings.
// A board larger than the terminal is played through the scrolling viewport.
const (
	MinWidth  = 2
	MaxWidth  = 999
	MinHeight = 2
	MaxHeight = 999
	MinMines  = 1
	MaxMines  = MaxWidth*MaxHeight - 1
)

// The custom setup screen sets mines as a percent of the board's cells.
const (
	MinPercent     = 1
	MaxPercent     = 90
	DefaultPercent = 15
)

// PercentMines is the mine count for a percent of the board's cells: the
// percentage rounded to the nearest mine, kept between one mine and one safe
// cell.
func PercentMines(w, h, percent int) int {
	cells := w * h
	if cells <= 1 {
		return 0
	}
	mines := (cells*percent + 50) / 100
	return max(MinMines, min(mines, cells-1))
}

// DensityPercent is the percentage of cells that hold a mine, rounded and
// clamped to the custom screen's range.
func DensityPercent(w, h, mines int) int {
	cells := w * h
	if cells <= 0 {
		return DefaultPercent
	}
	percent := (mines*100 + cells/2) / cells
	return max(MinPercent, min(MaxPercent, percent))
}

// ValidatePercent checks the custom screen's mine percentage.
func ValidatePercent(percent int) error {
	if percent < MinPercent || percent > MaxPercent {
		return fmt.Errorf("--percent must be %d-%d", MinPercent, MaxPercent)
	}
	return nil
}

// ValidateLevel checks the custom level's per-field limits. A mine count above
// the board's capacity is accepted: NewBoard clamps it to leave one safe cell.
func ValidateLevel(level Level) error {
	for _, field := range []struct {
		name            string
		value, min, max int
	}{
		{"width", level.W, MinWidth, MaxWidth},
		{"height", level.H, MinHeight, MaxHeight},
		{"mines", level.Mines, MinMines, MaxMines},
	} {
		if field.value < field.min || field.value > field.max {
			return fmt.Errorf("--%s must be %d-%d", field.name, field.min, field.max)
		}
	}
	return nil
}
