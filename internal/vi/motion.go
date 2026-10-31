package vi

// --- Motion destination (used by normal mode & operators) ---

type rangeKind uint8

const (
	rangeExclusive rangeKind = iota
	rangeInclusive
	rangeLinewise
)

type motionResult struct {
	row, col int
	kind     rangeKind
}

func (e *Editor) motionDest(key string, count int, countGiven bool) (motionResult, bool) {
	switch key {
	case "w", "b", "e", "W", "B", "E", "ge", "gE", "{", "}", "(", ")":
		count = e.motionLimit(count)
	}
	switch key {
	case "h", "left":
		c := e.col - count
		if c < 0 {
			c = 0
		}
		return motionResult{e.row, c, rangeExclusive}, true
	case "l", "right":
		runes := e.lineRunes(e.row)
		mx := len(runes) - 1
		if mx < 0 {
			mx = 0
		}
		c := e.col + count
		if c > mx {
			c = mx
		}
		return motionResult{e.row, c, rangeExclusive}, true
	case "j", "down":
		r := e.row + count
		if r >= len(e.lines) {
			r = len(e.lines) - 1
		}
		return motionResult{r, e.col, rangeLinewise}, true
	case "k", "up":
		r := e.row - count
		if r < 0 {
			r = 0
		}
		return motionResult{r, e.col, rangeLinewise}, true
	case "w", "b", "W", "B", "E", "ge", "gE":
		big := key == "W" || key == "B" || key == "E" || key == "gE"
		kind := key[0]
		if kind >= 'A' && kind <= 'Z' {
			kind += 'a' - 'A'
		}
		r, c := e.wordMotion(count, big, kind)
		rangeKind := rangeExclusive
		if kind == 'g' && r == e.row && c == e.col {
			return motionResult{}, false
		}
		if kind == 'e' || kind == 'g' {
			rangeKind = rangeInclusive
		}
		return motionResult{r, c, rangeKind}, true
	case "e":
		r, c := e.wordEnd(count)
		return motionResult{r, c, rangeInclusive}, true
	case "$", "end":
		r := min(e.row+count-1, len(e.lines)-1)
		runes := e.lineRunes(r)
		c := len(runes) - 1
		if c < 0 {
			c = 0
		}
		return motionResult{r, c, rangeInclusive}, true
	case "0":
		return motionResult{e.row, 0, rangeExclusive}, true
	case "home":
		return motionResult{e.row, 0, rangeExclusive}, true
	case "^":
		return motionResult{e.row, e.firstNonBlank(e.row), rangeExclusive}, true
	case "_":
		r := min(e.row+count-1, len(e.lines)-1)
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
	case "H", "M", "L":
		return e.screenLineMotion(key, count), true
	case "|":
		return e.columnMotion(count), true
	case "+", "enter":
		r := e.row + count
		if r >= len(e.lines) {
			r = len(e.lines) - 1
		}
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
	case "-":
		r := e.row - count
		if r < 0 {
			r = 0
		}
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
	case "{", "}":
		return e.paragraphMotion(count, key == "}")
	case "(", ")":
		return e.sentenceMotion(count, key == ")")
	case "G":
		if countGiven {
			r := count - 1
			if r >= len(e.lines) {
				r = len(e.lines) - 1
			}
			if r < 0 {
				r = 0
			}
			return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
		}
		r := len(e.lines) - 1
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
	case "gg":
		r := 0
		if countGiven {
			r = count - 1
		}
		if r >= len(e.lines) {
			r = len(e.lines) - 1
		}
		if r < 0 {
			r = 0
		}
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
	}
	return motionResult{}, false
}
