package vi

// Quoted text objects: i" a" i' a' i` a`.
//
// A quoted string never leaves its line. A quote preceded by an odd number of
// backslashes is escaped and is ordinary text.
//
// With the cursor on a quote, the quotes of the line are paired from its start
// to tell an opening quote from a closing one. Anywhere else the string is
// taken from the closest quote before the cursor to the closest one after it,
// whether or not they pair up, and a cursor before the first quote uses the
// first string after it.
//
// i" excludes the quotes, and with a count of two or more includes them. a"
// includes them and the white space after the closing quote or, when there is
// none, the white space in front of the opening one.

// quoteMode selects what part of a quoted string an object covers.
type quoteMode uint8

const (
	quoteInner  quoteMode = iota // between the quotes
	quoteQuotes                  // the quotes and what they enclose
	quoteAround                  // the quotes plus surrounding white space
)

// isQuote reports whether runes[i] is an unescaped quote.
func isQuote(runes []rune, i int, q rune) bool {
	if runes[i] != q {
		return false
	}
	slashes := 0
	for j := i - 1; j >= 0 && runes[j] == '\\'; j-- {
		slashes++
	}
	return slashes%2 == 0
}

// nextQuote finds the first unescaped quote at or after from.
func nextQuote(runes []rune, from int, q rune) (int, bool) {
	for i := max(from, 0); i < len(runes); i++ {
		if isQuote(runes, i, q) {
			return i, true
		}
	}
	return 0, false
}

// prevQuote finds the last unescaped quote before before.
func prevQuote(runes []rune, before int, q rune) (int, bool) {
	for i := min(before, len(runes)) - 1; i >= 0; i-- {
		if isQuote(runes, i, q) {
			return i, true
		}
	}
	return 0, false
}

// quoteBounds finds the opening and closing quote of the string at col.
func quoteBounds(runes []rune, col int, q rune) (start, end int, ok bool) {
	if col < len(runes) && isQuote(runes, col, q) {
		before := 0
		for i := 0; i < col; i++ {
			if isQuote(runes, i, q) {
				before++
			}
		}
		if before%2 == 0 {
			end, ok = nextQuote(runes, col+1, q)
			return col, end, ok
		}
		start, ok = prevQuote(runes, col, q)
		return start, col, ok
	}
	if start, ok = prevQuote(runes, col, q); ok {
		end, ok = nextQuote(runes, col+1, q)
		return start, end, ok
	}
	if start, ok = nextQuote(runes, col, q); !ok {
		return 0, 0, false
	}
	end, ok = nextQuote(runes, start+1, q)
	return start, end, ok
}

// quoteRange turns the quotes of a string into the half-open columns of an
// object.
func quoteRange(runes []rune, start, end int, mode quoteMode) (from, to int) {
	switch mode {
	case quoteInner:
		return start + 1, end
	case quoteQuotes:
		return start, end + 1
	}
	from, to = start, end+1
	for to < len(runes) && isBlank(runes[to]) {
		to++
	}
	if to > end+1 {
		return from, to
	}
	for from > 0 && isBlank(runes[from-1]) {
		from--
	}
	return from, to
}

// isBlank reports a space or tab.
func isBlank(r rune) bool { return r == ' ' || r == '\t' }

// quoteObject computes the object of the string around col.
func quoteObject(runes []rune, col int, q rune, mode quoteMode) (from, to int, ok bool) {
	start, end, ok := quoteBounds(runes, col, q)
	if !ok {
		return 0, 0, false
	}
	from, to = quoteRange(runes, start, end, mode)
	return from, to, true
}
