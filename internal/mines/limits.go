package mines

import "fmt"

// Custom level limits shared by the command line and interactive settings.
const (
	MinWidth  = 2
	MaxWidth  = 99
	MinHeight = 2
	MaxHeight = 99
	MinMines  = 1
	MaxMines  = 9999
)

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
