// Behaviour adapted from uutils coreutils numfmt (MIT).
package builtins

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func init() { Register(Command{Name: "numfmt", Run: runNumfmt}) }

type numfmtRange struct{ first, last int }
type numfmtOptions struct {
	from, to, round, invalid                            string
	fromUnit, toUnit                                    *big.Rat
	suffix, separator, delimiter                        string
	explicitSeparator, delimited, zero, debug, grouping bool
	padding, header                                     int
	fields                                              []numfmtRange
	format                                              numfmtFormat
}

func numfmtChoice(value string, choices ...string) (string, bool) {
	found := ""
	for _, choice := range choices {
		if value == choice {
			return choice, true
		}
		if value != "" && strings.HasPrefix(choice, value) {
			if found != "" {
				return "", false
			}
			found = choice
		}
	}
	return found, found != ""
}

func runNumfmt(ctx *Context) int {
	fail := func(err error) int { fmt.Fprintf(ctx.Stderr, "numfmt: %s\n", err); return 1 }
	spec, _ := ParseSpec("(from):(to):(from-unit):(to-unit):(round):(invalid):(format):(padding):(header):?(field):d(delimiter):(suffix):(unit-separator):z(zero-terminated)(grouping)(debug)")
	args, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		return fail(fmt.Errorf("unexpected argument: %s", err))
	}
	if HandleHelpVersion(ctx, args, spec) {
		return 0
	}
	o := numfmtOptions{from: "none", to: "none", round: "from-zero", invalid: "abort", fromUnit: big.NewRat(1, 1), toUnit: big.NewRat(1, 1), separator: " ", fields: []numfmtRange{{1, 1}}, format: numfmtFormat{precision: -1}}
	for _, key := range []string{"from", "to", "round", "invalid"} {
		if v, ok := args.String(key); ok {
			choices := []string{"none", "si", "iec", "iec-i", "auto"}
			if key == "to" {
				choices = choices[:4]
			}
			if key == "round" {
				choices = []string{"from-zero", "towards-zero", "up", "down", "nearest"}
			}
			if key == "invalid" {
				choices = []string{"abort", "fail", "warn", "ignore"}
			}
			c, valid := numfmtChoice(v, choices...)
			if !valid {
				return fail(fmt.Errorf("invalid argument '%s' for '--%s'", v, key))
			}
			switch key {
			case "from":
				o.from = c
			case "to":
				o.to = c
			case "round":
				o.round = c
			case "invalid":
				o.invalid = c
			}
		}
	}
	for _, key := range []string{"from-unit", "to-unit"} {
		if v, ok := args.String(key); ok {
			unit := numfmtParseUnit(v)
			if unit == nil {
				return fail(fmt.Errorf("invalid unit size: '%s'", v))
			}
			if key == "from-unit" {
				o.fromUnit = unit
			} else {
				o.toUnit = unit
			}
		}
	}
	if v, ok := args.String("padding"); ok {
		n, e := strconv.Atoi(v)
		if e != nil || n == 0 || n > 1000000 || n < -1000000 {
			return fail(fmt.Errorf("invalid padding value '%s'", v))
		}
		o.padding = n
	}
	if v, ok := args.String("header"); ok {
		if v == "" {
			o.header = 1
		} else {
			n, e := strconv.Atoi(v)
			if e != nil || n < 1 {
				return fail(fmt.Errorf("invalid header value '%s'", v))
			}
			o.header = n
		}
	}
	if v, ok := args.String("field"); ok {
		o.fields, err = numfmtParseFields(v)
		if err != nil {
			return fail(err)
		}
	}
	if v, ok := args.String("d"); ok {
		v = strings.TrimPrefix(v, "=")
		if utf8.RuneCountInString(v) > 1 {
			return fail(fmt.Errorf("the delimiter must be a single character"))
		}
		o.delimited = true
		o.delimiter = v
	}
	o.suffix, _ = args.String("suffix")
	if v, ok := args.String("unit-separator"); ok {
		o.separator = v
		o.explicitSeparator = true
	}
	o.zero = args.Bool("z")
	o.debug = args.Bool("debug")
	o.grouping = args.Bool("grouping")
	if v, ok := args.String("format"); ok {
		o.format, err = numfmtParseFormat(v)
		if err != nil {
			return fail(err)
		}
		if o.grouping {
			return fail(fmt.Errorf("--grouping cannot be combined with --format"))
		}
	}
	if (o.grouping || o.format.grouping) && o.to != "none" {
		return fail(fmt.Errorf("grouping cannot be combined with --to"))
	}
	if o.debug {
		if o.from == "none" && o.to == "none" && o.padding == 0 && !o.grouping && !o.format.present && o.fromUnit.Cmp(big.NewRat(1, 1)) == 0 && o.toUnit.Cmp(big.NewRat(1, 1)) == 0 {
			fmt.Fprintln(ctx.Stderr, "numfmt: no conversion option specified")
		}
		if o.header > 0 && len(args.Operands) > 0 {
			fmt.Fprintln(ctx.Stderr, "numfmt: --header ignored with command-line input")
		}
		if o.grouping || o.format.grouping {
			fmt.Fprintln(ctx.Stderr, "numfmt: grouping has no effect in this locale")
		}
	}
	terminator := byte('\n')
	if o.zero {
		terminator = 0
	}
	status := 0
	sawInvalid := false
	process := func(line string, eol bool) bool {
		if i := strings.IndexByte(line, 0); i >= 0 {
			line = line[:i]
		}
		output, conversionErr := o.line(line)
		if conversionErr != nil {
			sawInvalid = true
			if o.invalid != "ignore" {
				fmt.Fprintf(ctx.Stderr, "numfmt: %s\n", conversionErr)
			}
			if o.invalid == "abort" {
				status = 2
				if _, e := io.WriteString(ctx.Stdout, output); e != nil {
					status = 1
				}
				return false
			}
			output = line
			if o.invalid == "fail" {
				status = 2
			}
		}
		if eol {
			output += string(terminator)
		}
		if _, e := io.WriteString(ctx.Stdout, output); e != nil {
			fmt.Fprintf(ctx.Stderr, "numfmt: write error: %s\n", e)
			status = 1
			return false
		}
		return true
	}
	if len(args.Operands) > 0 {
		for _, line := range args.Operands {
			if !process(line, true) {
				return status
			}
		}
	} else {
		reader := bufio.NewReader(ctx.Stdin)
		n := 0
		for {
			line, e := reader.ReadString(terminator)
			if len(line) > 0 {
				eol := line[len(line)-1] == terminator
				if eol {
					line = line[:len(line)-1]
				}
				n++
				if n <= o.header {
					if eol {
						line += string(terminator)
					}
					if _, err := io.WriteString(ctx.Stdout, line); err != nil {
						return fail(err)
					}
				} else if !process(line, eol) {
					return status
				}
			}
			if e != nil {
				if e != io.EOF {
					return fail(e)
				}
				break
			}
		}
	}
	if o.debug && sawInvalid {
		fmt.Fprintln(ctx.Stderr, "numfmt: failed to convert some of the input numbers")
	}
	return status
}

func numfmtParseUnit(s string) *big.Rat {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n := big.NewInt(1)
	if i > 0 {
		var ok bool
		n, ok = new(big.Int).SetString(s[:i], 10)
		if !ok || n.Sign() <= 0 {
			return nil
		}
	}
	suffix := s[i:]
	if suffix != "" {
		index := strings.IndexByte("KMGTPEZYRQ", suffix[0])
		if index < 0 {
			return nil
		}
		base := int64(1000)
		if suffix[1:] == "i" {
			base = 1024
		} else if len(suffix) > 1 {
			return nil
		}
		n.Mul(n, new(big.Int).Exp(big.NewInt(base), big.NewInt(int64(index+1)), nil))
	}
	if n.BitLen() > 64 {
		return nil
	}
	return new(big.Rat).SetInt(n)
}

func numfmtParseFields(s string) ([]numfmtRange, error) {
	var result []numfmtRange
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	for _, part := range parts {
		a, b, rangeFound := strings.Cut(part, "-")
		first, last := 1, int(^uint(0)>>1)
		var err error
		if a != "" {
			first, err = strconv.Atoi(a)
			if err != nil || first < 1 {
				return nil, fmt.Errorf("invalid field range: '%s'", s)
			}
		}
		if rangeFound {
			if b != "" {
				last, err = strconv.Atoi(b)
				if err != nil || last < first {
					return nil, fmt.Errorf("invalid field range: '%s'", s)
				}
			}
		} else {
			last = first
		}
		result = append(result, numfmtRange{first, last})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("invalid field range: '%s'", s)
	}
	return result, nil
}
func (o *numfmtOptions) selected(n int) bool {
	for _, r := range o.fields {
		if n >= r.first && n <= r.last {
			return true
		}
	}
	return false
}

func (o *numfmtOptions) line(line string) (string, error) {
	var out strings.Builder
	if o.delimited {
		parts := []string{line}
		if o.delimiter != "" {
			parts = strings.Split(line, o.delimiter)
		}
		for i, field := range parts {
			if i > 0 {
				out.WriteString(o.delimiter)
			}
			if o.selected(i + 1) {
				s, err := o.number(strings.TrimLeftFunc(field, unicode.IsSpace), 0)
				if err != nil {
					return out.String(), err
				}
				out.WriteString(s)
			} else {
				out.WriteString(field)
			}
		}
		return out.String(), nil
	}
	remaining := line
	for n := 1; remaining != ""; n++ {
		trimmed := strings.TrimLeftFunc(remaining, unicode.IsSpace)
		prefix := remaining[:len(remaining)-len(trimmed)]
		end := strings.IndexFunc(trimmed, unicode.IsSpace)
		if end < 0 {
			end = len(trimmed)
		}
		field := trimmed[:end]
		rest := trimmed[end:]
		if o.from != "none" && o.separator != "" && strings.HasPrefix(rest, o.separator) {
			tail := rest[len(o.separator):]
			suffixEnd := strings.IndexFunc(tail, unicode.IsSpace)
			if suffixEnd < 0 {
				suffixEnd = len(tail)
			}
			suffix := tail[:suffixEnd]
			if len(suffix) > 0 && strings.ContainsRune("KkMGTPEZYRQ", rune(suffix[0])) && (len(suffix) == 1 || len(suffix) == 2 && suffix[1] == 'i') {
				field += o.separator + suffix
				rest = tail[suffixEnd:]
			}
		}
		if o.selected(n) {
			implicit := 0
			if n > 1 {
				out.WriteByte(' ')
				if prefix != "" {
					_, size := utf8.DecodeRuneInString(prefix)
					prefix = prefix[size:]
				}
			}
			if o.padding == 0 && len(trimmed) != len(remaining) {
				implicit = utf8.RuneCountInString(prefix + field)
			}
			value, err := o.number(field, implicit)
			if err != nil {
				return out.String(), err
			}
			out.WriteString(value)
		} else {
			if o.zero && strings.HasPrefix(prefix, "\n") {
				prefix = " " + prefix[1:]
			}
			out.WriteString(prefix)
			out.WriteString(field)
		}
		remaining = rest
	}
	if line == "" {
		return "", fmt.Errorf("invalid number: ''")
	}
	return out.String(), nil
}
