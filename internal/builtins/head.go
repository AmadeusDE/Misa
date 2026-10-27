package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const headSpec = "n(lines)#c(bytes)#q(quiet)v(verbose)z(zero-terminated)"

var headSpecParsed *Spec

func init() {
	var err error
	headSpecParsed, err = ParseSpec(headSpec)
	if err != nil {
		panic("builtins: bad head spec: " + err.Error())
	}
	Register(Command{Name: "head", Run: runHead})
}

func headCleanNum(s string) string {
	trimmed := strings.TrimLeft(s, " \t")
	cleaned := strings.TrimSuffix(trimmed, "R")
	sign := ""
	rest := cleaned
	if strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, "+") {
		sign = string(rest[0])
		rest = rest[1:]
	}
	numPart, _ := parseSizeSuffix(rest)
	if _, err := strconv.ParseInt(numPart, 10, 64); err != nil {
		if allDigits(numPart) && len(numPart) > 0 {
			return sign + "9223372036854775807"
		}
		return s
	}
	return sign + rest
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

func headParseCount(s string) (int64, bool, bool) {
	s = strings.TrimLeft(s, " \t")
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	s = strings.TrimSuffix(s, "R")
	numStr, mult := parseSizeSuffix(s)
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, false, false
	}
	if n > 0 && mult > 1 {
		const maxVal = int64(1<<63 - 1)
		if n > maxVal/mult {
			n = maxVal / mult
		}
	}
	val := n * mult
	if neg {
		val = -val
	}
	return val, neg && n == 0, true
}

func headQuoteString(s string) string {
	needsQuote := false
	for _, c := range []byte(s) {
		if c < 32 || c >= 127 || c == ' ' || c == '\'' || c == '\\' || c == '"' || c == '$' || c == '`' || c == '*' || c == '?' || c == '[' || c == '#' || c == '~' || c == '<' || c == '>' || c == '|' || c == '&' || c == ';' || c == '(' || c == ')' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return s
	}

	var sb strings.Builder
	sb.WriteString("''")
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 32 || b >= 127 {
			sb.WriteString("$'")
			switch b {
			case '\n':
				sb.WriteString("\\n")
			case '\r':
				sb.WriteString("\\r")
			case '\t':
				sb.WriteString("\\t")
			case '\a':
				sb.WriteString("\\a")
			case '\b':
				sb.WriteString("\\b")
			case '\f':
				sb.WriteString("\\f")
			case '\v':
				sb.WriteString("\\v")
			case '\\':
				sb.WriteString("\\\\")
			case '\'':
				sb.WriteString("\\'")
			default:
				fmt.Fprintf(&sb, "\\%03o", b)
			}
			sb.WriteString("'")
		} else if b == '\'' {
			sb.WriteString("'\\''")
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

func runHead(ctx *Context) int {
	var args []string
	for _, a := range ctx.Args {
		if a == "---presume-input-pipe" {
			continue
		}
		args = append(args, a)
	}

	var rawNVal, rawCVal string
	for i, a := range args {
		if i > 0 && (args[i-1] == "-n" || args[i-1] == "-c") {
			continue
		}
		if len(a) >= 2 && a[0] == '-' && a[1] >= '0' && a[1] <= '9' {
			j := 1
			for j < len(a) && a[j] >= '0' && a[j] <= '9' {
				j++
			}
			num := a[1:j]
			trail := a[j:]
			var expanded []string
			if len(trail) > 0 && (trail[0] == 'c' || trail[0] == 'n') {
				expanded = append(expanded, "-"+string(trail[0]), num)
				trail = trail[1:]
			} else {
				expanded = append(expanded, "-n", num)
			}
			if trail != "" {
				expanded = append(expanded, "-"+trail)
			}
			args = append(args[:i], append(expanded, args[i+1:]...)...)
			break
		}
	}

	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			rawNVal = args[i+1]
			i++
		} else if args[i] == "-c" && i+1 < len(args) {
			rawCVal = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--lines=") {
			rawNVal = args[i][len("--lines="):]
		} else if strings.HasPrefix(args[i], "--bytes=") {
			rawCVal = args[i][len("--bytes="):]
		}
	}

	var preArgs []string
	skipNext := false
	for i, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if (a == "-n" || a == "-c") && i+1 < len(args) {
			val := args[i+1]
			cleaned := headCleanNum(val)
			if cleaned != val {
				preArgs = append(preArgs, a, cleaned)
			} else {
				preArgs = append(preArgs, a, val)
			}
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "--lines=") || strings.HasPrefix(a, "--bytes=") {
			eq := strings.IndexByte(a, '=')
			prefix := a[:eq+1]
			val := a[eq+1:]
			cleaned := headCleanNum(val)
			preArgs = append(preArgs, prefix+cleaned)
			continue
		}
		if len(a) > 2 && a[0] == '-' && (a[1] == 'n' || a[1] == 'c') {
			flag := string(a[:2])
			val := a[2:]
			if strings.HasPrefix(val, "=") {
				val = val[1:]
			}
			cleaned := headCleanNum(val)
			preArgs = append(preArgs, flag, cleaned)
			continue
		}
		preArgs = append(preArgs, a)
	}

	flags, err := headSpecParsed.ParseArgs(preArgs)
	if err != nil {
		pe, ok := err.(*ParseError)
		if ok {
			msg := pe.Msg
			if strings.Contains(msg, "invalid value") {
				if strings.Contains(msg, "-c/--bytes") {
					start := strings.Index(msg, "'") + 1
					end := strings.Index(msg[start:], "'") + start
					badVal := msg[start:end]
					badVal = strings.TrimLeft(badVal, "-")
					fmt.Fprintf(ctx.Stderr, "head: invalid number of bytes: '%s'\n", badVal)
				} else if strings.Contains(msg, "-n/--lines") {
					start := strings.Index(msg, "'") + 1
					end := strings.Index(msg[start:], "'") + start
					badVal := msg[start:end]
					badVal = strings.TrimLeft(badVal, "-")
					fmt.Fprintf(ctx.Stderr, "head: invalid number of lines: '%s'\n", badVal)
				} else {
					fmt.Fprintln(ctx.Stderr, "head:", msg)
				}
			} else {
				fmt.Fprintln(ctx.Stderr, "head:", msg)
			}
		} else {
			fmt.Fprintln(ctx.Stderr, "head:", err)
		}
		return 1
	}
	if HandleHelpVersion(ctx, flags, headSpecParsed) {
		return 0
	}

	nLines := int64(10)
	negZeroN := false
	if _, ok := flags.Num("n"); ok {
		if rawNVal != "" {
			val, isNegZero, valid := headParseCount(rawNVal)
			if valid {
				nLines = val
				negZeroN = isNegZero
			}
		} else {
			v, _ := flags.Num("n")
			nLines = v
		}
	}

	nBytes := int64(0)
	negZeroC := false
	hasBytes := false
	if _, ok := flags.Num("c"); ok {
		hasBytes = true
		if rawCVal != "" {
			val, isNegZero, valid := headParseCount(rawCVal)
			if valid {
				nBytes = val
				negZeroC = isNegZero
			}
		} else {
			nBytes, _ = flags.Num("c")
		}
	}

	useBytes := hasBytes
	if hasBytes {
		for _, a := range ctx.Args {
			if a == "-n" || (len(a) >= 2 && a[0] == '-' && a[len(a)-1] == 'n') {
				useBytes = false
			}
			if a == "-c" || (len(a) >= 2 && a[0] == '-' && a[len(a)-1] == 'c') {
				useBytes = true
			}
		}
	}

	files := flags.Operands
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "head: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	multi := len(files) > 1
	verbose := flags.Bool("v")
	quiet := flags.Bool("q")
	exitCode := 0

	// Check total number of lines/bytes to output
	count := nLines
	if useBytes {
		count = nBytes
	}
	isZeroCount := count == 0 && !((useBytes && negZeroC) || (!useBytes && negZeroN))

	for i, name := range files {
		showHeader := (multi && !quiet) || verbose
		var r io.Reader
		var openErr error
		isDir := false
		if name == "-" {
			if ctx.Stdin == nil {
				openErr = fmt.Errorf("stdin not available")
			} else {
				r = ctx.Stdin
			}
		} else {
			path := resolvePath(ctx, name)
			fi, statErr := os.Stat(path)
			if statErr != nil {
				openErr = statErr
			} else if fi.IsDir() {
				isDir = true
				if !isZeroCount {
					openErr = fmt.Errorf("Is a directory")
				}
			} else if !isZeroCount {
				f, err := os.Open(path)
				if err != nil {
					openErr = err
				} else {
					defer f.Close()
					r = f
				}
			}
		}

		if openErr != nil {
			if name == "-" {
				fmt.Fprintf(ctx.Stderr, "head: %s\n", openErr)
			} else if openErr.Error() == "Is a directory" {
				fmt.Fprintf(ctx.Stderr, "head: error reading '%s': Is a directory\n", name)
			} else {
				fmt.Fprintf(ctx.Stderr, "head: cannot open '%s' for reading: %s\n", name, fmtOsErr(openErr))
			}
			exitCode = 1
			continue
		}

		if showHeader {
			if i > 0 {
				fmt.Fprintln(ctx.Stdout)
			}
			hdr := name
			if hdr == "-" {
				hdr = "standard input"
			} else {
				hdr = headQuoteString(hdr)
			}
			fmt.Fprintf(ctx.Stdout, "==> %s <==\n", hdr)
		}

		if isZeroCount || isDir {
			continue
		}

		zeroTerm := flags.Bool("z")
		lineSep := byte('\n')
		if zeroTerm {
			lineSep = 0
		}

		if useBytes {
			if negZeroC {
				io.Copy(ctx.Stdout, r)
			} else if nBytes < 0 {
				data, _ := io.ReadAll(r)
				end := int64(len(data)) + nBytes
				if end > 0 {
					ctx.Stdout.Write(data[:end])
				}
			} else {
				io.CopyN(ctx.Stdout, r, nBytes)
			}
		} else if negZeroN {
			io.Copy(ctx.Stdout, r)
		} else if nLines < 0 {
			br := bufio.NewReader(r)
			var lines [][]byte
			for {
				line, err := br.ReadBytes(lineSep)
				if len(line) > 0 {
					lines = append(lines, append([]byte(nil), line...))
				}
				if err != nil {
					break
				}
			}
			end := len(lines) + int(nLines)
			for j := 0; j < end; j++ {
				ctx.Stdout.Write(lines[j])
			}
		} else {
			br := bufio.NewReader(r)
			for count := int64(0); count < nLines; count++ {
				line, err := br.ReadBytes(lineSep)
				if len(line) > 0 {
					ctx.Stdout.Write(line)
				}
				if err != nil {
					break
				}
			}
		}
	}
	return exitCode
}
