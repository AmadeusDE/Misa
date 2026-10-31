package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	expandSpec   = "i(initial)t(tabs):"
	unexpandSpec = "a(all)f(first-only)t(tabs):"
)

var (
	expandSpecParsed   *Spec
	unexpandSpecParsed *Spec
)

func init() {
	var err error
	expandSpecParsed, err = ParseSpec(expandSpec)
	if err != nil {
		panic("builtins: bad expand spec: " + err.Error())
	}
	unexpandSpecParsed, err = ParseSpec(unexpandSpec)
	if err != nil {
		panic("builtins: bad unexpand spec: " + err.Error())
	}
	Register(Command{Name: "expand", Run: runExpand})
	Register(Command{Name: "unexpand", Run: runUnexpand})
}

type repeatMode int

const (
	repeatNone repeatMode = iota
	repeatSlash
	repeatPlus
)

type tabStopResult struct {
	stops    []int
	repeat   repeatMode
	interval int
}

func tabEntryParseSpecifiers(e string) (mode repeatMode, numStr string) {
	prefix := ""
	rest := e
	for len(rest) > 0 && (rest[0] == '+' || rest[0] == '/') {
		prefix += string(rest[0])
		rest = rest[1:]
	}
	suffix := ""
	for len(rest) > 0 && (rest[len(rest)-1] == '+' || rest[len(rest)-1] == '/') {
		suffix = string(rest[len(rest)-1]) + suffix
		rest = rest[:len(rest)-1]
	}
	allSpec := prefix + suffix
	if len(allSpec) == 0 {
		return repeatNone, rest
	}
	last := allSpec[len(allSpec)-1]
	if last == '/' {
		return repeatSlash, rest
	}
	return repeatPlus, rest
}

func parseTabStops(s string) (*tabStopResult, error) {
	s = strings.NewReplacer("\t", ",", " ", ",").Replace(s)
	parts := strings.Split(s, ",")

	var entries []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			entries = append(entries, p)
		}
	}

	if len(entries) == 0 {
		return &tabStopResult{stops: []int{8}}, nil
	}

	result := &tabStopResult{}
	for i, e := range entries {
		isLast := i == len(entries)-1

		if isLast {
			mode, numStr := tabEntryParseSpecifiers(e)
			if numStr == "" {
				if mode != repeatNone {
					result.repeat = mode
					result.interval = 8
				}
				continue
			}
			var invalidChars string
			for _, ch := range numStr {
				if ch < '0' || ch > '9' {
					invalidChars += string(ch)
				}
			}
			if invalidChars != "" {
				return nil, fmt.Errorf("tab size contains invalid character(s): '%s'", numStr[strings.IndexAny(numStr, invalidChars):])
			}
			n, err := strconv.Atoi(numStr)
			if err != nil {
				return nil, fmt.Errorf("tab size contains invalid character(s): %q", numStr)
			}
			if n == 0 {
				return nil, fmt.Errorf("tab size cannot be 0")
			}
			if mode != repeatNone {
				result.repeat = mode
				result.interval = n
			} else {
				if len(result.stops) > 0 && n <= result.stops[len(result.stops)-1] {
					return nil, fmt.Errorf("tab sizes must be ascending")
				}
				result.stops = append(result.stops, n)
			}
		} else {
			stripped := strings.TrimLeft(e, "+/")
			for _, ch := range stripped {
				if ch == '/' || ch == '+' {
					return nil, fmt.Errorf("'/' specifier only allowed with the last value")
				}
			}
			if stripped == "" {
				continue
			}
			var invalidChars string
			for _, ch := range stripped {
				if ch < '0' || ch > '9' {
					invalidChars += string(ch)
				}
			}
			if invalidChars != "" {
				return nil, fmt.Errorf("tab size contains invalid character(s): '%s'", stripped[strings.IndexAny(stripped, invalidChars):])
			}
			n, err := strconv.Atoi(stripped)
			if err != nil {
				return nil, fmt.Errorf("tab size contains invalid character(s): %q", stripped)
			}
			if n == 0 {
				return nil, fmt.Errorf("tab size cannot be 0")
			}
			if len(result.stops) > 0 && n <= result.stops[len(result.stops)-1] {
				return nil, fmt.Errorf("tab sizes must be ascending")
			}
			result.stops = append(result.stops, n)
		}
	}

	if len(result.stops) == 0 && result.repeat == repeatNone {
		return &tabStopResult{stops: []int{8}}, nil
	}

	return result, nil
}

func nextTabStop(col int, ts *tabStopResult) int {
	for _, s := range ts.stops {
		if s > col {
			return s
		}
	}

	if ts.repeat != repeatNone && ts.interval > 0 {
		var base int
		if ts.repeat == repeatPlus && len(ts.stops) > 0 {
			base = ts.stops[len(ts.stops)-1]
		}
		if ts.repeat == repeatSlash {
			next := ((col / ts.interval) + 1) * ts.interval
			return next
		}
		if base <= col {
			diff := col - base
			next := base + ((diff/ts.interval)+1)*ts.interval
			return next
		}
		return base
	}

	if len(ts.stops) == 1 {
		w := ts.stops[0]
		return col + w - (col % w)
	}
	return col + 1
}

func expandPreprocess(args []string) (processed []string, tabsVal string, isExplicitTab bool) {
	var out []string
	var tabValues []string
	explicit := false
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			out = append(out, args[i:]...)
			break
		}
		if len(a) >= 2 && a[0] == '-' && a[1] >= '0' && a[1] <= '9' {
			j := 1
			for j < len(a) && ((a[j] >= '0' && a[j] <= '9') || a[j] == ',') {
				j++
			}
			tabValues = append(tabValues, a[1:j])
			if j < len(a) {
				out = append(out, "-"+a[j:])
			}
			i++
			continue
		}
		if a == "-t" || a == "--tabs" {
			explicit = true
			i++
			if i < len(args) {
				tabValues = append(tabValues, args[i])
			}
			i++
			continue
		}
		if strings.HasPrefix(a, "--tabs=") {
			explicit = true
			tabValues = append(tabValues, a[len("--tabs="):])
			i++
			continue
		}
		if strings.HasPrefix(a, "-t") && len(a) > 2 {
			explicit = true
			tabValues = append(tabValues, a[2:])
			i++
			continue
		}
		out = append(out, a)
		i++
	}
	combined := ""
	if len(tabValues) > 0 {
		combined = strings.Join(tabValues, ",")
	}
	return out, combined, explicit
}

func runExpand(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	processed, tabsVal, _ := expandPreprocess(ctx.Args)
	flags, err := expandSpecParsed.ParseArgs(processed)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "expand:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, expandSpecParsed) {
		return 0
	}

	tabSpec := tabsVal
	if v, ok := flags.String("t"); ok && tabSpec == "" {
		tabSpec = v
	}
	ts := &tabStopResult{stops: []int{8}}
	if tabSpec != "" {
		ts, err = parseTabStops(tabSpec)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "expand:", err)
			return 1
		}
	}
	initialOnly := flags.Bool("i")

	return expandProcessLines(ctx, "expand", flags.Operands, func(line string) string {
		var out strings.Builder
		col := 0
		initial := true
		for _, c := range line {
			switch c {
			case '\t':
				if !initialOnly || initial {
					target := nextTabStop(col, ts)
					out.WriteString(strings.Repeat(" ", target-col))
					col = target
				} else {
					out.WriteRune(c)
					col++
				}
			case '\b':
				out.WriteRune(c)
				if col > 0 {
					col--
				}
			default:
				if c != ' ' {
					initial = false
				}
				out.WriteRune(c)
				col += runeWidth(c)
			}
		}
		return out.String()
	})
}

func runeWidth(c rune) int {
	if c >= 0x1100 && (c <= 0x115F ||
		c == 0x2329 || c == 0x232A ||
		(c >= 0x2E80 && c <= 0xA4CF && c != 0x303F) ||
		(c >= 0xAC00 && c <= 0xD7A3) ||
		(c >= 0xF900 && c <= 0xFAFF) ||
		(c >= 0xFE10 && c <= 0xFE19) ||
		(c >= 0xFE30 && c <= 0xFE6F) ||
		(c >= 0xFF01 && c <= 0xFF60) ||
		(c >= 0xFFE0 && c <= 0xFFE6) ||
		(c >= 0x1F300 && c <= 0x1F64F) ||
		(c >= 0x1F680 && c <= 0x1F6FF) ||
		(c >= 0x20000 && c <= 0x3FFFD)) {
		return 2
	}
	return 1
}

func runUnexpand(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	cleanArgs, tabsStr, hasExplicitTab := expandPreprocess(ctx.Args)
	flags, err := unexpandSpecParsed.ParseArgs(cleanArgs)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "unexpand:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, unexpandSpecParsed) {
		return 0
	}

	var ts *tabStopResult
	if tabsStr != "" {
		parsedTs, err := parseTabStops(tabsStr)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "unexpand:", err)
			return 1
		}
		ts = parsedTs
	} else {
		ts = &tabStopResult{stops: []int{8}}
	}

	convertAll := flags.Bool("a") || hasExplicitTab
	firstOnly := flags.Bool("f")
	if firstOnly {
		convertAll = false
	}

	return expandProcessLines(ctx, "unexpand", flags.Operands, func(line string) string {
		var out strings.Builder
		col := 0
		spaceStartCol := 0
		inWhitespace := false
		hasTab := false
		leadingDone := false

		flush := func() {
			if !inWhitespace {
				return
			}
			canConvert := !leadingDone || convertAll
			if canConvert || hasTab {
				for spaceStartCol < col {
					next := nextTabStop(spaceStartCol, ts)
					if next <= col && (next-spaceStartCol) > 1 {
						out.WriteByte('\t')
						spaceStartCol = next
					} else if next <= col {
						out.WriteByte(' ')
						spaceStartCol = next
					} else {
						for spaceStartCol < col {
							out.WriteByte(' ')
							spaceStartCol++
						}
					}
				}
			} else {
				for spaceStartCol < col {
					out.WriteByte(' ')
					spaceStartCol++
				}
			}
			inWhitespace = false
			hasTab = false
		}

		for _, c := range line {
			if c == ' ' || c == '\t' {
				if !inWhitespace {
					spaceStartCol = col
					inWhitespace = true
					hasTab = false
				}
				if c == '\t' {
					hasTab = true
					col = nextTabStop(col, ts)
				} else {
					col++
				}
			} else {
				flush()
				out.WriteRune(c)
				col += runeWidth(c)
				leadingDone = true
			}
		}
		flush()
		return out.String()
	})
}

func expandProcessLines(ctx *Context, cmdName string, files []string, transform func(string) string) int {
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintf(ctx.Stderr, "%s: missing operand\n", cmdName)
			return 1
		}
		files = []string{"-"}
	}

	exitCode := 0
	for _, f := range files {
		var r io.Reader
		if f == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintf(ctx.Stderr, "%s: stdin not available\n", cmdName)
				exitCode = 1
				continue
			}
			r = ctx.Stdin
		} else {
			path := resolvePath(ctx, f)
			st, statErr := os.Stat(path)
			if statErr != nil {
				fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", cmdName, f, fmtOsErr(statErr))
				exitCode = 1
				continue
			}
			if st.IsDir() {
				fmt.Fprintf(ctx.Stderr, "%s: %s: Is a directory\n", cmdName, f)
				exitCode = 1
				continue
			}
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", cmdName, f, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer fh.Close()
			r = fh
		}

		br := bufio.NewReader(ContextReader(ctx.CancellationContext(), r))
		for {
			rawLine, readErr := br.ReadBytes('\n')
			if len(rawLine) == 0 && readErr != nil {
				if readErr != io.EOF {
					fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", cmdName, f, fmtOsErr(readErr))
					exitCode = 1
				}
				break
			}
			hasNewline := len(rawLine) > 0 && rawLine[len(rawLine)-1] == '\n'
			content := string(rawLine)
			if hasNewline {
				content = content[:len(content)-1]
			}
			transformed := transform(content)
			if _, err := ctx.Stdout.Write([]byte(transformed)); err != nil {
				return 1
			}
			if hasNewline {
				if _, err := ctx.Stdout.Write([]byte{'\n'}); err != nil {
					return 1
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", cmdName, f, fmtOsErr(readErr))
					exitCode = 1
				}
				break
			}
		}
	}
	return exitCode
}
