// Derived from gobox pkg/args (0BSD).
//
// POSIX/GNU command-line option parser with compact spec strings.
//
// Spec grammar:
//
//	^          disable GNU permutation
//	c          boolean short flag -c
//	c:         short flag -c takes a string value
//	c#         short flag -c takes a numeric value
//	c(name)    pairs short -c with long --name
//	(name)     long-only flag --name (boolean unless followed by : or #)

package builtins

import (
	"fmt"
	"strconv"
	"strings"
)

type argKind int

const (
	kindBool argKind = iota
	kindString
	kindNum
	kindOptString
)

type flagDef struct {
	short byte
	long  string
	kind  argKind
}

func (f flagDef) key() string {
	if f.short != 0 {
		return string(f.short)
	}
	return f.long
}

type Spec struct {
	byShort            map[byte]flagDef
	byLong             map[string]flagDef
	stopAtFirstOperand bool
}

func ParseSpec(spec string) (*Spec, error) {
	s := &Spec{byShort: map[byte]flagDef{}, byLong: map[string]flagDef{}}

	i := 0
	if strings.HasPrefix(spec, "^") {
		s.stopAtFirstOperand = true
		i++
	}

	for i < len(spec) {
		c := spec[i]
		var def flagDef

		switch {
		case c == '(':
			end := strings.IndexByte(spec[i:], ')')
			if end < 0 {
				return nil, fmt.Errorf("args: unterminated long name in spec %q", spec)
			}
			def.long = spec[i+1 : i+end]
			i += end + 1
		case isSpecLetter(c):
			def.short = c
			i++
			if i < len(spec) && spec[i] == '(' {
				end := strings.IndexByte(spec[i:], ')')
				if end < 0 {
					return nil, fmt.Errorf("args: unterminated long name in spec %q", spec)
				}
				def.long = spec[i+1 : i+end]
				i += end + 1
			}
		default:
			return nil, fmt.Errorf("args: unexpected char %q in spec %q", c, spec)
		}

		if i < len(spec) && (spec[i] == ':' || spec[i] == '#') {
			if spec[i] == ':' {
				def.kind = kindString
				i++
				if i < len(spec) && spec[i] == '?' {
					def.kind = kindOptString
					i++
				}
			} else {
				def.kind = kindNum
				i++
			}
		}

		if def.short != 0 {
			if _, dup := s.byShort[def.short]; dup {
				return nil, fmt.Errorf("args: duplicate short flag %q in spec %q", def.short, spec)
			}
			s.byShort[def.short] = def
		}
		if def.long != "" {
			longKey := strings.ToLower(def.long)
			if _, dup := s.byLong[longKey]; dup {
				return nil, fmt.Errorf("args: duplicate long flag %q in spec %q", def.long, spec)
			}
			s.byLong[longKey] = def
		}
	}

	return s, nil
}

func isSpecLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

type ParseResult struct {
	bools            map[string]bool
	strings          map[string]string
	allStrings       map[string][]string
	nums             map[string]int64
	order            []string
	Operands         []string
	HelpRequested    bool
	VersionRequested bool
}

// caseFold reports whether a lowercase fallback lookup makes sense for name:
// only for multi-character (long) flag names. A single character is a short
// flag, where upper/lower case are legitimately different flags (e.g. cut's
// -f vs -F), so those must never be folded together.
func caseFold(name string) bool {
	return len(name) > 1
}

func (r *ParseResult) Bool(name string) bool {
	if r.bools[name] {
		return true
	}
	if caseFold(name) {
		return r.bools[strings.ToLower(name)]
	}
	return false
}

func (r *ParseResult) ExplicitString(name string) (string, bool) {
	if v, ok := r.strings[name]; ok {
		return v, true
	}
	if caseFold(name) {
		v, ok := r.strings[strings.ToLower(name)]
		return v, ok
	}
	return "", false
}

func (r *ParseResult) String(name string) (string, bool) {
	if v, ok := r.ExplicitString(name); ok {
		return v, true
	}
	if r.Bool(name) {
		return "", true
	}
	return "", false
}

func (r *ParseResult) AllStrings(name string) []string {
	if v, ok := r.allStrings[name]; ok {
		return v
	}
	if caseFold(name) {
		return r.allStrings[strings.ToLower(name)]
	}
	return nil
}

func (r *ParseResult) Num(name string) (int64, bool) {
	if v, ok := r.nums[name]; ok {
		return v, true
	}
	if caseFold(name) {
		v, ok := r.nums[strings.ToLower(name)]
		return v, ok
	}
	return 0, false
}

func (r *ParseResult) Order() []string {
	return r.order
}

type ParseError struct{ Msg string }

func (e *ParseError) Error() string { return e.Msg }

func (s *Spec) ParseArgs(argv []string) (*ParseResult, error) {
	r := &ParseResult{
		bools:      map[string]bool{},
		strings:    map[string]string{},
		allStrings: map[string][]string{},
		nums:       map[string]int64{},
	}

	i := 0
	for i < len(argv) {
		a := argv[i]

		if a == "--" {
			r.Operands = append(r.Operands, argv[i+1:]...)
			return r, nil
		}

		if !isOption(a) {
			if s.stopAtFirstOperand {
				r.Operands = append(r.Operands, argv[i:]...)
				return r, nil
			}
			r.Operands = append(r.Operands, a)
			i++
			continue
		}

		if strings.HasPrefix(a, "--") || strings.HasPrefix(a, "---") {
			consumed, err := s.parseLong(r, a, argv[i+1:])
			if err != nil {
				return nil, err
			}
			i += 1 + consumed
			continue
		}

		consumed, err := s.parseShortGroup(r, a[1:], argv[i+1:])
		if err != nil {
			return nil, err
		}
		i += 1 + consumed
	}

	return r, nil
}

func isOption(a string) bool {
	return len(a) > 1 && a[0] == '-'
}

func (f flagDef) keys() []string {
	var k []string
	if f.short != 0 {
		k = append(k, string(f.short))
	}
	if f.long != "" {
		k = append(k, f.long)
		k = append(k, strings.ToLower(f.long))
	}
	return k
}

func (s *Spec) parseLong(r *ParseResult, tok string, rest []string) (int, error) {
	name := strings.TrimLeft(tok, "-")
	var inlineVal string
	hasInline := false
	if eq := strings.IndexByte(name, '='); eq >= 0 {
		inlineVal = name[eq+1:]
		name = name[:eq]
		hasInline = true
	}
	nameLower := strings.ToLower(name)

	if nameLower == "help" && !hasInline {
		if _, defined := s.byLong["help"]; !defined {
			r.HelpRequested = true
			return 0, nil
		}
	}
	if nameLower == "version" && !hasInline {
		if _, defined := s.byLong["version"]; !defined {
			r.VersionRequested = true
			return 0, nil
		}
	}

	def, ok := s.byLong[nameLower]
	if !ok {
		def, ok = s.matchLongPrefix(nameLower)
		if !ok {
			return 0, &ParseError{Msg: fmt.Sprintf("unknown option --%s", name)}
		}
	}

	if def.kind == kindBool {
		if hasInline {
			return 0, &ParseError{Msg: fmt.Sprintf("option --%s takes no argument", name)}
		}
		for _, k := range def.keys() {
			r.bools[k] = true
		}
		r.order = append(r.order, def.key())
		return 0, nil
	}

	if def.kind == kindOptString {
		for _, k := range def.keys() {
			r.bools[k] = true
			if hasInline {
				r.strings[k] = inlineVal
			}
		}
		r.order = append(r.order, def.key())
		return 0, nil
	}

	val := inlineVal
	consumed := 0
	if !hasInline {
		if len(rest) == 0 {
			return 0, &ParseError{Msg: fmt.Sprintf("option --%s requires an argument", name)}
		}
		val = rest[0]
		consumed = 1
	}
	return consumed, s.storeValue(r, def, val)
}

func (s *Spec) parseShortGroup(r *ParseResult, group string, rest []string) (int, error) {
	for j := 0; j < len(group); j++ {
		c := group[j]
		if c == 'h' {
			if _, defined := s.byShort['h']; !defined {
				r.HelpRequested = true
				continue
			}
		}
		def, ok := s.byShort[c]
		if !ok {
			return 0, &ParseError{Msg: fmt.Sprintf("unknown option -%c", c)}
		}

		if def.kind == kindBool {
			for _, k := range def.keys() {
				r.bools[k] = true
			}
			r.order = append(r.order, def.key())
			continue
		}

		if def.kind == kindOptString {
			for _, k := range def.keys() {
				r.bools[k] = true
				if j+1 < len(group) {
					r.strings[k] = group[j+1:]
				}
			}
			r.order = append(r.order, def.key())
			return 0, nil
		}

		if j+1 < len(group) {
			return 0, s.storeValue(r, def, group[j+1:])
		}
		if len(rest) == 0 {
			return 0, &ParseError{Msg: fmt.Sprintf("option -%c requires an argument", c)}
		}
		return 1, s.storeValue(r, def, rest[0])
	}
	return 0, nil
}

func (s *Spec) storeValue(r *ParseResult, def flagDef, val string) error {
	keys := def.keys()
	switch def.kind {
	case kindString:
		for _, k := range keys {
			r.strings[k] = val
			r.allStrings[k] = append(r.allStrings[k], val)
		}
	case kindNum:
		trimmed := strings.TrimLeft(val, " \t")
		numStr, mult := parseSizeSuffix(trimmed)
		n, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return &ParseError{Msg: fmt.Sprintf("invalid value '%s' for option %s", val, flagDesc(def))}
		}
		for _, k := range keys {
			r.nums[k] = n * mult
		}
	}
	r.order = append(r.order, def.key())
	return nil
}

func (s *Spec) matchLongPrefix(prefix string) (flagDef, bool) {
	var match flagDef
	count := 0
	for longName, def := range s.byLong {
		if strings.HasPrefix(longName, prefix) {
			match = def
			count++
		}
	}
	if count == 1 {
		return match, true
	}
	return flagDef{}, false
}

func flagDesc(def flagDef) string {
	switch {
	case def.short != 0 && def.long != "":
		return fmt.Sprintf("-%c/--%s", def.short, def.long)
	case def.short != 0:
		return fmt.Sprintf("-%c", def.short)
	default:
		return "--" + def.long
	}
}

func (s *Spec) GenerateUsage(name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s [OPTION]... [OPERAND]...\n\n", name)

	type entry struct {
		short byte
		long  string
		kind  argKind
	}
	seen := map[string]bool{}
	var entries []entry
	for _, def := range s.byShort {
		key := def.key()
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry{def.short, def.long, def.kind})
	}
	for _, def := range s.byLong {
		key := def.key()
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry{def.short, def.long, def.kind})
	}

	for _, e := range entries {
		var left string
		switch {
		case e.short != 0 && e.long != "":
			left = fmt.Sprintf("  -%c, --%s", e.short, e.long)
		case e.short != 0:
			left = fmt.Sprintf("  -%c", e.short)
		default:
			left = fmt.Sprintf("      --%s", e.long)
		}
		if e.kind == kindString {
			left += "=VALUE"
		} else if e.kind == kindOptString {
			left += "[=VALUE]"
		} else if e.kind == kindNum {
			left += "=N"
		}
		fmt.Fprintln(&b, left)
	}
	fmt.Fprintf(&b, "  -h, --help     display this help and exit\n")
	fmt.Fprintf(&b, "      --version  output version information and exit\n")
	return b.String()
}

func HandleHelpVersion(ctx *Context, flags *ParseResult, spec *Spec) bool {
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, spec.GenerateUsage(ctx.CmdName))
		return true
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", ctx.CmdName)
		return true
	}
	return false
}
