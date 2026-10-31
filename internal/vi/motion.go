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
	case "w", "b", "e", "{", "}", "(", ")":
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
	case "w":
		r, c := e.wordForward(count)
		return motionResult{r, c, rangeExclusive}, true
	case "b":
		r, c := e.wordBackward(count)
		return motionResult{r, c, rangeExclusive}, true
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
	case "H":
		r := e.topLine
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
	case "M":
		r := e.topLine + e.height/2
		if r >= len(e.lines) {
			r = len(e.lines) - 1
		}
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
	case "L":
		r := e.topLine + e.height - 1
		if r >= len(e.lines) {
			r = len(e.lines) - 1
		}
		return motionResult{r, e.firstNonBlank(r), rangeLinewise}, true
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
	case "{":
		r := e.paragraphBack(count)
		return motionResult{r, 0, rangeLinewise}, true
	case "}":
		r := e.paragraphForward(count)
		return motionResult{r, 0, rangeLinewise}, true
	case "(":
		r, c := e.sentenceBack(count)
		return motionResult{r, c, rangeExclusive}, true
	case ")":
		r, c := e.sentenceForward(count)
		return motionResult{r, c, rangeExclusive}, true
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
