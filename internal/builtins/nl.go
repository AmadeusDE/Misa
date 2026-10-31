package builtins

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const nlSpec = "^b(body-numbering):d(section-delimiter):f(footer-numbering):h(header-numbering):i(line-increment)#l(join-blank-lines)#n(number-format):p(no-renumber)s(number-separator):v(starting-line-number)#w(number-width)#"

var nlSpecParsed *Spec

func init() {
	var err error
	nlSpecParsed, err = ParseSpec(nlSpec)
	if err != nil {
		panic("builtins: bad nl spec: " + err.Error())
	}
	Register(Command{Name: "nl", Run: runNl})
}

func nlCompileMode(mode string) (*regexp.Regexp, error) {
	if strings.HasPrefix(mode, "p") {
		return regexp.Compile(mode[1:])
	}
	return nil, nil
}

func nlShouldNumber(line, mode string, re *regexp.Regexp) bool {
	switch mode {
	case "a":
		return true
	case "t":
		return line != ""
	case "n":
		return false
	default:
		if re != nil {
			return re.MatchString(line)
		}
		return false
	}
}

func runNl(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	flags, err := nlSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "nl:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, nlSpecParsed) {
		return 0
	}

	bodyMode := "t"
	if b, ok := flags.String("b"); ok {
		bodyMode = b
	}
	headerMode := "n"
	if h, ok := flags.String("h"); ok {
		headerMode = h
	}
	footerMode := "n"
	if f, ok := flags.String("f"); ok {
		footerMode = f
	}

	delimStr := `\:`
	if d, ok := flags.String("d"); ok {
		if len(d) == 0 {
			delimStr = ""
		} else if len([]rune(d)) == 1 {
			delimStr = d + ":"
		} else {
			delimStr = d
		}
	}

	sep := "\t"
	if s, ok := flags.String("s"); ok {
		sep = s
	}

	width := int64(6)
	if w, ok := flags.Num("w"); ok {
		width = w
	}

	numFmt := "rn"
	if n, ok := flags.String("n"); ok {
		if n != "ln" && n != "rn" && n != "rz" {
			fmt.Fprintf(ctx.Stderr, "nl: invalid value '%s' for number format\n", n)
			return 1
		}
		numFmt = n
	}

	startLine := int64(1)
	if v, ok := flags.Num("v"); ok {
		startLine = v
	}

	increment := int64(1)
	if i, ok := flags.Num("i"); ok {
		increment = i
	}

	joinBlank := int64(1)
	if l, ok := flags.Num("l"); ok {
		joinBlank = l
	}

	if w, wok := flags.Num("w"); wok && (w <= 0 || w > 2147483647) {
		fmt.Fprintf(ctx.Stderr, "nl: invalid line number field width: '%d': is not in 1..=2147483647\n", w)
		return 1
	}

	noRenumber := flags.Bool("p")

	bodyRe, err := nlCompileMode(bodyMode)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "nl: invalid regex in body mode: %v\n", err)
		return 1
	}
	headerRe, err := nlCompileMode(headerMode)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "nl: invalid regex in header mode: %v\n", err)
		return 1
	}
	footerRe, err := nlCompileMode(footerMode)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "nl: invalid regex in footer mode: %v\n", err)
		return 1
	}

	headerDelim := ""
	bodyDelim := ""
	footerDelim := ""
	if delimStr != "" {
		headerDelim = delimStr + delimStr + delimStr
		bodyDelim = delimStr + delimStr
		footerDelim = delimStr
	}

	files := flags.Operands
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "nl: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	lineNum := startLine
	blankCount := int64(0)
	exitCode := 0
	currentMode := bodyMode
	currentRe := bodyRe
	nextLineNumWouldOverflow := false

	for _, name := range files {
		var r io.Reader
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "nl: stdin not available")
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
				fmt.Fprintf(ctx.Stderr, "nl: %s: %s\n", name, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer f.Close()
			r = f
		}

		sc := bufio.NewScanner(ContextReader(ctx.CancellationContext(), r))
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()

			// Handle delimiters first (before overflow check)
			if delimStr != "" {
				if line == headerDelim {
					currentMode = headerMode
					currentRe = headerRe
					if !noRenumber {
						lineNum = startLine
						nextLineNumWouldOverflow = false // Reset overflow flag when counter resets
					}
					blankCount = 0
					if _, err := fmt.Fprintln(ctx.Stdout); err != nil {
						return 1
					}
					continue
				}
				if line == bodyDelim {
					currentMode = bodyMode
					currentRe = bodyRe
					if !noRenumber {
						lineNum = startLine
						nextLineNumWouldOverflow = false // Reset overflow flag when counter resets
					}
					blankCount = 0
					if _, err := fmt.Fprintln(ctx.Stdout); err != nil {
						return 1
					}
					continue
				}
				if line == footerDelim {
					currentMode = footerMode
					currentRe = footerRe
					if !noRenumber {
						lineNum = startLine
						nextLineNumWouldOverflow = false // Reset overflow flag when counter resets
					}
					blankCount = 0
					if _, err := fmt.Fprintln(ctx.Stdout); err != nil {
						return 1
					}
					continue
				}
			}

			// Check if overflow was detected on previous numbered line
			// and now we have another line to process
			if nextLineNumWouldOverflow {
				// This line needs to be numbered but would overflow
				if nlShouldNumber(line, currentMode, currentRe) {
					fmt.Fprintf(ctx.Stderr, "nl: %s: Numerical result out of range\n", name)
					exitCode = 1
					break
				}
				nextLineNumWouldOverflow = false
			}

			shouldNumber := nlShouldNumber(line, currentMode, currentRe)

			if line == "" {
				blankCount++
			} else {
				blankCount = 0
			}

			numberThisLine := shouldNumber
			if shouldNumber && line == "" && joinBlank > 1 {
				numberThisLine = blankCount%joinBlank == 0
			}
			if numberThisLine {
				var numStr string
				switch numFmt {
				case "ln":
					numStr = fmt.Sprintf("%-*d", int(width), lineNum)
				case "rz":
					numStr = fmt.Sprintf("%0*d", int(width), lineNum)
				default:
					numStr = fmt.Sprintf("%*d", int(width), lineNum)
				}
				if _, err := fmt.Fprintf(ctx.Stdout, "%s%s%s\n", numStr, sep, line); err != nil {
					return 1
				}

				// Check if next increment would overflow
				var wouldOverflow bool
				if increment > 0 {
					wouldOverflow = lineNum > math.MaxInt64-increment
				} else if increment < 0 {
					if increment == math.MinInt64 {
						wouldOverflow = lineNum < 0
					} else {
						wouldOverflow = (lineNum - math.MinInt64) < -increment
					}
				}
				nextLineNumWouldOverflow = wouldOverflow
				lineNum += increment
			} else {
				blank := int(width) + len(sep)
				if _, err := fmt.Fprintf(ctx.Stdout, "%*s%s\n", blank, "", line); err != nil {
					return 1
				}
			}
		}
		if err := sc.Err(); err != nil {
			fmt.Fprintf(ctx.Stderr, "nl: %s: %s\n", name, fmtOsErr(err))
			exitCode = 1
		}
	}
	return exitCode
}
