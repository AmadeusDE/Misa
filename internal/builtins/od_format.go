// Behaviour adapted from uutils coreutils od (MIT), commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663, files src/uu/od/src/prn_int.rs,
// prn_char.rs, prn_float.rs and formatter_item_info.rs.
package builtins

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// odFormatKind is one printable representation of the input.
type odFormatKind int

const (
	odFmtAscii  odFormatKind = iota // -a, -t a: named characters
	odFmtChar                       // -c, -t c: escaped characters
	odFmtOct                        // -o, -b, -t o[1248]
	odFmtHex                        // -x, -t x[1248]
	odFmtDecS                       // -d, -s, -i, -l, -t d[1248]
	odFmtDecU                       // -t u[1248]
	odFmtHalf                       // -t f2, -t fH: IEEE binary16
	odFmtBFloat                     // -t fB: bfloat16
	odFmtFloat                      // -f, -t f4: IEEE binary32
	odFmtDouble                     // -e, -F, -t f, -t f8: IEEE binary64
)

// odFormat is one -t entry. Every rendered field is exactly width bytes, which
// is what the column alignment in od's multi-format raster depends on.
type odFormat struct {
	kind odFormatKind
	size int // bytes per value
	// width of the printed field, including the leading space od puts in front
	// of every value
	width int
	dump  bool // 'z' suffix: append the printable-character dump
}

// odIntWidths returns the print width of an integer field, matching the
// maximum string length plus the leading space.
func odIntWidth(kind odFormatKind, size int) (int, bool) {
	table := map[odFormatKind]map[int]int{
		odFmtOct:  {1: 4, 2: 7, 4: 12, 8: 23},
		odFmtHex:  {1: 3, 2: 5, 4: 9, 8: 17},
		odFmtDecU: {1: 4, 2: 6, 4: 11, 8: 21},
		odFmtDecS: {1: 5, 2: 7, 4: 12, 8: 21},
	}
	width, ok := table[kind][size]
	return width, ok
}

// odNewFormat builds a format for kind and size in bytes; ok is false for an
// unsupported size (including the 16-byte long double).
func odNewFormat(kind odFormatKind, size int) (odFormat, bool) {
	switch kind {
	case odFmtAscii, odFmtChar:
		return odFormat{kind: kind, size: 1, width: 4}, true
	case odFmtHalf, odFmtBFloat:
		if size != 2 {
			return odFormat{}, false
		}
		return odFormat{kind: kind, size: 2, width: 16}, true
	case odFmtFloat:
		if size != 4 {
			return odFormat{}, false
		}
		return odFormat{kind: kind, size: 4, width: 16}, true
	case odFmtDouble:
		if size != 8 {
			return odFormat{}, false
		}
		return odFormat{kind: kind, size: 8, width: 25}, true
	}
	if width, ok := odIntWidth(kind, size); ok {
		return odFormat{kind: kind, size: size, width: width}, true
	}
	return odFormat{}, false
}

// odExtraSpacing returns the number of spaces to insert in front of each value
// of f so that all formats of a raster line up. See OutputInfo::calculate_alignment.
func odSpacing(f odFormat, block, printBlock int) [16]int {
	var spacing [16]int
	size, items := f.size, block/f.size
	missing := printBlock - f.width*items
	for items > 0 {
		avg := missing / items
		for i := 0; i < items; i++ {
			spacing[i*size] += avg
			missing -= avg
		}
		items /= 2
		size *= 2
	}
	return spacing
}

// odField renders one value of f at offset off in buf.
func (f odFormat) odField(buf []byte, off int, order binary.ByteOrder) string {
	switch f.kind {
	case odFmtAscii:
		return odPadLeft(odAsciiName(buf[off]), f.width)
	case odFmtChar:
		return odPadLeft(odCharText(buf[off]), f.width)
	case odFmtHalf:
		bits := uint16(odReadUint(buf, off, 2, order))
		return " " + odPadLeft(odTrimFloat(odBinary16Raw(odHalfFloat(bits), odHalfSubnormal(bits))), f.width-1)
	case odFmtBFloat:
		bits := uint16(odReadUint(buf, off, 2, order))
		return " " + odPadLeft(odTrimFloat(odBinary16Raw(odBFloatValue(bits), odBFloatSubnormal(bits))), f.width-1)
	case odFmtFloat:
		v := float64(math.Float32frombits(uint32(odReadUint(buf, off, 4, order))))
		return " " + odPadLeft(odShortestFloat(v, 32, odIsSubnormalFloat32(v)), f.width-1)
	case odFmtDouble:
		v := math.Float64frombits(odReadUint(buf, off, 8, order))
		return " " + odPadLeft(odShortestFloat(v, 64, odIsSubnormalFloat64(v)), f.width-1)
	}
	v := odReadUint(buf, off, f.size, order)
	switch f.kind {
	case odFmtOct:
		return " " + fmt.Sprintf("%0*o", f.width-1, v)
	case odFmtHex:
		return " " + fmt.Sprintf("%0*x", f.width-1, v)
	case odFmtDecU:
		return " " + odPadLeft(strconv.FormatUint(v, 10), f.width-1)
	}
	// signed decimal: sign-extend from the field size
	shift := 64 - uint(f.size)*8
	signed := int64(v<<shift) >> shift
	return " " + odPadLeft(strconv.FormatInt(signed, 10), f.width-1)
}

func odReadUint(buf []byte, off, size int, order binary.ByteOrder) uint64 {
	var v uint64
	switch size {
	case 1:
		return uint64(buf[off])
	case 2:
		v = uint64(order.Uint16(buf[off:]))
	case 4:
		v = uint64(order.Uint32(buf[off:]))
	default:
		v = order.Uint64(buf[off:])
	}
	return v
}

func odPadLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// odControlNames are the names -a prints for the 32 C0 control characters.
var odControlNames = [32]string{
	"nul", "soh", "stx", "etx", "eot", "enq", "ack", "bel",
	"bs", "ht", "nl", "vt", "ff", "cr", "so", "si",
	"dle", "dc1", "dc2", "dc3", "dc4", "nak", "syn", "etb",
	"can", "em", "sub", "esc", "fs", "gs", "rs", "us",
}

// odAsciiName is the -a ("named character") representation of b.
func odAsciiName(b byte) string {
	switch b &= 0x7f; {
	case b < 32:
		return odControlNames[b]
	case b == 32:
		return "sp"
	case b < 127:
		return string(rune(b))
	}
	return "del"
}

// odCharText is the -c ("escaped character") representation of one byte.
func odCharText(b byte) string {
	if b >= 0x80 {
		return fmt.Sprintf("%03o", b)
	}
	switch b {
	case 0:
		return "\\0"
	case 7:
		return "\\a"
	case 8:
		return "\\b"
	case 9:
		return "\\t"
	case 10:
		return "\\n"
	case 11:
		return "\\v"
	case 12:
		return "\\f"
	case 13:
		return "\\r"
	case 127:
		return "177"
	}
	if b < 32 {
		return fmt.Sprintf("%03o", b)
	}
	return string(rune(b))
}

// odDumpText is the '>...<' column the z suffix and -tx1z append.
func odDumpText(buf []byte) string {
	var b strings.Builder
	b.WriteByte('>')
	for _, c := range buf {
		if c >= 0x20 && c <= 0x7e {
			b.WriteByte(c)
		} else {
			b.WriteByte('.')
		}
	}
	b.WriteByte('<')
	return b.String()
}

// odHalfFloat converts IEEE binary16 bits to a float64 value.
func odHalfFloat(bits uint16) float64 {
	sign := 1.0
	if bits&0x8000 != 0 {
		sign = -1
	}
	exp := int(bits>>10) & 0x1f
	mant := int(bits & 0x3ff)
	switch exp {
	case 0x1f:
		if mant == 0 {
			return sign * math.Inf(1)
		}
		return math.NaN()
	case 0:
		return sign * math.Ldexp(float64(mant), -24)
	}
	return sign * math.Ldexp(float64(mant+1024), exp-25)
}

// odBFloatValue converts bfloat16 bits (a truncated binary32) to float64.
func odBFloatValue(bits uint16) float64 {
	return float64(math.Float32frombits(uint32(bits) << 16))
}

func odHalfSubnormal(bits uint16) bool {
	return bits&0x7c00 == 0 && bits&0x03ff != 0
}

func odBFloatSubnormal(bits uint16) bool {
	return bits&0x7f80 == 0 && bits&0x007f != 0
}

func odIsSubnormalFloat32(f float64) bool {
	bits := math.Float32bits(float32(f))
	return bits&0x7f800000 == 0 && bits&0x007fffff != 0
}

func odIsSubnormalFloat64(f float64) bool {
	bits := math.Float64bits(f)
	return bits&0x7ff0000000000000 == 0 && bits&0xfffffffffffff != 0
}

// odBinary16Raw mimics uutils' format_binary16_like: 8 significant digits for
// half and bfloat values, exponential notation for subnormals.
func odBinary16Raw(f float64, subnormal bool) string {
	const width, precision = 15, 8
	if subnormal {
		return odPadLeft(odExpText(f, 64, precision-1), width)
	}
	if !odIsNormalFloat(f) {
		switch {
		case math.IsNaN(f):
			return odPadLeft("NaN", width)
		case math.IsInf(f, 1):
			return odPadLeft("inf", width)
		case math.IsInf(f, -1):
			return odPadLeft("-inf", width)
		case math.Signbit(f):
			return odPadLeft("-0", width)
		}
		return odPadLeft("0", width)
	}
	l := odFloatExponent10(f)
	switch {
	case l >= 0 && l <= precision-1:
		return odPadLeft(strconv.FormatFloat(f, 'f', precision-1-l, 64), width)
	case l == -1:
		return odPadLeft(strconv.FormatFloat(f, 'f', precision, 64), width)
	}
	return odPadLeft(odExpText(f, 64, precision-1), width)
}

func odIsNormalFloat(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0) && f != 0 && !odIsSubnormalFloat64(f)
}

// odFloatExponent10 is floor(log10(|f|)), corrected for the rounding error in
// math.Log10 so that 10^l brackets f.
func odFloatExponent10(f float64) int {
	l := int(math.Floor(math.Log10(math.Abs(f))))
	r := math.Pow(10, float64(l))
	if (f > 0 && r > f) || (f < 0 && -r < f) {
		l--
	}
	return l
}

// odExpText formats f in exponential notation with prec digits after the point
// and the exponent in the shortest form od prints ("1e+16", "1e-05" becomes
// "1e-5"). prec -1 uses the shortest representation that round-trips.
func odExpText(f float64, bits, prec int) string {
	s := strconv.FormatFloat(f, 'e', prec, bits)
	i := strings.IndexByte(s, 'e')
	if i < 0 {
		return s
	}
	mant, exp := s[:i], s[i+1:]
	sign := "+"
	if strings.HasPrefix(exp, "-") {
		sign = "-"
		exp = exp[1:]
	} else {
		exp = strings.TrimPrefix(exp, "+")
	}
	exp = strings.TrimLeft(exp, "0")
	if exp == "" {
		exp = "0"
	}
	if sign == "+" && exp == "0" {
		return mant + "e0"
	}
	return mant + "e" + sign + exp
}

// odShortestFloat prints f the way uutils (following GNU's ftoastr) does: the
// shorter of Rust's Display and Debug representations, with "inf"/"NaN"/"-0"
// spellings for the special values. sub forces exponential notation, as od does
// for subnormals.
func odShortestFloat(f float64, bits int, sub bool) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case f == 0:
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	display := strconv.FormatFloat(f, 'f', -1, bits)
	if sub {
		return odExpText(f, bits, -1)
	}
	exp := odShortestExponent(f, bits)
	if exp < -4 || exp >= 16 {
		if e := odExpText(f, bits, -1); len(e) < len(display) {
			return e
		}
		return display
	}
	debug := display
	if !strings.Contains(display, ".") {
		debug += ".0"
	}
	if len(display) <= len(debug) {
		return display
	}
	return debug
}

// odShortestExponent is the decimal exponent of f's shortest round-trip
// representation, which is what decides between plain and exponential notation.
func odShortestExponent(f float64, bits int) int {
	s := strconv.FormatFloat(f, 'e', -1, bits)
	i := strings.IndexByte(s, 'e')
	if i < 0 {
		return 0
	}
	exp, _ := strconv.Atoi(s[i+1:])
	return exp
}

// odTrimFloat drops the padding and the redundant trailing zeros of a formatted
// float, keeping any exponent untouched.
func odTrimFloat(raw string) string {
	s := strings.TrimLeft(raw, " ")
	switch strings.ToLower(s) {
	case "nan", "inf", "-inf":
		return s
	}
	mant, exp := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant, exp = s[:i], s[i:]
	}
	if strings.Contains(mant, ".") {
		mant = strings.TrimRight(mant, "0")
		mant = strings.TrimSuffix(mant, ".")
	}
	if mant == "" || mant == "-" || mant == "+" {
		mant = "0"
	}
	return mant + exp
}
