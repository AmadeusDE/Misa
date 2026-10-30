package builtins

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

type hexConv struct {
	text, format string
	kind         byte
	size         int
	end          bool
}
type hexUnit struct {
	repeat, size int
	explicit     bool
	conv         []hexConv
}
type hexFormat struct {
	units     []hexUnit
	size      int
	canonical bool
}

func hexParse(s string) (hexFormat, error) {
	var f hexFormat
	for i := 0; i < len(s); {
		for i < len(s) && unicode.IsSpace(rune(s[i])) {
			i++
		}
		if i == len(s) {
			break
		}
		u := hexUnit{repeat: 1}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i > start {
			n, e := strconv.Atoi(s[start:i])
			if e != nil || n < 1 || n > 1<<20 {
				return f, fmt.Errorf("invalid iteration count")
			}
			u.repeat = n
			u.explicit = true
		}
		for i < len(s) && unicode.IsSpace(rune(s[i])) {
			i++
		}
		if i < len(s) && s[i] == '/' {
			i++
			for i < len(s) && unicode.IsSpace(rune(s[i])) {
				i++
			}
			start = i
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			n, e := strconv.Atoi(s[start:i])
			if e != nil || n < 1 || n > 1<<20 {
				return f, fmt.Errorf("invalid byte count")
			}
			u.size = n
		}
		for i < len(s) && unicode.IsSpace(rune(s[i])) {
			i++
		}
		if i == len(s) || s[i] != '"' {
			return f, fmt.Errorf("expected quoted format")
		}
		i++
		var b strings.Builder
		for i < len(s) && s[i] != '"' {
			if s[i] == '\\' {
				i++
				if i == len(s) {
					return f, fmt.Errorf("unterminated escape")
				}
				switch s[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'r':
					b.WriteByte('\r')
				case 'b':
					b.WriteByte('\b')
				case 'f':
					b.WriteByte('\f')
				case 'v':
					b.WriteByte('\v')
				case 'a':
					b.WriteByte('\a')
				case '\\', '"':
					b.WriteByte(s[i])
				default:
					if s[i] < '0' || s[i] > '7' {
						return f, fmt.Errorf("invalid escape")
					}
					start := i
					for i+1 < len(s) && i-start < 2 && s[i+1] >= '0' && s[i+1] <= '7' {
						i++
					}
					n, _ := strconv.ParseUint(s[start:i+1], 8, 16)
					b.WriteByte(byte(n))
				}
				i++
			} else {
				b.WriteByte(s[i])
				i++
			}
		}
		if i == len(s) {
			return f, fmt.Errorf("unterminated format")
		}
		i++
		cs, consumed, err := hexConversions(b.String(), u.size)
		if err != nil {
			return f, err
		}
		u.conv = cs
		if u.size == 0 {
			u.size = consumed
		}
		if u.size > 0 && consumed == 0 {
			return f, fmt.Errorf("byte count without conversion")
		}
		if u.size > 0 && consumed != u.size {
			return f, fmt.Errorf("incompatible byte count")
		}
		if u.repeat > 0 && u.size > (1<<20)/u.repeat {
			return f, fmt.Errorf("format block too large")
		}
		f.size += u.repeat * u.size
		if f.size > 1<<20 {
			return f, fmt.Errorf("format block too large")
		}
		f.units = append(f.units, u)
	}
	return f, nil
}
func hexConversions(s string, explicit int) ([]hexConv, int, error) {
	var cs []hexConv
	count := 0
	numeric := 0
	for i := 0; i < len(s); {
		if s[i] != '%' {
			j := strings.IndexByte(s[i:], '%')
			if j < 0 {
				j = len(s) - i
			}
			cs = append(cs, hexConv{text: s[i : i+j]})
			i += j
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			cs = append(cs, hexConv{text: "%"})
			i += 2
			continue
		}
		start := i
		i++
		for i < len(s) && strings.ContainsRune("#0- +", rune(s[i])) {
			i++
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i < len(s) && s[i] == '.' {
			i++
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
		}
		if i >= len(s) {
			return nil, 0, fmt.Errorf("incomplete conversion")
		}
		prefix := s[start:i]
		kind := s[i]
		i++
		end := false
		if kind == '_' {
			if i >= len(s) {
				return nil, 0, fmt.Errorf("incomplete special conversion")
			}
			kind = s[i]
			i++
			if kind == 'a' || kind == 'A' {
				end = kind == 'A'
				if i >= len(s) || !strings.ContainsRune("dox", rune(s[i])) {
					return nil, 0, fmt.Errorf("invalid address radix")
				}
				kind = s[i]
				i++
				cs = append(cs, hexConv{format: prefix + string(kind), kind: 'a', end: end})
				continue
			}
			if !strings.ContainsRune("cpu", rune(kind)) {
				return nil, 0, fmt.Errorf("invalid special conversion")
			}
			kind = kind - 32
		}
		size := 0
		switch kind {
		case 'C', 'P', 'U', 'c':
			size = 1
		case 's':
			size = explicit
			if size == 0 {
				if dot := strings.IndexByte(prefix, '.'); dot >= 0 {
					size, _ = strconv.Atoi(prefix[dot+1:])
				}
			}
			if size < 1 || size > 1<<20 {
				return nil, 0, fmt.Errorf("%%s requires a bounded precision or byte count")
			}
		case 'd', 'i', 'u', 'o', 'x', 'X':
			size = 4
			if explicit != 0 {
				size = explicit
			}
			if size != 1 && size != 2 && size != 4 && size != 8 {
				return nil, 0, fmt.Errorf("invalid integer size")
			}
		case 'e', 'E', 'f', 'F', 'g', 'G':
			size = 8
			if explicit != 0 {
				size = explicit
			}
			if size != 4 && size != 8 {
				return nil, 0, fmt.Errorf("invalid float size")
			}
		default:
			return nil, 0, fmt.Errorf("invalid conversion %%%c", kind)
		}
		numeric++
		if explicit > 0 && numeric > 1 {
			return nil, 0, fmt.Errorf("byte count with multiple conversions")
		}
		count += size
		cs = append(cs, hexConv{format: prefix + string(kind), kind: kind, size: size})
	}
	return cs, count, nil
}
func (f hexFormat) render(buf []byte, n int, pos uint64, end bool, block int) string {
	if f.canonical {
		if end {
			return fmt.Sprintf("%08x\n", pos)
		}
		if n > 16 {
			n = 16
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%08x  ", pos)
		for i := 0; i < 16; i++ {
			if i < n {
				fmt.Fprintf(&b, "%02x ", buf[i])
			} else {
				b.WriteString("   ")
			}
			if i == 7 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(" |")
		for _, c := range buf[:n] {
			if c >= 32 && c <= 126 {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
		return b.String()
	}
	var b strings.Builder
	off := 0
	for ui, u := range f.units {
		finalUnit := false
		for _, c := range u.conv {
			if c.end {
				finalUnit = true
			}
		}
		if finalUnit != end {
			continue
		}
		reps := u.repeat
		if !u.explicit && u.size > 0 && ui == len(f.units)-1 && f.size < block {
			reps += (block - f.size) / u.size
		}
		for rep := 0; rep < reps; rep++ {
			for _, c := range u.conv {
				if c.kind == 0 {
					b.WriteString(c.text)
					continue
				}
				if c.kind == 'a' {
					if c.end == end {
						fmt.Fprintf(&b, c.format, pos+uint64(off))
					}
					continue
				}
				if end {
					continue
				}
				if off >= n { // preserve field widths for partial blocks
					format := c.format
					var sample any = uint64(0)
					switch c.kind {
					case 'C', 'P', 'U', 's':
						sample = ""
						format = format[:len(format)-1] + "s"
					case 'e', 'E', 'f', 'F', 'g', 'G':
						sample = float64(0)
						if c.kind == 'F' {
							format = format[:len(format)-1] + "f"
						}
					case 'i', 'u':
						format = format[:len(format)-1] + "d"
					}
					b.WriteString(strings.Repeat(" ", len(fmt.Sprintf(format, sample))))
					off += c.size
					continue
				}
				var padded [8]byte
				p := buf[off:]
				if len(p) < c.size {
					if c.size > len(padded) {
						p = append(append([]byte(nil), p...), make([]byte, c.size-len(p))...)
					} else {
						copy(padded[:], p)
						p = padded[:c.size]
					}
				} else {
					p = p[:c.size]
				}
				off += c.size
				var v uint64
				for i := len(p) - 1; i >= 0; i-- {
					v = v<<8 | uint64(p[i])
				}
				var value any = v
				format := c.format
				switch c.kind {
				case 'd', 'i':
					shift := 64 - uint(c.size*8)
					value = int64(v<<shift) >> shift
					if c.kind == 'i' {
						format = format[:len(format)-1] + "d"
					}
				case 'u':
					format = format[:len(format)-1] + "d"
				case 'c':
					value = rune(p[0])
				case 'C':
					value = odCharText(p[0])
					format = format[:len(format)-1] + "s"
				case 'P':
					if p[0] >= 32 && p[0] <= 126 {
						value = string(p[0])
					} else {
						value = "."
					}
					format = format[:len(format)-1] + "s"
				case 'U':
					if p[0] < 32 {
						value = odControlNames[p[0]]
					} else if p[0] == 127 {
						value = "del"
					} else if p[0] < 127 {
						value = string(p[0])
					} else {
						value = fmt.Sprintf("%02x", p[0])
					}
					format = format[:len(format)-1] + "s"
				case 's':
					value = strings.SplitN(string(p), "\x00", 2)[0]
				case 'e', 'E', 'f', 'F', 'g', 'G':
					if c.size == 4 {
						value = float64(math.Float32frombits(binary.LittleEndian.Uint32(p)))
					} else {
						value = math.Float64frombits(v)
					}
					if c.kind == 'F' {
						format = format[:len(format)-1] + "f"
					}
				}
				fmt.Fprintf(&b, format, value)
			}
			if !end && rep == reps-1 && u.explicit { // BSD hexdump drops trailing whitespace on last iteration
				s := b.String()
				trim := strings.TrimRight(s, " \t")
				if len(trim) != len(s) {
					b.Reset()
					b.WriteString(trim)
				}
			}
		}
	}
	return b.String()
}
