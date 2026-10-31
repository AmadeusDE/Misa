package awk

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type fmtSpec struct {
	minus, plus, space, alt, zero bool
	width                         int
	prec                          int
	hasPrec                       bool
}

type argList struct {
	vals    []Value
	convfmt string
}

func (a *argList) next() Value {
	if len(a.vals) == 0 {
		return Value{}
	}
	v := a.vals[0]
	a.vals = a.vals[1:]
	return v
}

// sprintf formats like C's printf with AWK's conversions. Missing arguments
// count as uninitialised values.
func sprintf(format string, vals []Value, byteMode bool, convfmt string) string {
	var out strings.Builder
	args := &argList{vals: vals, convfmt: convfmt}
	for i := 0; i < len(format); {
		c := format[i]
		if c != '%' {
			j := strings.IndexByte(format[i:], '%')
			if j < 0 {
				out.WriteString(format[i:])
				break
			}
			out.WriteString(format[i : i+j])
			i += j
			continue
		}
		if i+1 < len(format) && format[i+1] == '%' {
			out.WriteByte('%')
			i += 2
			continue
		}
		spec, conv, end := parseSpec(format, i+1, args)
		if conv == 0 {
			out.WriteString(format[i:])
			break
		}
		out.WriteString(formatOne(spec, conv, args, byteMode))
		i = end
	}
	return out.String()
}

// parseSpec reads flags, width and precision after a %, returning the
// conversion letter (0 at the end of the format) and the next index.
func parseSpec(f string, i int, args *argList) (fmtSpec, byte, int) {
	var s fmtSpec
	for ; i < len(f); i++ {
		switch f[i] {
		case '-':
			s.minus = true
			continue
		case '+':
			s.plus = true
			continue
		case ' ':
			s.space = true
			continue
		case '#':
			s.alt = true
			continue
		case '0':
			s.zero = true
			continue
		}
		break
	}
	s.width, i = parseCount(f, i, args)
	if s.width < 0 {
		s.minus, s.width = true, -s.width
	}
	if i < len(f) && f[i] == '.' {
		s.hasPrec = true
		s.prec, i = parseCount(f, i+1, args)
		if s.prec < 0 {
			s.hasPrec = false
		}
	}
	for i < len(f) && strings.IndexByte("hlLqjzt", f[i]) >= 0 {
		i++
	}
	if i >= len(f) {
		return s, 0, i
	}
	return s, f[i], i + 1
}

func parseCount(f string, i int, args *argList) (int, int) {
	if i < len(f) && f[i] == '*' {
		return int(args.next().float()), i + 1
	}
	n := 0
	for i < len(f) && isDigit(f[i]) {
		if n < 1<<24 {
			n = n*10 + int(f[i]-'0')
		}
		i++
	}
	return n, i
}

func formatOne(s fmtSpec, conv byte, args *argList, byteMode bool) string {
	switch conv {
	case 'd', 'i':
		return formatInt(s, args.next().float())
	case 'o', 'x', 'X', 'u':
		return formatUnsigned(s, conv, args.next().float())
	case 'e', 'E', 'f', 'F', 'g', 'G':
		return formatFloat(s, conv, args.next().float())
	case '%':
		return "%"
	case 'c':
		return pad(s, charOf(args.next(), byteMode), false)
	case 's':
		text := args.next().text(args.convfmt)
		if s.hasPrec {
			text = truncateChars(text, s.prec, byteMode)
		}
		return pad(s, text, false)
	}
	return "%" + string(conv)
}

func charOf(v Value, byteMode bool) string {
	if v.k == kStr {
		if v.s == "" {
			return "\x00"
		}
		_, w := utf8.DecodeRuneInString(v.s)
		if byteMode {
			w = 1
		}
		return v.s[:w]
	}
	n := int64(v.float())
	if byteMode {
		return string([]byte{byte(n)})
	}
	if n < 0 || n > utf8.MaxRune || !utf8.ValidRune(rune(n)) {
		return string(utf8.RuneError)
	}
	return string(rune(n))
}

func truncateChars(s string, n int, byteMode bool) string {
	if byteMode {
		if n < len(s) {
			return s[:n]
		}
		return s
	}
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}

// pad applies the width; zero padding goes after the sign and any prefix.
func pad(s fmtSpec, body string, numeric bool) string {
	n := displayWidth(body)
	if n >= s.width {
		return body
	}
	fill := s.width - n
	switch {
	case s.minus:
		return body + strings.Repeat(" ", fill)
	case numeric && s.zero:
		sign := signLen(body)
		return body[:sign] + strings.Repeat("0", fill) + body[sign:]
	}
	return strings.Repeat(" ", fill) + body
}

func displayWidth(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// signLen is the length of a sign and 0x/0X prefix at the start of a number.
func signLen(body string) int {
	n := 0
	if n < len(body) && (body[n] == '+' || body[n] == '-' || body[n] == ' ') {
		n++
	}
	if strings.HasPrefix(body[n:], "0x") || strings.HasPrefix(body[n:], "0X") {
		n += 2
	}
	return n
}

func signOf(s fmtSpec, negative bool) string {
	switch {
	case negative:
		return "-"
	case s.plus:
		return "+"
	case s.space:
		return " "
	}
	return ""
}

func formatInt(s fmtSpec, f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nonFinite(s, f)
	}
	f = math.Trunc(f)
	var digits string
	if math.Abs(f) < 1<<63 {
		digits = strconv.FormatInt(int64(math.Abs(f)), 10)
	} else {
		digits = strconv.FormatFloat(math.Abs(f), 'f', 0, 64)
	}
	return finishInt(s, signOf(s, f < 0), digits)
}

func finishInt(s fmtSpec, sign, digits string) string {
	if s.hasPrec {
		s.zero = false
		if s.prec == 0 && digits == "0" {
			digits = ""
		}
		if len(digits) < s.prec {
			digits = strings.Repeat("0", s.prec-len(digits)) + digits
		}
	}
	return pad(s, sign+digits, true)
}

func toUint(f float64) uint64 {
	f = math.Trunc(f)
	switch {
	case f >= 1<<64:
		return math.MaxUint64
	case f >= 0:
		return uint64(f)
	case f > -(1 << 63):
		return uint64(int64(f))
	}
	return 1 << 63
}

func formatUnsigned(s fmtSpec, conv byte, f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nonFinite(s, f)
	}
	u := toUint(f)
	base := map[byte]int{'o': 8, 'x': 16, 'X': 16, 'u': 10}[conv]
	digits := strconv.FormatUint(u, base)
	if conv == 'X' {
		digits = strings.ToUpper(digits)
	}
	prefix := ""
	switch {
	case s.alt && conv == 'o' && !strings.HasPrefix(digits, "0"):
		digits = "0" + digits
	case s.alt && u != 0 && conv == 'x':
		prefix = "0x"
	case s.alt && u != 0 && conv == 'X':
		prefix = "0X"
	}
	if s.hasPrec {
		s.zero = false
		if s.prec == 0 && u == 0 && !(s.alt && conv == 'o') {
			digits = ""
		}
		if len(digits) < s.prec {
			digits = strings.Repeat("0", s.prec-len(digits)) + digits
		}
	}
	return pad(s, prefix+digits, true)
}

func nonFinite(s fmtSpec, f float64) string {
	s.zero = false
	text := "inf"
	if math.IsNaN(f) {
		text = "nan"
	}
	return pad(s, signOf(s, math.Signbit(f))+text, false)
}

func formatFloat(s fmtSpec, conv byte, f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		out := nonFinite(s, f)
		if conv == 'E' || conv == 'F' || conv == 'G' {
			out = strings.ToUpper(out)
		}
		return out
	}
	prec := 6
	if s.hasPrec {
		prec = s.prec
	}
	abs := math.Abs(f)
	var body string
	switch conv {
	case 'e', 'E':
		body = strconv.FormatFloat(abs, 'e', prec, 64)
		body = fixExponent(body, s.alt && prec == 0)
	case 'f', 'F':
		body = strconv.FormatFloat(abs, 'f', prec, 64)
		if s.alt && prec == 0 {
			body += "."
		}
	default:
		body = formatG(abs, prec, s.alt)
	}
	if conv == 'E' || conv == 'G' {
		body = strings.ToUpper(body)
	}
	return pad(s, signOf(s, math.Signbit(f))+body, true)
}

// fixExponent makes Go's exponent match C's: at least two digits, which Go
// already emits, so only the alternate-form decimal point is added.
func fixExponent(body string, point bool) string {
	if !point {
		return body
	}
	i := strings.IndexByte(body, 'e')
	return body[:i] + "." + body[i:]
}

func formatG(abs float64, prec int, alt bool) string {
	if prec == 0 {
		prec = 1
	}
	exp := 0
	if abs != 0 {
		e := strconv.FormatFloat(abs, 'e', prec-1, 64)
		exp, _ = strconv.Atoi(e[strings.IndexByte(e, 'e')+1:])
	}
	var body string
	if exp < -4 || exp >= prec {
		body = strconv.FormatFloat(abs, 'e', prec-1, 64)
		if !alt {
			i := strings.IndexByte(body, 'e')
			body = trimZeros(body[:i]) + body[i:]
		}
		return body
	}
	body = strconv.FormatFloat(abs, 'f', prec-1-exp, 64)
	if !alt {
		body = trimZeros(body)
	} else if !strings.Contains(body, ".") {
		body += "."
	}
	return body
}

func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}
