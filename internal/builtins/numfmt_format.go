// Behaviour adapted from uutils coreutils numfmt (MIT).
package builtins

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type numfmtFormat struct {
	present, zero, grouping bool
	width, precision        int
	prefix, suffix          string
}

func numfmtQuote(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			fmt.Fprintf(&b, `\%03o`, s[0])
			s = s[1:]
			continue
		}
		s = s[size:]
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		default:
			b.WriteRune(r)
		}
	}
	return "'" + b.String() + "'"
}

func numfmtParseFormat(s string) (numfmtFormat, error) {
	f := numfmtFormat{present: true, precision: -1}
	q := numfmtQuote(s)
	i := strings.IndexByte(s, '%')
	if i < 0 {
		return f, fmt.Errorf("format %s has no %% directive", q)
	}
	if strings.Contains(s[i+1:], "%") {
		return f, fmt.Errorf("format %s has too many %% directives", q)
	}
	if i == len(s)-1 {
		return f, fmt.Errorf("format %s ends in %%", q)
	}
	f.prefix = s[:i]
	i++
	negative := false
	for i < len(s) {
		c := s[i]
		if c == '0' {
			f.zero = true
		} else if c == '\'' {
			f.grouping = true
		} else if c == '-' {
			negative = true
		} else if c != ' ' {
			break
		}
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > start {
		width, e := strconv.ParseUint(s[start:i], 10, 64)
		if e != nil || width > 1000000 {
			return f, fmt.Errorf("invalid format %s (width overflow)", q)
		}
		f.width = int(width)
		if negative {
			f.width = -f.width
		}
	}
	if i < len(s) && s[i] == '.' {
		i++
		start = i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if start == i {
			return f, fmt.Errorf("invalid precision in format %s", q)
		}
		p, e := strconv.ParseUint(s[start:i], 10, 64)
		if e != nil {
			return f, fmt.Errorf("invalid precision in format %s", q)
		}
		f.precision = int(uint16(p))
	}
	if i >= len(s) || s[i] != 'f' {
		return f, fmt.Errorf("invalid format %s, directive must be %%[0]['][-][N][.][N]f", q)
	}
	f.suffix = s[i+1:]
	return f, nil
}

func numfmtPow(base int64, power int) *big.Rat {
	return new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(base), big.NewInt(int64(power)), nil))
}
func numfmtAbs(n *big.Rat) *big.Rat { return new(big.Rat).Abs(n) }

func numfmtRound(n *big.Rat, precision int, method string) *big.Rat {
	factor := numfmtPow(10, precision)
	v := new(big.Rat).Mul(n, factor)
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(v.Num(), v.Denom(), r)
	if r.Sign() != 0 {
		increment := false
		switch method {
		case "from-zero":
			increment = true
		case "up":
			increment = v.Sign() > 0
		case "down":
			increment = v.Sign() < 0
		case "nearest", "even":
			c := new(big.Int).Lsh(new(big.Int).Abs(r), 1).Cmp(v.Denom())
			increment = c > 0 || c == 0 && (method == "nearest" || q.Bit(0) == 1)
		}
		if increment {
			q.Add(q, big.NewInt(int64(v.Sign())))
		}
	}
	return new(big.Rat).Quo(new(big.Rat).SetInt(q), factor)
}

func (o *numfmtOptions) parse(s string) (*big.Rat, int, error) {
	original := s
	if o.suffix != "" {
		s = strings.TrimSuffix(s, o.suffix)
	}
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	invalidNumber := func() (*big.Rat, int, error) { return nil, 0, fmt.Errorf("invalid number: %s", numfmtQuote(original)) }
	invalidSuffix := func() (*big.Rat, int, error) {
		return nil, 0, fmt.Errorf("invalid suffix in input: %s", numfmtQuote(original))
	}
	if s == "" || s[0] == '+' {
		return invalidNumber()
	}
	i := 0
	if s[0] == '-' {
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	digits := i - start
	precision := 0
	if i < len(s) && s[i] == '.' {
		i++
		fractionStart := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		precision = i - fractionStart
		if precision == 0 {
			if i < len(s) && s[i] == '.' {
				return invalidSuffix()
			}
			return invalidNumber()
		}
		digits += precision
	}
	if digits == 0 {
		if strings.HasPrefix(s, "..") {
			return invalidSuffix()
		}
		return invalidNumber()
	}
	value, ok := new(big.Rat).SetString(s[:i])
	if !ok {
		return invalidNumber()
	}
	tail := s[i:]
	if tail != "" && o.separator != "" && strings.HasPrefix(tail, o.separator) {
		tail = tail[len(o.separator):]
	}
	hasSuffix := tail != ""
	if hasSuffix {
		idx := strings.IndexByte("KMGTPEZYRQ", tail[0])
		if tail[0] == 'k' {
			idx = 0
		}
		if idx < 0 {
			return invalidSuffix()
		}
		if o.from == "none" {
			return nil, 0, fmt.Errorf("rejecting suffix in input: %s (consider using --from)", numfmtQuote(original))
		}
		rest := tail[1:]
		base := int64(1000)
		if o.from == "iec-i" {
			if !strings.HasPrefix(rest, "i") {
				return nil, 0, fmt.Errorf("missing 'i' suffix in input: %s (e.g Ki/Mi/Gi)", numfmtQuote(original))
			}
			rest = rest[1:]
			base = 1024
		} else if o.from == "iec" {
			base = 1024
		} else if o.from == "auto" && strings.HasPrefix(rest, "i") {
			rest = rest[1:]
			base = 1024
		}
		if rest != "" {
			return nil, 0, fmt.Errorf("invalid suffix in input %s: %s", numfmtQuote(original), numfmtQuote(rest))
		}
		value.Mul(value, numfmtPow(base, idx+1))
		precision = 0
	}
	value.Mul(value, o.fromUnit)
	if hasSuffix {
		value = numfmtRound(value, 0, "from-zero")
	}
	value.Quo(value, o.toUnit)
	return value, precision, nil
}

func numfmtPad(s string, width int, zero bool) string {
	count := width
	if count < 0 {
		count = -count
	}
	count -= utf8.RuneCountInString(s)
	if count <= 0 {
		return s
	}
	if width < 0 {
		return s + strings.Repeat(" ", count)
	}
	if zero {
		if strings.HasPrefix(s, "-") {
			return "-" + strings.Repeat("0", count) + s[1:]
		}
		return strings.Repeat("0", count) + s
	}
	return strings.Repeat(" ", count) + s
}

func (o *numfmtOptions) number(s string, implicit int) (string, error) {
	n, inputPrecision, err := o.parse(s)
	if err != nil {
		return "", err
	}
	precision := inputPrecision
	if o.format.precision >= 0 {
		precision = o.format.precision
	}
	scaled := o.to != "none"
	suffix := ""
	negativeZero := false
	if scaled {
		base := int64(1000)
		if o.to != "si" {
			base = 1024
		}
		abs := numfmtAbs(n)
		exponent := 0
		if abs.Cmp(big.NewRat(base-1, 1)) > 0 {
			exponent = 1
			for exponent < 10 && abs.Cmp(numfmtPow(base, exponent+1)) >= 0 {
				exponent++
			}
		}
		if abs.Cmp(numfmtPow(base, 10+1)) >= 0 {
			return "", fmt.Errorf("number too big to be printed")
		}
		if exponent == 0 {
			method := "even"
			if o.format.precision >= 0 {
				method = o.round
			}
			negativeZero = n.Sign() < 0
			n = numfmtRound(n, 0, method)
			if o.format.precision < 0 {
				precision = 0
			}
		} else {
			n.Quo(n, numfmtPow(base, exponent))
			if o.format.precision < 0 {
				precision = 0
				if numfmtAbs(n).Cmp(big.NewRat(10, 1)) < 0 {
					precision = 1
				}
			}
			effective := precision
			if o.to != "si" && effective > 3 {
				effective = 3
			}
			if effective > 19 {
				return "", numfmtTooLarge(n, precision)
			}
			n = numfmtRound(n, effective, o.round)
			if o.format.precision < 0 && numfmtAbs(n).Cmp(big.NewRat(10, 1)) >= 0 {
				precision = 0
			}
			if numfmtAbs(n).Cmp(big.NewRat(base, 1)) >= 0 {
				exponent++
				n.Quo(n, big.NewRat(base, 1))
				if o.format.precision < 0 {
					precision = 1
				}
			}
			if exponent > 10 {
				return "", fmt.Errorf("number too big to be printed")
			}
			suffix = string("KMGTPEZYRQ"[exponent-1])
			if o.to == "si" && exponent == 1 {
				suffix = "k"
			}
			if o.to == "iec-i" {
				suffix += "i"
			}
		}
	} else {
		if precision > 19 {
			return "", numfmtTooLarge(n, precision)
		}
		magnitude := numfmtAbs(n)
		if magnitude.Cmp(big.NewRat(1, 1)) < 0 {
			magnitude = big.NewRat(1, 1)
		}
		if new(big.Rat).Mul(magnitude, numfmtPow(10, precision)).Cmp(numfmtPow(10, 19)) >= 0 {
			return "", numfmtTooLarge(n, precision)
		}
		n = numfmtRound(n, precision, o.round)
	}
	if precision > 1000 {
		return "", numfmtTooLarge(n, precision)
	}
	number := n.FloatString(precision)
	if negativeZero && n.Sign() == 0 {
		number = "-" + number
	}
	if o.format.zero {
		number = numfmtPad(number, o.format.width, true)
	}
	if suffix != "" {
		if o.explicitSeparator {
			number += o.separator
		}
		number += suffix
	}
	number += o.suffix
	padding := o.padding
	if padding == 0 {
		padding = implicit
	}
	if o.format.width != 0 && !o.format.zero {
		padding = o.format.width
	}
	number = numfmtPad(number, padding, false)
	return o.format.prefix + number + o.format.suffix, nil
}

func numfmtTooLarge(n *big.Rat, precision int) error {
	f, _ := n.Float64()
	s := strconv.FormatFloat(f, 'e', 5, 64)
	mantissa, exponent, _ := strings.Cut(s, "e")
	mantissa = strings.TrimRight(strings.TrimRight(mantissa, "0"), ".")
	exp, _ := strconv.Atoi(exponent)
	return fmt.Errorf("value/precision too large to be printed: '%se%+d/%d' (consider using --to)", mantissa, exp, precision)
}
