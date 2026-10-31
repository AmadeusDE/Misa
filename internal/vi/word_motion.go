package vi

import "unicode"

// wordClass treats an empty line as a separate word, as vi's word motions do.
func (e *Editor) wordClass(row, col int, big bool) int {
	r := e.lineRunes(row)
	if len(r) == 0 {
		return 3
	}
	if unicode.IsSpace(r[col]) {
		return 0
	}
	if big || isWordChar(r[col]) {
		return 1
	}
	return 2
}

func (e *Editor) wordStep(row, col, dir int) (int, int, bool) {
	if dir > 0 {
		if col+1 < len(e.lineRunes(row)) {
			return row, col + 1, true
		}
		if row+1 < len(e.lines) {
			return row + 1, 0, true
		}
	} else {
		if col > 0 {
			return row, col - 1, true
		}
		if row > 0 {
			row--
			return row, max(0, len(e.lineRunes(row))-1), true
		}
	}
	return row, col, false
}

func (e *Editor) wordMotion(count int, big bool, kind byte) (int, int) {
	row, col := e.row, e.col
	for n := 0; n < count; n++ {
		switch kind {
		case 'w':
			class := e.wordClass(row, col, big)
			if class != 0 && class != 3 {
				for {
					nr, nc, ok := e.wordStep(row, col, 1)
					if !ok {
						return row, col
					}
					if nr != row || e.wordClass(nr, nc, big) != class {
						row, col = nr, nc
						break
					}
					row, col = nr, nc
				}
			} else if class == 3 {
				nr, nc, ok := e.wordStep(row, col, 1)
				if !ok {
					return row, col
				}
				row, col = nr, nc
			}
			for e.wordClass(row, col, big) == 0 {
				nr, nc, ok := e.wordStep(row, col, 1)
				if !ok {
					return row, col
				}
				row, col = nr, nc
			}
		case 'b':
			nr, nc, ok := e.wordStep(row, col, -1)
			if !ok {
				return row, col
			}
			row, col = nr, nc
			for e.wordClass(row, col, big) == 0 {
				nr, nc, ok = e.wordStep(row, col, -1)
				if !ok {
					return row, col
				}
				row, col = nr, nc
			}
			class := e.wordClass(row, col, big)
			if class != 3 {
				for {
					nr, nc, ok = e.wordStep(row, col, -1)
					if !ok || nr != row || e.wordClass(nr, nc, big) != class {
						break
					}
					row, col = nr, nc
				}
			}
		case 'e':
			nr, nc, ok := e.wordStep(row, col, 1)
			if !ok {
				return row, col
			}
			row, col = nr, nc
			for e.wordClass(row, col, big) == 0 {
				nr, nc, ok = e.wordStep(row, col, 1)
				if !ok {
					return row, col
				}
				row, col = nr, nc
			}
			class := e.wordClass(row, col, big)
			if class != 3 {
				for {
					nr, nc, ok = e.wordStep(row, col, 1)
					if !ok || nr != row || e.wordClass(nr, nc, big) != class {
						break
					}
					row, col = nr, nc
				}
			}
		case 'g':
			class := e.wordClass(row, col, big)
			if class != 0 && class != 3 {
				for {
					nr, nc, ok := e.wordStep(row, col, -1)
					if !ok {
						return row, col
					}
					if nr != row || e.wordClass(nr, nc, big) != class {
						row, col = nr, nc
						break
					}
					row, col = nr, nc
				}
			} else {
				nr, nc, ok := e.wordStep(row, col, -1)
				if !ok {
					return row, col
				}
				row, col = nr, nc
			}
			for e.wordClass(row, col, big) == 0 {
				nr, nc, ok := e.wordStep(row, col, -1)
				if !ok {
					return row, col
				}
				row, col = nr, nc
			}
		}
	}
	return row, col
}
