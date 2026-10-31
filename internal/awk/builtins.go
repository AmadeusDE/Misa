package awk

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/AmadeusDE/Misa/internal/posixre"
)

func arith(op tokKind, x, y float64) float64 {
	switch op {
	case tPlus:
		return x + y
	case tMinus:
		return x - y
	case tStar:
		return x * y
	case tSlash:
		if y == 0 {
			runtimeErrorf("division by zero")
		}
		return x / y
	case tPercent:
		if y == 0 {
			runtimeErrorf("division by zero in %%")
		}
		return math.Mod(x, y)
	}
	return math.Pow(x, y)
}

func (in *interp) call(c *callExpr) Value {
	args := c.args
	switch c.name {
	case "length":
		if len(args) == 0 {
			in.ensureRecord()
			return num(float64(in.count(in.r.rec)))
		}
		if v, ok := args[0].(*varRef); ok {
			if a := in.peekArray(v); a != nil {
				return num(float64(a.len()))
			}
		}
		return num(float64(in.count(in.eval(args[0]).text(in.convfmt))))
	case "substr":
		return in.substr(args)
	case "index":
		s := in.eval(args[0]).text(in.convfmt)
		t := in.eval(args[1]).text(in.convfmt)
		if i := strings.Index(s, t); i >= 0 && t != "" {
			return num(float64(in.count(s[:i]) + 1))
		}
		return num(0)
	case "sprintf":
		format := in.eval(args[0]).text(in.convfmt)
		return str(sprintf(format, in.evalAll(args[1:]), in.cfg.Bytes, in.convfmt))
	case "tolower":
		return str(in.mapCase(in.eval(args[0]).text(in.convfmt), unicode.ToLower))
	case "toupper":
		return str(in.mapCase(in.eval(args[0]).text(in.convfmt), unicode.ToUpper))
	case "int":
		return num(math.Trunc(in.eval(args[0]).float()))
	case "sqrt", "exp", "log", "sin", "cos":
		return num(mathFunc(c.name, in.eval(args[0]).float()))
	case "atan2":
		y := in.eval(args[0]).float()
		return num(math.Atan2(y, in.eval(args[1]).float()))
	case "rand":
		return num(in.rng.Float64())
	case "srand":
		return in.srand(args)
	case "match":
		return in.match(args)
	case "split":
		return in.split(args)
	case "close":
		return num(float64(in.closeNamed(in.eval(args[0]).text(in.convfmt))))
	case "system":
		return in.system(in.eval(args[0]).text(in.convfmt))
	case "fflush":
		return in.fflush(args)
	}
	return in.substitute(c)
}

var mathFuncs = map[string]func(float64) float64{
	"sqrt": math.Sqrt, "exp": math.Exp, "log": math.Log, "sin": math.Sin, "cos": math.Cos,
}

func mathFunc(name string, x float64) float64 { return mathFuncs[name](x) }

func (in *interp) srand(args []expr) Value {
	prev := in.seed
	if len(args) == 0 {
		in.seed = float64(time.Now().Unix())
	} else {
		in.seed = in.eval(args[0]).float()
	}
	in.rng = rand.New(rand.NewSource(int64(in.seed)))
	return num(prev)
}

// count is the length of s in characters, or bytes with Config.Bytes.
func (in *interp) count(s string) int {
	if in.cfg.Bytes {
		return len(s)
	}
	return utf8.RuneCountInString(s)
}

// width is the size of the character at the start of s.
func (in *interp) width(s string) int {
	if in.cfg.Bytes || s == "" {
		return 1
	}
	_, w := utf8.DecodeRuneInString(s)
	return w
}

// offset returns the byte offset of character index n (0-based), clamped.
func (in *interp) offset(s string, n int) int {
	if in.cfg.Bytes {
		return min(n, len(s))
	}
	i := 0
	for ; n > 0 && i < len(s); n-- {
		i += in.width(s[i:])
	}
	return i
}

func (in *interp) substr(args []expr) Value {
	s := in.eval(args[0]).text(in.convfmt)
	start := math.RoundToEven(in.eval(args[1]).float())
	end := math.Inf(1)
	if len(args) == 3 {
		end = start + math.RoundToEven(in.eval(args[2]).float())
	}
	if math.IsNaN(start) || math.IsNaN(end) {
		return str("")
	}
	total := float64(in.count(s))
	start = math.Max(start, 1)
	end = math.Min(end, total+1)
	if end <= start {
		return str("")
	}
	from := in.offset(s, int(start)-1)
	return str(s[from : from+in.offset(s[from:], int(end-start))])
}

func (in *interp) mapCase(s string, f func(rune) rune) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		switch {
		case in.cfg.Bytes || (r == utf8.RuneError && w == 1):
			c := s[i]
			if c < utf8.RuneSelf {
				c = byte(f(rune(c)))
			}
			b.WriteByte(c)
			w = 1
		default:
			b.WriteRune(f(r))
		}
		i += w
	}
	return b.String()
}

func (in *interp) match(args []expr) Value {
	s := in.eval(args[0]).text(in.convfmt)
	re := in.regexOf(args[1])
	m, err := re.Find([]byte(s), 0)
	if err != nil {
		runtimeErrorf("regular expression error: %v", err)
	}
	if m == nil {
		in.vars[vRSTART], in.vars[vRLENGTH] = num(0), num(-1)
		return num(0)
	}
	start := in.count(s[:m[0]]) + 1
	in.vars[vRSTART] = num(float64(start))
	in.vars[vRLENGTH] = num(float64(in.count(s[m[0]:m[1]])))
	return num(float64(start))
}

// substitute implements sub and gsub.
func (in *interp) substitute(c *callExpr) Value {
	global := c.name == "gsub"
	re := in.regexOf(c.args[0])
	repl := in.eval(c.args[1]).text(in.convfmt)
	target := ref{field: true, idx: 0}
	if len(c.args) == 3 {
		target = in.resolve(c.args[2])
	}
	s := in.load(target).text(in.convfmt)
	out, n := in.replace(re, s, repl, global)
	if n > 0 {
		in.store(target, str(out))
	}
	return num(float64(n))
}

func (in *interp) replace(re *posixre.Regexp, s, repl string, global bool) (string, int) {
	bs := []byte(s)
	var out []byte
	count, copied, lastEnd := 0, 0, -1
	for pos := 0; pos <= len(bs); {
		m, err := re.Find(bs, pos)
		if err != nil {
			runtimeErrorf("regular expression error: %v", err)
		}
		if m == nil {
			break
		}
		ms, me := m[0], m[1]
		if ms == me && ms == lastEnd {
			if ms >= len(bs) {
				break
			}
			pos = ms + in.width(s[ms:])
			continue
		}
		out = append(out, bs[copied:ms]...)
		out = appendReplacement(out, repl, bs[ms:me])
		count++
		copied, lastEnd = me, me
		if !global {
			break
		}
		if me > ms {
			pos = me
			continue
		}
		if ms >= len(bs) {
			break
		}
		w := in.width(s[ms:])
		out = append(out, bs[ms:ms+w]...)
		copied, pos = ms+w, ms+w
	}
	out = append(out, bs[copied:]...)
	return string(out), count
}

// appendReplacement expands & to the match; \& is a literal ampersand and
// \\ a literal backslash.
func appendReplacement(out []byte, repl string, matched []byte) []byte {
	for i := 0; i < len(repl); i++ {
		switch c := repl[i]; {
		case c == '\\' && i+1 < len(repl) && (repl[i+1] == '&' || repl[i+1] == '\\'):
			out = append(out, repl[i+1])
			i++
		case c == '&':
			out = append(out, matched...)
		default:
			out = append(out, c)
		}
	}
	return out
}

var regexEscapes = map[byte]byte{
	'n': '\n', 't': '\t', 'r': '\r', 'f': '\f', 'v': '\v', 'a': 7, 'b': 8, '"': '"',
}

// translateRegex rewrites the C-style escapes AWK allows in regular
// expressions into the characters they stand for; the rest is left to the
// ERE engine. A produced special character is wrapped in a bracket.
func translateRegex(src string) string {
	if !strings.Contains(src, "\\") {
		return src
	}
	var b strings.Builder
	for i := 0; i < len(src); i++ {
		c := src[i]
		if c != '\\' || i+1 >= len(src) {
			b.WriteByte(c)
			continue
		}
		i++
		d := src[i]
		if r, ok := regexEscapes[d]; ok {
			b.WriteByte(r)
			continue
		}
		if d >= '0' && d <= '7' {
			j := i
			for j < len(src) && j < i+3 && src[j] >= '0' && src[j] <= '7' {
				j++
			}
			v, _ := strconv.ParseUint(src[i:j], 8, 16)
			i = j - 1
			ch := byte(v)
			if strings.IndexByte(`\^$.[]|()*+?{}`, ch) >= 0 {
				b.WriteString("[" + string(ch) + "]")
			} else {
				b.WriteByte(ch)
			}
			continue
		}
		b.WriteByte('\\')
		b.WriteByte(d)
	}
	return b.String()
}
