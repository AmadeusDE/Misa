package vi

import "strings"

// parseRange preserves the historical clamped range semantics for non-write callers.
func (e *Editor) parseRange(cmd string) (from, to int, rest string, hasRange bool) {
	from, to, rest, hasRange = e.parseRawRange(cmd)
	from, to = e.normalizeRange(from, to)
	return
}

func (e *Editor) normalizeRange(from, to int) (int, int) {
	if from < 0 {
		from = 0
	}
	if to < 0 {
		to = 0
	}
	if from >= len(e.lines) {
		from = len(e.lines) - 1
	}
	if to >= len(e.lines) {
		to = len(e.lines) - 1
	}
	if from > to {
		from, to = to, from
	}

	return from, to
}

// Saturation keeps arbitrarily long addresses and offset chains from wrapping.
func rangeOffset(value, sign, amount int) int {
	// Once an address saturates, don't let later offsets wrap it back into
	// the buffer and accidentally turn an invalid write into a valid one.
	if value >= maxCommandCount || value <= -maxCommandCount || amount >= maxCommandCount {
		if value <= -maxCommandCount || value < maxCommandCount && sign < 0 {
			return -maxCommandCount
		}
		return maxCommandCount
	}
	if sign > 0 {
		if value > maxCommandCount-amount {
			return maxCommandCount
		}
		return value + amount
	}
	if value < -maxCommandCount+amount {
		return -maxCommandCount
	}
	return value - amount
}

func (e *Editor) parseRawRange(cmd string) (from, to int, rest string, hasRange bool) {
	from = e.row
	to = e.row
	i := 0

	parseLine := func() (int, bool) {
		if i >= len(cmd) {
			return -1, false
		}
		switch cmd[i] {
		case '.':
			i++
			return e.row, true
		case '$':
			i++
			return len(e.lines) - 1, true
		case '%':
			i++
			from = 0
			to = len(e.lines) - 1
			hasRange = true
			return -1, false
		case '\'':
			if i+1 < len(cmd) {
				i += 2
				ch := cmd[i-1]
				if p, ok := e.marks[ch]; ok {
					return p.row, true
				}
			}
			return -1, false
		default:
			if cmd[i] >= '0' && cmd[i] <= '9' {
				n := 0
				for i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '9' {
					n = countDigit(n, int(cmd[i]-'0'))
					i++
				}
				if n == maxCommandCount {
					return maxCommandCount, true
				}
				return n - 1, true
			}
		}
		return -1, false
	}

	if i < len(cmd) && cmd[i] == '%' {
		i++
		from = 0
		to = len(e.lines) - 1
		hasRange = true
		rest = strings.TrimSpace(cmd[i:])
		return
	}

	if line, ok := parseLine(); ok {
		from = line
		to = line
		hasRange = true

		for i < len(cmd) && (cmd[i] == '+' || cmd[i] == '-') {
			sign := 1
			if cmd[i] == '-' {
				sign = -1
			}
			i++
			n := 0
			for i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '9' {
				n = countDigit(n, int(cmd[i]-'0'))
				i++
			}
			if n == 0 {
				n = 1
			}
			from = rangeOffset(from, sign, n)
			to = from
		}

		if i < len(cmd) && cmd[i] == ',' {
			i++
			if line2, ok := parseLine(); ok {
				to = line2
			}
			for i < len(cmd) && (cmd[i] == '+' || cmd[i] == '-') {
				sign := 1
				if cmd[i] == '-' {
					sign = -1
				}
				i++
				n := 0
				for i < len(cmd) && cmd[i] >= '0' && cmd[i] <= '9' {
					n = countDigit(n, int(cmd[i]-'0'))
					i++
				}
				if n == 0 {
					n = 1
				}
				to = rangeOffset(to, sign, n)
			}
		}
	}

	rest = strings.TrimSpace(cmd[i:])
	return
}
