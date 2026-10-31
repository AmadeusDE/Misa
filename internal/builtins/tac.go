package builtins

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

const tacSpec = "s(separator):b(before)r(regex)"

var tacSpecParsed *Spec

func init() {
	var err error
	tacSpecParsed, err = ParseSpec(tacSpec)
	if err != nil {
		panic("builtins: bad tac spec: " + err.Error())
	}
	Register(Command{Name: "tac", Run: runTac})
}

func runTac(ctx *Context) (code int) {
	originalStdin := ctx.Stdin
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	flags, err := tacSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tac:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, tacSpecParsed) {
		return 0
	}

	sep := "\n"
	if s, ok := flags.String("s"); ok {
		sep = s
	}
	before := flags.Bool("b")
	useRegex := flags.Bool("r")

	files := flags.Operands
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "tac: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	exitCode := 0
	for _, name := range files {
		var r io.Reader
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "tac: stdin not available")
				exitCode = 1
				continue
			}
			if f, ok := originalStdin.(*os.File); ok {
				if _, err := f.Stat(); err != nil {
					fmt.Fprintf(ctx.Stderr, "tac: read error: %s\n", fmtOsErr(err))
					exitCode = 1
					continue
				}
			}
			r = ctx.Stdin
		} else {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(ctx.Dir, path)
			}
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "tac: %s: %s\n", name, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer f.Close()
			r = f
		}

		data, err := io.ReadAll(ContextReader(ctx.CancellationContext(), r))
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "tac: %s: %s\n", name, fmtOsErr(err))
			exitCode = 1
			continue
		}

		if useRegex {
			tacRegex(ctx, data, sep, before)
		} else {
			tacString(ctx, data, sep, before)
		}
	}
	return exitCode
}

func tacString(ctx *Context, data []byte, sep string, before bool) {
	if len(data) == 0 {
		return
	}
	sepBytes := []byte(sep)
	if len(sepBytes) == 0 {
		sepBytes = []byte{0}
	}

	end := len(data)

	if before {
		for end > 0 {
			if ctx.CancellationContext().Err() != nil {
				return
			}
			idx := bytes.LastIndex(data[:end], sepBytes)
			if idx < 0 {
				if _, err := ctx.Stdout.Write(data[:end]); err != nil {
					return
				}
				break
			}
			if _, err := ctx.Stdout.Write(data[idx:end]); err != nil {
				return
			}
			end = idx
		}
	} else {
		idx := bytes.LastIndex(data[:end], sepBytes)
		if idx < 0 {
			if _, err := ctx.Stdout.Write(data[:end]); err != nil {
				return
			}
			return
		}

		matchEnd := idx + len(sepBytes)
		if matchEnd < end {
			if _, err := ctx.Stdout.Write(data[matchEnd:end]); err != nil {
				return
			}
			end = matchEnd
		}

		for end > 0 {
			if ctx.CancellationContext().Err() != nil {
				return
			}
			idx := bytes.LastIndex(data[:end], sepBytes)
			if idx < 0 {
				if _, err := ctx.Stdout.Write(data[:end]); err != nil {
					return
				}
				break
			}
			matchStart := idx
			prevIdx := bytes.LastIndex(data[:matchStart], sepBytes)
			if prevIdx < 0 {
				if _, err := ctx.Stdout.Write(data[:end]); err != nil {
					return
				}
				break
			}
			prevEnd := prevIdx + len(sepBytes)
			if _, err := ctx.Stdout.Write(data[prevEnd:end]); err != nil {
				return
			}
			end = prevEnd
		}
	}
}

func prepareTacRegex(pattern string) string {
	var buf []byte
	n := len(pattern)
	for i := 0; i < n; i++ {
		if pattern[i] == '\\' && i+1 < n && pattern[i+1] == '|' {
			buf = append(buf, '|')
			i++
		} else {
			buf = append(buf, pattern[i])
		}
	}
	pattern = string(buf)

	var res []byte
	pn := len(pattern)
	for i := 0; i < pn; i++ {
		ch := pattern[i]
		if ch == '^' && i > 0 {
			prev := pattern[i-1]
			if prev != '|' && prev != '(' && prev != '\n' && prev != '[' && prev != '\\' {
				res = append(res, '\\', '^')
				continue
			}
		}
		if ch == '$' && i < pn-1 {
			next := pattern[i+1]
			prev := byte(0)
			if i > 0 {
				prev = pattern[i-1]
			}
			if next != '|' && next != ')' && next != '\n' && prev != '\\' {
				res = append(res, '\\', '$')
				continue
			}
		}
		res = append(res, ch)
	}

	return "(?m)" + string(res)
}

func hasStartAnchor(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '^' {
			if i == 0 {
				return true
			}
			prev := pattern[i-1]
			if prev == '|' || prev == '(' || prev == 'm' || prev == '\n' {
				return true
			}
		}
	}
	return false
}

func findRightmostMatch(re *regexp.Regexp, data []byte, end int, checkStartAnchor bool) (int, int, bool) {
	for pos := end - 1; pos >= 0; pos-- {
		if checkStartAnchor && pos > 0 && data[pos-1] != '\n' {
			continue
		}
		loc := re.FindIndex(data[pos:end])
		if loc != nil && loc[0] == 0 {
			return pos, pos + loc[1], true
		}
	}
	return 0, 0, false
}

func tacRegex(ctx *Context, data []byte, pattern string, before bool) {
	if len(data) == 0 {
		return
	}
	if pattern == "" {
		pattern = "\x00"
	}
	rawPattern := pattern
	pattern = prepareTacRegex(pattern)
	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "tac: invalid regex: %v\n", err)
		return
	}

	checkStartAnchor := hasStartAnchor(pattern) || hasStartAnchor(rawPattern)

	end := len(data)

	if before {
		for end > 0 {
			if ctx.CancellationContext().Err() != nil {
				return
			}
			mStart, _, found := findRightmostMatch(re, data, end, checkStartAnchor)
			if !found {
				if _, err := ctx.Stdout.Write(data[:end]); err != nil {
					return
				}
				break
			}
			if _, err := ctx.Stdout.Write(data[mStart:end]); err != nil {
				return
			}
			end = mStart
		}
	} else {
		_, mEnd, found := findRightmostMatch(re, data, end, checkStartAnchor)
		if !found {
			if _, err := ctx.Stdout.Write(data[:end]); err != nil {
				return
			}
			return
		}
		if mEnd < end {
			if _, err := ctx.Stdout.Write(data[mEnd:end]); err != nil {
				return
			}
			end = mEnd
		}

		for end > 0 {
			if ctx.CancellationContext().Err() != nil {
				return
			}
			mStart, _, found := findRightmostMatch(re, data, end, checkStartAnchor)
			if !found {
				if _, err := ctx.Stdout.Write(data[:end]); err != nil {
					return
				}
				break
			}
			_, prevEnd, prevFound := findRightmostMatch(re, data, mStart, checkStartAnchor)
			if !prevFound {
				if _, err := ctx.Stdout.Write(data[:end]); err != nil {
					return
				}
				break
			}
			if _, err := ctx.Stdout.Write(data[prevEnd:end]); err != nil {
				return
			}
			end = prevEnd
		}
	}
}
