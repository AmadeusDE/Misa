package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const wcSpec = "l(lines)w(words)c(bytes)m(chars)L(max-line-length)(debug)(files0-from):"

var wcSpecParsed *Spec

func init() {
	var err error
	wcSpecParsed, err = ParseSpec(wcSpec)
	if err != nil {
		panic("builtins: bad wc spec: " + err.Error())
	}
	Register(Command{Name: "wc", Run: runWc})
}

type wcCounts struct {
	lines   int
	words   int
	bytes   int
	chars   int
	maxLine int
}

func isWhitespace(ru rune, posixlyCorrect bool) bool {
	if ru == ' ' || ru == '\t' || ru == '\n' || ru == '\r' || ru == '\v' || ru == '\f' {
		return true
	}
	if !posixlyCorrect {
		return unicode.IsSpace(ru)
	}
	return false
}

func wcCount(r io.Reader, posixlyCorrect bool) wcCounts {
	var c wcCounts
	br := bufio.NewReader(r)
	inWord := false
	lineLen := 0
	for {
		ru, size, err := br.ReadRune()
		if err != nil {
			break
		}
		c.bytes += size
		isValid := !(ru == utf8.RuneError && size == 1)
		if isValid {
			c.chars++
		}
		if ru == '\n' {
			c.lines++
			if lineLen > c.maxLine {
				c.maxLine = lineLen
			}
			lineLen = 0
			inWord = false
		} else if ru == '\t' {
			lineLen = (lineLen/8 + 1) * 8
			inWord = false
		} else if ru == '\r' {
			lineLen = 0
			inWord = false
		} else if isWhitespace(ru, posixlyCorrect) {
			if isValid {
				lineLen++
			}
			inWord = false
		} else if isValid && (unicode.Is(unicode.Mn, ru) || unicode.Is(unicode.Me, ru) || unicode.Is(unicode.Mc, ru)) {
			// Combining mark has 0 display width and doesn't change word state
		} else {
			if isValid && ru >= 32 {
				lineLen++
			}
			if !inWord {
				c.words++
				inWord = true
			}
		}
	}
	if lineLen > c.maxLine {
		c.maxLine = lineLen
	}
	return c
}

func wcValues(c wcCounts, showLines, showWords, showBytes, showChars, showMaxLine bool) []int {
	var vals []int
	if showLines {
		vals = append(vals, c.lines)
	}
	if showWords {
		vals = append(vals, c.words)
	}
	if showChars {
		vals = append(vals, c.chars)
	}
	if showBytes {
		vals = append(vals, c.bytes)
	}
	if showMaxLine {
		vals = append(vals, c.maxLine)
	}
	return vals
}

func numDigits(n int) int {
	if n == 0 {
		return 1
	}
	d := 0
	if n < 0 {
		d = 1
		n = -n
	}
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

func runWc(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	posixlyCorrect := false
	for _, e := range ctx.Env {
		if strings.HasPrefix(e, "POSIXLY_CORRECT=") {
			posixlyCorrect = true
			break
		}
	}

	totalMode := "auto"
	files0From := ""
	hasFiles0From := false
	var filteredArgs []string

	for i := 0; i < len(ctx.Args); i++ {
		a := ctx.Args[i]
		if strings.HasPrefix(a, "--files0-from=") {
			files0From = strings.TrimPrefix(a, "--files0-from=")
			hasFiles0From = true
			continue
		} else if a == "--files0-from" {
			if i+1 < len(ctx.Args) {
				files0From = ctx.Args[i+1]
				hasFiles0From = true
				i++
				continue
			}
		}

		matched := false
		longName := "--total"
		if strings.Contains(a, "=") {
			eqIdx := strings.IndexByte(a, '=')
			prefix := a[:eqIdx]
			if strings.HasPrefix(longName, prefix) && len(prefix) >= 3 {
				totalMode = a[eqIdx+1:]
				matched = true
			}
		} else if strings.HasPrefix(longName, a) && len(a) >= 3 && strings.HasPrefix(a, "--") {
			totalMode = "always"
			matched = true
		}
		if matched {
			for _, v := range []string{"auto", "always", "never", "only"} {
				if strings.HasPrefix(v, totalMode) {
					totalMode = v
					break
				}
			}
			continue
		}
		filteredArgs = append(filteredArgs, a)
	}

	flags, err := wcSpecParsed.ParseArgs(filteredArgs)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "wc:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, wcSpecParsed) {
		return 0
	}

	showLines := flags.Bool("l")
	showWords := flags.Bool("w")
	showBytes := flags.Bool("c")
	showChars := flags.Bool("m")
	showMaxLine := flags.Bool("L")
	defaultMode := !showLines && !showWords && !showBytes && !showChars && !showMaxLine
	if defaultMode {
		showLines = true
		showWords = true
		showBytes = true
	}

	files := flags.Operands
	if hasFiles0From {
		if len(files) > 0 {
			fmt.Fprintf(ctx.Stderr, "wc: extra operand '%s'\nfile operands cannot be combined with --files0-from\n", files[0])
			return 1
		}
		var r io.Reader
		if files0From == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "wc: when reading file names from standard input, no file name of '-' allowed")
				return 1
			}
			r = ctx.Stdin
		} else {
			p := resolvePath(ctx, files0From)
			f, err := os.Open(p)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "wc: cannot open '%s' for reading: %s\n", files0From, fmtOsErr(err))
				return 1
			}
			defer f.Close()
			r = ContextReader(ctx.CancellationContext(), f)
		}
		sc := bufio.NewScanner(r)
		sc.Split(scanZeroTerminated)
		for sc.Scan() {
			t := sc.Text()
			if t == "-" && files0From == "-" {
				fmt.Fprintln(ctx.Stderr, "wc: when reading file names from standard input, no file name of '-' allowed")
				return 1
			}
			files = append(files, t)
		}
	}

	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "wc: missing operand")
			return 1
		}
		files = []string{""}
	}

	type wcResult struct {
		counts wcCounts
		name   string
	}

	var results []wcResult
	var total wcCounts
	exitCode := 0
	hasFileArgs := false
	hasStdin := false
	totalSize := int64(0)
	for _, name := range files {
		if name == "-" || name == "" {
			hasStdin = true
			continue
		}
		hasFileArgs = true
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(ctx.Dir, path)
		}
		if fi, err := os.Stat(path); err == nil {
			totalSize += fi.Size()
		}
	}

	for _, name := range files {
		var r io.Reader
		if name == "-" || name == "" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "wc: stdin not available")
				exitCode = 1
				continue
			}
			r = ctx.Stdin
		} else {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(ctx.Dir, path)
			}
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "wc: %s: %s\n", name, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer f.Close()
			if fi, serr := f.Stat(); serr == nil && fi.IsDir() {
				fmt.Fprintf(ctx.Stderr, "wc: %s: read error: Is a directory\n", name)
				exitCode = 1
				continue
			}
			r = ContextReader(ctx.CancellationContext(), f)
		}

		c := wcCount(r, posixlyCorrect)
		total.lines += c.lines
		total.words += c.words
		total.bytes += c.bytes
		total.chars += c.chars
		if c.maxLine > total.maxLine {
			total.maxLine = c.maxLine
		}
		results = append(results, wcResult{counts: c, name: name})
	}

	multi := len(results) > 1
	numCounters := 0
	if showLines {
		numCounters++
	}
	if showWords {
		numCounters++
	}
	if showBytes {
		numCounters++
	}
	if showChars {
		numCounters++
	}
	if showMaxLine {
		numCounters++
	}

	showTotal := false
	switch totalMode {
	case "always":
		showTotal = true
	case "never":
		showTotal = false
	case "only":
		showTotal = true
	default:
		showTotal = multi
	}

	width := 0
	if multi || showTotal {
		if hasFiles0From {
			if multi {
				width = numDigits(int(totalSize))
			}
		} else {
			width = numDigits(int(totalSize))
			if hasStdin || !hasFileArgs {
				if width < 7 {
					width = 7
				}
			}
		}
	} else if hasStdin && !hasFileArgs && numCounters > 1 {
		width = 7
	}

	if totalMode != "only" {
		for _, r := range results {
			wcPrint(ctx, r.counts, r.name, width, showLines, showWords, showBytes, showChars, showMaxLine)
		}
	}
	if showTotal {
		totalName := "total"
		totalWidth := width
		if totalMode == "only" {
			totalName = ""
			totalWidth = 0
		}
		wcPrint(ctx, total, totalName, totalWidth, showLines, showWords, showBytes, showChars, showMaxLine)
	}

	return exitCode
}

func wcPrint(ctx *Context, c wcCounts, name string, width int, showLines, showWords, showBytes, showChars, showMaxLine bool) {
	vals := wcValues(c, showLines, showWords, showBytes, showChars, showMaxLine)
	maxVal := 0
	for _, v := range vals {
		if v > maxVal {
			maxVal = v
		}
	}
	effectiveWidth := width
	if width > 0 && len(vals) > 1 {
		w := numDigits(maxVal)
		if w > effectiveWidth {
			effectiveWidth = w
		}
	}
	for i, v := range vals {
		if i > 0 {
			fmt.Fprint(ctx.Stdout, " ")
		}
		if effectiveWidth > 0 {
			fmt.Fprintf(ctx.Stdout, "%*d", effectiveWidth, v)
		} else {
			fmt.Fprintf(ctx.Stdout, "%d", v)
		}
	}
	if name != "" {
		fmt.Fprintf(ctx.Stdout, " %s", quoteFilename(name))
	}
	fmt.Fprintln(ctx.Stdout)
}

func quoteFilename(name string) string {
	if !strings.ContainsAny(name, "\n\r\t\a\b\f\v") {
		return name
	}
	var sb strings.Builder
	inQuote := false
	for i := 0; i < len(name); i++ {
		b := name[i]
		switch b {
		case '\n', '\r', '\t':
			esc := ""
			switch b {
			case '\n':
				esc = "\\n"
			case '\r':
				esc = "\\r"
			case '\t':
				esc = "\\t"
			}
			if inQuote {
				sb.WriteString("'$'" + esc + "'")
				inQuote = false
			} else {
				sb.WriteString("$'" + esc + "'")
			}
		default:
			if !inQuote {
				sb.WriteByte('\'')
				inQuote = true
			}
			sb.WriteByte(b)
		}
	}
	if inQuote {
		sb.WriteByte('\'')
	}
	return sb.String()
}
