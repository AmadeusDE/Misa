package term

// charsets tracks G0/G1 designations and which one is invoked into GL (SO/SI).
// Only US ASCII and DEC Special Graphics (line drawing) are distinguished.
type charsets struct {
	g0, g1 bool // true = DEC Special Graphics
	gl     int
}

func (cs *charsets) designate(target, final byte) {
	graphics := final == '0'
	switch target {
	case '(':
		cs.g0 = graphics
	case ')':
		cs.g1 = graphics
	}
}

// graphics reports whether the invoked set is DEC Special Graphics.
func (cs *charsets) graphics() bool {
	if cs.gl == 1 {
		return cs.g1
	}
	return cs.g0
}

func (cs *charsets) translate(r rune) rune {
	if !cs.graphics() || r < '_' || r > '~' {
		return r
	}
	return decSpecialGraphics[r-'_']
}

var decSpecialGraphics = [...]rune{
	' ', '◆', '▒', '␉', '␌', '␍', '␊', '°', '±', '␤', '␋', '┘', '┐', '┌', '└', '┼',
	'⎺', '⎻', '─', '⎼', '⎽', '├', '┤', '┴', '┬', '│', '≤', '≥', 'π', '≠', '£', '·',
}
