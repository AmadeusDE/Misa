package builtins

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const seqSpec = "w(equal-width)s(separator):f(format):t(terminator):"

var seqSpecParsed *Spec

func init() {
	var err error
	seqSpecParsed, err = ParseSpec(seqSpec)
	if err != nil {
		panic("builtins: bad seq spec: " + err.Error())
	}
	Register(Command{Name: "seq", Run: runSeq})
}

func seqIsNumArg(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	return (s[1] >= '0' && s[1] <= '9') || s[1] == '.'
}

func runSeq(ctx *Context) int {
	var flagArgs []string
	var operands []string
	for i := 0; i < len(ctx.Args); i++ {
		a := ctx.Args[i]
		if a == "--" {
			operands = append(operands, ctx.Args[i+1:]...)
			break
		}
		if seqIsNumArg(a) {
			operands = append(operands, ctx.Args[i:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flagArgs = append(flagArgs, a)
			needsNext := false
			for j := 1; j < len(a); j++ {
				if a[j] == 's' || a[j] == 'f' || a[j] == 't' {
					if j+1 >= len(a) {
						needsNext = true
					}
					break
				}
			}
			if needsNext && i+1 < len(ctx.Args) {
				i++
				flagArgs = append(flagArgs, ctx.Args[i])
			}
		} else {
			operands = append(operands, ctx.Args[i:]...)
			break
		}
	}

	flags, err := seqSpecParsed.ParseArgs(flagArgs)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "seq:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, seqSpecParsed) {
		return 0
	}

	ops := operands
	if len(ops) == 0 || len(ops) > 3 {
		fmt.Fprintln(ctx.Stderr, "seq: missing operand")
		return 1
	}

	isBig := false
	for _, op := range ops {
		s := strings.TrimSpace(op)
		if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
			s = s[1:]
		}
		if len(s) > 15 && !strings.ContainsAny(s, ".eEpP") {
			isBig = true
		}
	}
	if isBig {
		return runSeqBig(ctx, ops, flags)
	}

	var first, incr, last float64
	badArg := ""
	switch len(ops) {
	case 1:
		first, incr = 1, 1
		last, err = seqParseNumber(ops[0])
		badArg = ops[0]
	case 2:
		first, err = seqParseNumber(ops[0])
		badArg = ops[0]
		if err == nil {
			last, err = seqParseNumber(ops[1])
			badArg = ops[1]
		}
		incr = 1
	case 3:
		first, err = seqParseNumber(ops[0])
		badArg = ops[0]
		if err == nil {
			incr, err = seqParseNumber(ops[1])
			badArg = ops[1]
		}
		if err == nil {
			last, err = seqParseNumber(ops[2])
			badArg = ops[2]
		}
	}
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "seq: invalid floating point argument: '%s'\n", badArg)
		return 1
	}

	if math.IsNaN(first) || math.IsNaN(incr) || math.IsNaN(last) {
		fmt.Fprintln(ctx.Stderr, "seq: invalid 'not-a-number' argument: 'nan'")
		fmt.Fprintln(ctx.Stderr, "Try 'seq --help' for more information.")
		return 1
	}

	if incr == 0 {
		fmt.Fprintln(ctx.Stderr, "seq: invalid Zero increment value: '0'")
		fmt.Fprintln(ctx.Stderr, "Try 'seq --help' for more information.")
		return 1
	}

	sep := "\n"
	if s, ok := flags.String("s"); ok {
		sep = s
	}
	term := "\n"
	if t, ok := flags.String("t"); ok {
		term = t
	}

	hasHexFloat := false
	for _, op := range ops {
		h := strings.ToLower(strings.TrimSpace(op))
		if (strings.HasPrefix(h, "0x") || strings.HasPrefix(h, "-0x") || strings.HasPrefix(h, "+0x")) && strings.Contains(h, ".") {
			hasHexFloat = true
		}
	}

	var precOps []string
	if len(ops) > 1 {
		precOps = ops[:len(ops)-1]
	}

	maxPrec := 0
	for _, s := range precOps {
		head := strings.ToLower(strings.TrimSpace(s))
		if strings.HasPrefix(head, "+") || strings.HasPrefix(head, "-") {
			head = head[1:]
		}
		if strings.HasPrefix(head, "0x") {
			continue
		}
		p := seqPrecision(s)
		if p > maxPrec {
			maxPrec = p
		}
	}

	isInt := maxPrec == 0 && first == math.Trunc(first) && incr == math.Trunc(incr) && last == math.Trunc(last)

	fmtStr := "%g"
	if hasHexFloat {
		fmtStr = "%g"
	} else if isInt {
		fmtStr = "%.0f"
	} else if maxPrec > 0 {
		fmtStr = fmt.Sprintf("%%.%df", maxPrec)
	}

	if flags.Bool("w") && !hasHexFloat {
		if isInt {
			width := seqWidth(ops[0], maxPrec)
			if len(ops) >= 2 {
				if lw := seqWidth(ops[len(ops)-1], maxPrec); lw > width {
					width = lw
				}
			}
			fmtStr = fmt.Sprintf("%%0%d.0f", width)
		} else {
			width := max(seqLiteralWidth(ops[0], first, maxPrec), seqLiteralWidth(ops[len(ops)-1], last, maxPrec))
			fmtStr = fmt.Sprintf("%%0%d.%df", width, maxPrec)
		}
	}

	if f, ok := flags.String("f"); ok {
		if flags.Bool("w") {
			fmt.Fprintf(ctx.Stderr, "seq: format string may not be specified when printing equal width strings\n")
			return 1
		}
		reason := seqValidateFormat(f)
		if reason != "" {
			if strings.HasSuffix(reason, "invalid conversion specification") {
				fmt.Fprintf(ctx.Stderr, "seq: %s\n", reason)
			} else {
				fmt.Fprintf(ctx.Stderr, "seq: format '%s' %s\n", f, reason)
			}
			return 1
		}
		fmtStr = f
		if f == "%g" {
			fmtStr = "%.6g"
		} else if f == "%E" {
			fmtStr = "%.6E"
		} else if f == "%e" {
			fmtStr = "%.6e"
		} else if f == "%f" {
			fmtStr = "%.6f"
		}
	}

	printed := false
	prevV := first
	for step := 0; ; step++ {
		var v float64
		if step == 0 {
			v = first
		} else {
			v = first + float64(step)*incr
		}

		if step > 0 && v == prevV {
			break
		}
		prevV = v
		if incr > 0 {
			if v > last+1e-9 {
				break
			}
		} else if v < last-1e-9 {
			break
		}
		if printed {
			if _, err := fmt.Fprint(ctx.Stdout, sep); err != nil {
				fmt.Fprintf(ctx.Stderr, "seq: write error: %v\n", err)
				return 1
			}
		}
		if _, err := fmt.Fprintf(ctx.Stdout, fmtStr, v); err != nil {
			fmt.Fprintf(ctx.Stderr, "seq: write error: %v\n", err)
			return 1
		}
		printed = true
	}
	if printed {
		if _, err := fmt.Fprint(ctx.Stdout, term); err != nil {
			fmt.Fprintf(ctx.Stderr, "seq: write error: %v\n", err)
			return 1
		}
	}
	return 0
}

func runSeqBig(ctx *Context, ops []string, flags *ParseResult) int {
	var first, incr, last big.Int
	switch len(ops) {
	case 1:
		first.SetInt64(1)
		incr.SetInt64(1)
		if _, ok := last.SetString(ops[0], 10); !ok {
			fmt.Fprintf(ctx.Stderr, "seq: invalid floating point argument: '%s'\n", ops[0])
			return 1
		}
	case 2:
		if _, ok := first.SetString(ops[0], 10); !ok {
			fmt.Fprintf(ctx.Stderr, "seq: invalid floating point argument: '%s'\n", ops[0])
			return 1
		}
		incr.SetInt64(1)
		if _, ok := last.SetString(ops[1], 10); !ok {
			fmt.Fprintf(ctx.Stderr, "seq: invalid floating point argument: '%s'\n", ops[1])
			return 1
		}
	case 3:
		if _, ok := first.SetString(ops[0], 10); !ok {
			fmt.Fprintf(ctx.Stderr, "seq: invalid floating point argument: '%s'\n", ops[0])
			return 1
		}
		if _, ok := incr.SetString(ops[1], 10); !ok {
			fmt.Fprintf(ctx.Stderr, "seq: invalid floating point argument: '%s'\n", ops[1])
			return 1
		}
		if _, ok := last.SetString(ops[2], 10); !ok {
			fmt.Fprintf(ctx.Stderr, "seq: invalid floating point argument: '%s'\n", ops[2])
			return 1
		}
	}
	if incr.Sign() == 0 {
		fmt.Fprintln(ctx.Stderr, "seq: invalid Zero increment value: '0'")
		return 1
	}

	sep := "\n"
	if s, ok := flags.String("s"); ok {
		sep = s
	}
	term := "\n"
	if t, ok := flags.String("t"); ok {
		term = t
	}

	printed := false
	curr := new(big.Int).Set(&first)
	for {
		if incr.Sign() > 0 && curr.Cmp(&last) > 0 {
			break
		}
		if incr.Sign() < 0 && curr.Cmp(&last) < 0 {
			break
		}
		if printed {
			fmt.Fprint(ctx.Stdout, sep)
		}
		fmt.Fprint(ctx.Stdout, curr.String())
		printed = true
		curr.Add(curr, &incr)
	}
	if printed {
		fmt.Fprint(ctx.Stdout, term)
	}
	return 0
}

func seqParseNumber(s string) (float64, error) {
	s = strings.TrimSpace(s)
	head := strings.ToLower(s)
	if strings.HasPrefix(head, "+") || strings.HasPrefix(head, "-") {
		head = head[1:]
	}
	if strings.HasPrefix(head, "0x") {
		if strings.ContainsAny(head, "p.") {
			return parseHexFloat(s)
		}
		n, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			return 0, err
		}
		return float64(n), nil
	}
	return strconv.ParseFloat(s, 64)
}

func parseHexFloat(s string) (float64, error) {
	sign := 1.0
	if s[0] == '+' {
		s = s[1:]
	} else if s[0] == '-' {
		sign = -1
		s = s[1:]
	}
	if len(s) < 2 || (s[0] != '0' || (s[1] != 'x' && s[1] != 'X')) {
		return 0, strconv.ErrSyntax
	}
	s = s[2:]
	exp := 0
	mantissa := s
	if pIdx := strings.IndexAny(s, "pP"); pIdx >= 0 {
		mantissa = s[:pIdx]
		expStr := s[pIdx+1:]
		if expStr == "" {
			return 0, strconv.ErrSyntax
		}
		var err error
		exp, err = strconv.Atoi(expStr)
		if err != nil {
			return 0, err
		}
	}
	intPart, fracPart := mantissa, ""
	if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
		intPart, fracPart = mantissa[:dot], mantissa[dot+1:]
	}
	if intPart == "" && fracPart == "" {
		return 0, strconv.ErrSyntax
	}
	val := 0.0
	if intPart != "" {
		n, err := strconv.ParseUint(intPart, 16, 64)
		if err != nil {
			return 0, err
		}
		val = float64(n)
	}
	place := 16.0
	for i := 0; i < len(fracPart); i++ {
		c := fracPart[i]
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return 0, strconv.ErrSyntax
		}
		val += float64(d) / place
		place *= 16
	}
	return sign * val * math.Pow(2, float64(exp)), nil
}

func seqWidth(arg string, maxPrec int) int {
	s := arg
	if idx := strings.IndexAny(s, "eE"); idx >= 0 {
		head := s[:idx]
		expVal, err := strconv.Atoi(s[idx+1:])
		if err == nil {
			if expVal > 0 {
				dot := strings.IndexByte(head, '.')
				if dot >= 0 {
					intDigits := len(head[:dot])
					fracDigits := len(head[dot+1:])
					if expVal >= fracDigits {
						return intDigits + expVal
					}
				} else {
					return len(head) + expVal
				}
			}
		}
		v, parseErr := seqParseNumber(s)
		if parseErr == nil {
			p := seqPrecision(s)
			s = strconv.FormatFloat(v, 'f', p, 64)
		}
	}
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if maxPrec == 0 {
		if dot := strings.IndexByte(s, '.'); dot >= 0 {
			s = s[:dot]
		}
	}
	width := len(s)
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return width
	}
	frac := s[dot+1:]
	if len(frac) == 0 {
		width--
	} else if dot == 0 || s[dot-1] < '0' || s[dot-1] > '9' {
		width++
	}
	return width
}

func seqPrecision(s string) int {
	s = strings.ToLower(s)
	exp := 0
	if idx := strings.IndexByte(s, 'e'); idx >= 0 {
		expVal, err := strconv.Atoi(s[idx+1:])
		if err == nil {
			exp = expVal
		}
		s = s[:idx]
	}
	prec := 0
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		prec = len(s) - dot - 1
	}
	res := prec - exp
	if res < 0 {
		return 0
	}
	return res
}

func seqValidateFormat(f string) string {
	count := 0
	i := 0
	for i < len(f) {
		if f[i] == '%' {
			startSpec := i
			i++
			if i >= len(f) {
				if count >= 1 {
					return "has too many % directives"
				}
				return "ends in %"
			}
			if f[i] == '%' {
				i++
				continue
			}
			count++
			if count > 1 {
				return "has too many % directives"
			}
			for i < len(f) && (f[i] == '-' || f[i] == '+' || f[i] == ' ' || f[i] == '\'' || f[i] == '#') {
				i++
			}
			for i < len(f) && f[i] >= '0' && f[i] <= '9' {
				i++
			}
			if i < len(f) && f[i] == '.' {
				i++
				precStart := i
				for i < len(f) && f[i] >= '0' && f[i] <= '9' {
					i++
				}
				if precStr := f[precStart:i]; len(precStr) > 9 {
					return "has invalid precision"
				}
			}
			if i >= len(f) {
				return "ends in %"
			}
			if f[i] != 'f' && f[i] != 'e' && f[i] != 'g' && f[i] != 'E' && f[i] != 'G' && f[i] != 'F' {
				specStr := f[startSpec : i+1]
				return fmt.Sprintf("%s: invalid conversion specification", specStr)
			}
			i++
		} else {
			i++
		}
	}
	if count == 0 {
		return "has no % directive"
	}
	return ""
}

// seqLiteralWidth is GNU's -w width for a fixed-point operand: its literal
// length, widened by the fraction digits (and '.') the shared precision adds.
// Exponent forms fall back to the formatted value's length.
func seqLiteralWidth(lit string, v float64, prec int) int {
	if strings.ContainsAny(lit, "eE") {
		return len(fmt.Sprintf("%.*f", prec, v))
	}
	own := seqPrecision(lit)
	digits := strings.TrimPrefix(lit, "+")
	width := len(digits) + prec - own
	if strings.HasPrefix(strings.TrimPrefix(digits, "-"), ".") {
		width++ // the implied leading 0
	}
	if own == 0 && prec > 0 && !strings.Contains(lit, ".") {
		width++
	}
	return width
}
