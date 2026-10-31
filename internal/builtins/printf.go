package builtins

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func init() {
	Register(Command{Name: "printf", Run: runPrintf})
}

func printfParseNum(s string) (int64, error) {
	return printfParseNumWarn(s, nil, nil)
}

func printfParseNumWarn(s string, ctx *Context, exitCode *int) (int64, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') {
		r, sz := utf8.DecodeRuneInString(s[1:])
		if 1+sz < len(s) && ctx != nil {
			trailing := s[1+sz:]
			fmt.Fprintf(ctx.Stderr, "printf: warning: %s: character(s) following character constant have been ignored\n", trailing)
			if exitCode != nil {
				*exitCode = 1
			}
		}
		return int64(r), nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if s[0] == '+' {
		s = s[1:]
	}
	return strconv.ParseInt(s, 0, 64)
}

func printfNumErrMsg(arg string, err error) string {
	var numErr *strconv.NumError
	if errors.As(err, &numErr) && numErr.Err == strconv.ErrRange {
		return fmt.Sprintf("printf: '%s': Numerical result out of range\n", arg)
	}
	return fmt.Sprintf("printf: '%s': expected a numeric value\n", arg)
}

func printfParseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') {
		r, _ := utf8.DecodeRuneInString(s[1:])
		return float64(r), nil
	}
	if s == "" {
		return 0, nil
	}
	if s[0] == '+' {
		s = s[1:]
	}
	return strconv.ParseFloat(s, 64)
}

func runPrintf(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	pArgs := ctx.Args
	if len(pArgs) > 0 && pArgs[0] == "--" {
		pArgs = pArgs[1:]
	}
	if len(pArgs) == 0 {
		fmt.Fprintln(ctx.Stderr, "printf: missing format")
		return 1
	}

	format := pArgs[0]
	args := pArgs[1:]
	argIdx := 0
	exitCode := 0

	nextArg := func() string {
		if argIdx < len(args) {
			s := args[argIdx]
			argIdx++
			return s
		}
		return ""
	}

	for {
		startArgIdx := argIdx
		i := 0
		for i < len(format) {
			if err := ctx.CancellationContext().Err(); err != nil {
				return ExitStatus(err)
			}
			if format[i] == '\\' {
				ch, n, stop := printfEscape(format[i:])
				if ch == "\\x" && n == 2 {
					fmt.Fprintln(ctx.Stderr, "printf: missing hexadecimal number in escape")
					return 1
				}
				if _, err := fmt.Fprint(ctx.Stdout, ch); err != nil {
					return 1
				}
				if stop {
					return exitCode
				}
				i += n
				continue
			}

			if format[i] == '%' {
				if i+1 >= len(format) {
					fmt.Fprintf(ctx.Stderr, "printf: %%: invalid format\n")
					return 1
				}

				spec, n := parsePrintfSpec(format[i:])
				if spec == "%%" {
					if _, err := fmt.Fprint(ctx.Stdout, "%"); err != nil {
						return 1
					}
					i += n
					continue
				}

				conv := spec[len(spec)-1]
				if conv == '%' || !isConvChar(conv) {
					fmt.Fprintf(ctx.Stderr, "printf: %%%c: invalid format\n", conv)
					return 1
				}

				if conv == 'c' && strings.Contains(spec, ".") {
					fmt.Fprintf(ctx.Stderr, "printf: %s: invalid conversion specification\n", spec)
					return 1
				}

				if err := validatePrintfFlags(spec); err != "" {
					fmt.Fprintf(ctx.Stderr, "printf: %s: invalid conversion specification\n", err)
					return 1
				}

				resolvedSpec, stop := resolvePrintfSpec(ctx, spec, nextArg, &exitCode)
				if stop {
					return exitCode
				}
				arg := nextArg()
				result, stopped := applyPrintfSpec(ctx, resolvedSpec, arg, &exitCode)
				if _, err := fmt.Fprint(ctx.Stdout, result); err != nil {
					return 1
				}
				if stopped {
					return exitCode
				}
				i += n
				continue
			}

			r, size := utf8.DecodeRuneInString(format[i:])
			if _, err := fmt.Fprint(ctx.Stdout, string(r)); err != nil {
				return 1
			}
			i += size
		}

		if argIdx <= startArgIdx || argIdx >= len(args) {
			break
		}
	}

	return exitCode
}

func validatePrintfFlags(spec string) string {
	if len(spec) < 2 {
		return ""
	}
	conv := spec[len(spec)-1]
	flags := ""
	for j := 1; j < len(spec)-1; j++ {
		c := spec[j]
		if c == '-' || c == '+' || c == ' ' || c == '0' || c == '#' || c == '\'' {
			flags += string(c)
		} else {
			break
		}
	}
	switch conv {
	case 'd', 'i', 'u':
		if strings.ContainsRune(flags, '#') {
			return spec
		}
	case 's':
		if strings.ContainsRune(flags, '0') || strings.ContainsRune(flags, '\'') {
			return spec
		}
	}
	return ""
}

func isConvChar(c byte) bool {
	switch c {
	case 'd', 'i', 'o', 'u', 'x', 'X', 'f', 'F', 'e', 'E', 'g', 'G', 'a', 'A', 'c', 's', 'b', 'q':
		return true
	}
	return false
}

func printfEscape(s string) (string, int, bool) {
	if len(s) < 2 {
		return "\\", 1, false
	}
	switch s[1] {
	case '\\':
		return "\\", 2, false
	case 'a':
		return "\a", 2, false
	case 'b':
		return "\b", 2, false
	case 'c':
		return "", 2, true
	case 'e':
		return "\x1b", 2, false
	case 'f':
		return "\f", 2, false
	case 'n':
		return "\n", 2, false
	case 'r':
		return "\r", 2, false
	case 't':
		return "\t", 2, false
	case 'v':
		return "\v", 2, false
	case '0', '1', '2', '3', '4', '5', '6', '7':
		val := 0
		j := 1
		count := 0
		for count < 3 && j < len(s) && s[j] >= '0' && s[j] <= '7' {
			val = val*8 + int(s[j]-'0')
			j++
			count++
		}
		return string([]byte{byte(val)}), j, false
	case 'x':
		val := 0
		j := 2
		count := 0
		for k := 0; k < 2 && j < len(s); k++ {
			c := s[j]
			var digit int
			switch {
			case c >= '0' && c <= '9':
				digit = int(c - '0')
			case c >= 'a' && c <= 'f':
				digit = int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				digit = int(c-'A') + 10
			default:
				goto xdone
			}
			val = val*16 + digit
			j++
			count++
		}
	xdone:
		if count == 0 {
			return "\\x", 2, false
		}
		return string([]byte{byte(val)}), j, false
	default:
		return "\\" + string(s[1]), 2, false
	}
}

func parsePrintfSpec(s string) (string, int) {
	i := 1 // skip %

	if i < len(s) && s[i] == '%' {
		return "%%", 2
	}

	for i < len(s) && (s[i] == '-' || s[i] == '+' || s[i] == ' ' || s[i] == '0' || s[i] == '#' || s[i] == '\'') {
		i++
	}
	if i < len(s) && s[i] == '*' {
		i++
	} else {
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i < len(s) && s[i] == '.' {
		i++
		if i < len(s) && s[i] == '*' {
			i++
		} else {
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
		}
	}
	for i+1 < len(s) && (s[i] == 'z' || s[i] == 'l' || s[i] == 'L' || s[i] == 'h' || s[i] == 'j' || s[i] == 't') {
		i++
	}
	if i < len(s) {
		i++ // conversion char
	}
	return s[:i], i
}

func resolvePrintfSpec(ctx *Context, spec string, nextArg func() string, exitCode *int) (string, bool) {
	conv := spec[len(spec)-1]
	prefix := spec[:len(spec)-1]

	var out strings.Builder
	out.WriteByte('%')

	i := 1
	for i < len(prefix) && (prefix[i] == '-' || prefix[i] == '+' || prefix[i] == ' ' || prefix[i] == '0' || prefix[i] == '#' || prefix[i] == '\'') {
		if prefix[i] != '\'' {
			out.WriteByte(prefix[i])
		}
		i++
	}

	if i < len(prefix) && prefix[i] == '*' {
		wArg := nextArg()
		w, err := printfParseNum(wArg)
		if err != nil {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(wArg, err))
			*exitCode = 1
		}
		if strings.TrimSpace(wArg) == "" {
			fmt.Fprintf(ctx.Stderr, "printf: '%s': expected a numeric value\n", wArg)
			*exitCode = 1
		}
		if w < 0 {
			out.WriteByte('-')
			w = -w
		}
		out.WriteString(strconv.FormatInt(w, 10))
		i++
	} else {
		for i < len(prefix) && prefix[i] >= '0' && prefix[i] <= '9' {
			out.WriteByte(prefix[i])
			i++
		}
	}

	if i < len(prefix) && prefix[i] == '.' {
		out.WriteByte('.')
		i++
		if i < len(prefix) && prefix[i] == '*' {
			pArg := nextArg()
			p, err := printfParseNum(pArg)
			if err != nil {
				fmt.Fprint(ctx.Stderr, printfNumErrMsg(pArg, err))
				*exitCode = 1
				return "", true
			}
			if strings.TrimSpace(pArg) == "" {
				fmt.Fprintf(ctx.Stderr, "printf: '%s': expected a numeric value\n", pArg)
				*exitCode = 1
			}
			if p > math.MaxInt32 {
				fmt.Fprintf(ctx.Stderr, "printf: invalid field width: '%s'\n", pArg)
				*exitCode = 1
				return "", true
			}
			if p < 0 {
				p = 0
				s := out.String()
				out.Reset()
				out.WriteString(s[:len(s)-1])
			} else {
				out.WriteString(strconv.FormatInt(p, 10))
			}
			i++
		} else {
			for i < len(prefix) && prefix[i] >= '0' && prefix[i] <= '9' {
				out.WriteByte(prefix[i])
				i++
			}
		}
	}
	// skip size modifiers
	for i < len(prefix) && (prefix[i] == 'z' || prefix[i] == 'l' || prefix[i] == 'L' || prefix[i] == 'h' || prefix[i] == 'j' || prefix[i] == 't') {
		i++
	}

	out.WriteByte(conv)
	return out.String(), false
}

func applyPrintfSpec(ctx *Context, spec, arg string, exitCode *int) (string, bool) {
	if len(spec) == 0 {
		return "", false
	}
	conv := spec[len(spec)-1]
	fmtStr := spec

	switch conv {
	case 's':
		return fmt.Sprintf(fmtStr, arg), false
	case 'q':
		return printfShellQuote(arg), false
	case 'b':
		expanded, stop := expandEscapes(arg)
		fmtPart := strings.TrimSuffix(fmtStr, "b") + "s"
		return fmt.Sprintf(fmtPart, expanded), stop
	case 'd', 'i':
		n, err := printfParseNumWarn(arg, ctx, exitCode)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		goFmt := fmtStr[:len(fmtStr)-1] + "d"
		return fmt.Sprintf(goFmt, n), false
	case 'u':
		n, err := printfParseNumWarn(arg, ctx, exitCode)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		var u uint64
		if err != nil {
			var numErr *strconv.NumError
			if errors.As(err, &numErr) && numErr.Err == strconv.ErrRange {
				u = math.MaxUint64
			}
		} else {
			u = uint64(n)
		}
		goFmt := fmtStr[:len(fmtStr)-1] + "d"
		return fmt.Sprintf(goFmt, u), false
	case 'o':
		n, err := printfParseNum(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		return fmt.Sprintf(fmtStr, n), false
	case 'x':
		n, err := printfParseNum(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		return fmt.Sprintf(fmtStr, n), false
	case 'X':
		n, err := printfParseNum(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		return fmt.Sprintf(fmtStr, n), false
	case 'a', 'A':
		f, err := printfParseFloat(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		goFmt := fmtStr[:len(fmtStr)-1] + "x"
		if conv == 'A' {
			goFmt = fmtStr[:len(fmtStr)-1] + "X"
		}
		return printfFixInf(fmt.Sprintf(goFmt, f), conv), false
	case 'f', 'F':
		f, err := printfParseFloat(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		goFmt := fmtStr[:len(fmtStr)-1] + "f"
		return printfFixInf(fmt.Sprintf(goFmt, f), conv), false
	case 'e':
		f, err := printfParseFloat(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		return printfFixInf(fmt.Sprintf(fmtStr, f), conv), false
	case 'E':
		f, err := printfParseFloat(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		return printfFixInf(fmt.Sprintf(fmtStr, f), conv), false
	case 'g':
		f, err := printfParseFloat(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		return printfFixInf(fmt.Sprintf(fmtStr, f), conv), false
	case 'G':
		f, err := printfParseFloat(arg)
		if err != nil && arg != "" {
			fmt.Fprint(ctx.Stderr, printfNumErrMsg(arg, err))
			*exitCode = 1
		}
		return printfFixInf(fmt.Sprintf(fmtStr, f), conv), false
	case 'c':
		if len(arg) > 0 {
			r, _ := utf8.DecodeRuneInString(arg)
			goFmt := fmtStr[:len(fmtStr)-1] + "c"
			return fmt.Sprintf(goFmt, r), false
		}
		goFmt := fmtStr[:len(fmtStr)-1] + "c"
		return fmt.Sprintf(goFmt, rune(0)), false
	default:
		return spec + arg, false
	}
}

// printfShellQuote is GNU's %q (quotearg's shell-escape style): bare when
// nothing needs quoting, "..." when only a ' does, otherwise '...' with
// $'...' runs for unprintable bytes and a backslash-escaped quote for each '.
func printfShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	needs, hasQuote, dqCompat := false, false, true
	for i, r := range s {
		switch {
		case !shellPrintable(r):
			needs, dqCompat = true, false
		case r == '\'':
			needs, hasQuote = true, true
		case r == ' ':
			needs = true
		case strings.ContainsRune("!\"$&()*;<=>?[\\^`|", r):
			needs, dqCompat = true, false
		case r == '{' || r == '}':
			needs = needs || len(s) == 1
			dqCompat = false
		case r == '#' || r == '~':
			needs = needs || i == 0
			dqCompat = false
		}
	}
	if !needs {
		return s
	}
	if hasQuote && dqCompat {
		return `"` + s + `"`
	}
	var b strings.Builder
	b.WriteByte('\'')
	dollar := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\'':
			b.WriteString(`'\''`)
			dollar = false
		case shellPrintable(r) && !(r == utf8.RuneError && size == 1):
			if dollar {
				b.WriteString("''")
				dollar = false
			}
			b.WriteString(s[i : i+size])
		default:
			if !dollar {
				b.WriteString(`'$'`)
				dollar = true
			}
			for _, c := range []byte(s[i : i+size]) {
				b.WriteString(shellEscapeByte(c))
			}
		}
		i += size
	}
	b.WriteByte('\'')
	return b.String()
}

func shellPrintable(r rune) bool {
	return r != utf8.RuneError && unicode.IsPrint(r)
}

func shellEscapeByte(c byte) string {
	if i := strings.IndexByte("\a\b\f\n\r\t\v", c); i >= 0 {
		return `\` + string("abfnrtv"[i])
	}
	return fmt.Sprintf(`\%03o`, c)
}

func printfFixInf(s string, conv byte) string {
	s = strings.ReplaceAll(s, "+Inf", "inf")
	s = strings.ReplaceAll(s, "-Inf", "-inf")
	if conv >= 'A' && conv <= 'Z' {
		s = strings.ReplaceAll(s, "inf", "INF")
		s = strings.ReplaceAll(s, "-INF", "-INF")
	}
	return s
}
