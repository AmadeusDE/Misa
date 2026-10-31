// Behaviour adapted from uutils coreutils csplit (MIT):
// src/uu/csplit/src/split_name.rs and the printf subset in
// src/uucore/src/lib/features/format/{format,spec,num_format}.rs at commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663.
package builtins

import (
	"errors"
	"strconv"
	"strings"
)

var (
	errCsplitSuffixIncorrect = errors.New("incorrect conversion specification in suffix")
	errCsplitSuffixTooMany   = errors.New("too many % conversion specifications in suffix")
)

// csplitName builds the output file names: prefix + formatted counter.
type csplitName struct {
	prefix string
	format csplitFormat
}

// csplitFormat is one printf conversion for an unsigned integer, with the
// literal text around it.
type csplitFormat struct {
	before       string
	after        string
	prefix       string // "0x" or "0X" for %#x / %#X
	octalPrefix  bool   // %#o
	base         int
	upper        bool
	align        byte // 'l' left, 'z' zero padded, 's' space padded
	width        int
	precision    int
	hasPrecision bool
}

// newCsplitName validates prefix, --suffix-format and --digits.
func newCsplitName(prefix, format string, digits int) (*csplitName, error) {
	if format == "" {
		return &csplitName{prefix: prefix, format: csplitFormat{
			base:  10,
			align: 'z',
			width: digits,
		}}, nil
	}
	f, err := parseCsplitFormat(format)
	if err != nil {
		return nil, err
	}
	return &csplitName{prefix: prefix, format: f}, nil
}

func (n *csplitName) get(i int) string {
	return n.prefix + n.format.render(uint64(i))
}

// parseCsplitFormat parses a format string with exactly one integer
// conversion. "%%" is a literal percent sign.
func parseCsplitFormat(s string) (csplitFormat, error) {
	var f csplitFormat
	var before strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			before.WriteByte(s[i])
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			before.WriteByte('%')
			i++
			continue
		}
		spec, used, err := parseCsplitSpec(s[i:])
		if err != nil {
			return f, err
		}
		spec.before = before.String()
		var after strings.Builder
		for j := i + used; j < len(s); j++ {
			if s[j] == '%' {
				if j+1 < len(s) && s[j+1] == '%' {
					after.WriteByte('%')
					j++
					continue
				}
				return f, errCsplitSuffixTooMany
			}
			after.WriteByte(s[j])
		}
		spec.after = after.String()
		return spec, nil
	}
	return f, errCsplitSuffixIncorrect
}

// parseCsplitSpec parses one % conversion starting at s[0] and returns how
// many bytes it used.
func parseCsplitSpec(s string) (csplitFormat, int, error) {
	var f csplitFormat
	var minus, plus, space, hash, zero bool
	i := 1
	for ; i < len(s); i++ {
		switch s[i] {
		case '-':
			minus = true
		case '+':
			plus = true
		case ' ':
			space = true
		case '#':
			hash = true
		case '0':
			zero = true
		case '\'':
			// the C locale thousands separator: accepted, no effect
		default:
			goto flags
		}
	}
flags:
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > start {
		w, err := strconv.Atoi(s[start:i])
		if err != nil {
			return f, 0, errCsplitSuffixIncorrect
		}
		f.width = w
	}
	if i < len(s) && s[i] == '.' {
		i++
		start = i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		f.hasPrecision = true
		if i > start {
			p, err := strconv.Atoi(s[start:i])
			if err != nil {
				return f, 0, errCsplitSuffixIncorrect
			}
			f.precision = p
		}
	}
	// length modifiers are parsed and ignored
	for i < len(s) {
		switch s[i] {
		case 'h', 'l', 'j', 'z', 't', 'L':
			i++
			continue
		}
		break
	}
	if i >= len(s) {
		return f, 0, errCsplitSuffixIncorrect
	}
	switch conv := s[i]; conv {
	case 'd', 'i':
		// %+d and % d select a signed conversion, which csplit rejects;
		// %#d is not a valid conversion at all.
		if hash || plus || space {
			return f, 0, errCsplitSuffixIncorrect
		}
		f.base = 10
	case 'u':
		if hash {
			return f, 0, errCsplitSuffixIncorrect
		}
		f.base = 10
	case 'o':
		f.base = 8
		f.octalPrefix = hash
	case 'x', 'X':
		f.base = 16
		f.upper = conv == 'X'
		if hash {
			f.prefix = "0x"
			if f.upper {
				f.prefix = "0X"
			}
		}
	default:
		return f, 0, errCsplitSuffixIncorrect
	}
	i++
	// The 0 flag is ignored with - or an explicit precision.
	switch {
	case minus:
		f.align = 'l'
	case zero && !f.hasPrecision:
		f.align = 'z'
	default:
		f.align = 's'
	}
	return f, i, nil
}

// render formats n the way printf would, then wraps it in the literal text.
func (f csplitFormat) render(n uint64) string {
	var digits string
	switch f.base {
	case 8:
		digits = strconv.FormatUint(n, 8)
	case 16:
		digits = strconv.FormatUint(n, 16)
		if f.upper {
			digits = strings.ToUpper(digits)
		}
	default:
		digits = strconv.FormatUint(n, 10)
	}
	prec := 0
	if f.hasPrecision {
		prec = f.precision
	}
	// An octal alternate form adds a leading zero unless the value is
	// already wide enough to start with one.
	if f.octalPrefix && n != 0 && len(digits) >= prec {
		digits = "0" + digits
	}
	if f.hasPrecision && f.precision == 0 && n == 0 {
		// No digits at all, except that %#o still prints a zero.
		digits = ""
		if f.octalPrefix {
			digits = "0"
		}
	} else if len(digits) < prec {
		digits = strings.Repeat("0", prec-len(digits)) + digits
	}
	if f.prefix != "" && n != 0 {
		digits = f.prefix + digits
	}

	pad := f.width - len(digits)
	if pad < 0 {
		pad = 0
	}
	switch f.align {
	case 'l':
		digits += strings.Repeat(" ", pad)
	case 'z':
		// zeros go after a "0x"/"0X" prefix, as printf does
		head, rest := "", digits
		if len(digits) >= 2 && (digits[0] == '0' && (digits[1] == 'x' || digits[1] == 'X')) {
			head, rest = digits[:2], digits[2:]
		}
		digits = head + strings.Repeat("0", pad) + rest
	default:
		digits = strings.Repeat(" ", pad) + digits
	}
	return f.before + digits + f.after
}
