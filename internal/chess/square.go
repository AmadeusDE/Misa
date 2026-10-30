package chess

import (
	"fmt"
	"strconv"
)

// formatSquare uses bijective base 26 for files: a through z, then aa, ab,
// and so on. Like Variant.SquareName, it expects an on-board square.
func formatSquare(v *Variant, sq int) string {
	x, y := v.XY(sq)
	return fileName(x) + strconv.Itoa(y+1)
}

// fileName returns the spreadsheet-style name of a nonnegative file index.
func fileName(x int) string {
	var letters [32]byte // enough for an int-sized file index
	i := len(letters)
	for {
		i--
		letters[i] = byte('a' + x%26)
		x = x/26 - 1
		if x < 0 {
			break
		}
	}
	return string(letters[i:])
}

func parseSquare(v *Variant, s string) (int, error) {
	i, file := 0, 0
	for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
		digit := int(s[i]-'a') + 1
		// Reject before multiplying, including arbitrarily long file names.
		if digit > v.Files || file > (v.Files-digit)/26 {
			return 0, fmt.Errorf("square %q is not on the %d×%d board", s, v.Files, v.Ranks)
		}
		file = file*26 + digit
		i++
	}
	if i == 0 || i == len(s) {
		return 0, fmt.Errorf("bad square %q", s)
	}
	rank, err := strconv.Atoi(s[i:])
	if err != nil || !v.OnBoard(file-1, rank-1) {
		return 0, fmt.Errorf("square %q is not on the %d×%d board", s, v.Files, v.Ranks)
	}
	return v.Square(file-1, rank-1), nil
}
