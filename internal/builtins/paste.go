package builtins

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const pasteSpec = "d(delimiters):s(serial)"

var pasteSpecParsed *Spec

func init() {
	var err error
	pasteSpecParsed, err = ParseSpec(pasteSpec)
	if err != nil {
		panic("builtins: bad paste spec: " + err.Error())
	}
	Register(Command{Name: "paste", Run: runPaste})
}

func pasteExpandDelims(s string) []string {
	var delims []string
	var current strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) {
			if current.Len() > 0 {
				delims = append(delims, current.String())
				current.Reset()
			}
			i++
			switch s[i] {
			case 'n':
				delims = append(delims, "\n")
			case 't':
				delims = append(delims, "\t")
			case '\\':
				delims = append(delims, "\\")
			case '0':
				delims = append(delims, "")
			default:
				delims = append(delims, string(s[i]))
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Mc, r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				current.WriteRune(r)
				delims = append(delims, current.String())
				current.Reset()
			} else {
				delims = append(delims, string(r))
			}
		}
	}
	if current.Len() > 0 {
		delims = append(delims, current.String())
	}
	return delims
}

func runPaste(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	flags, err := pasteSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "paste:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, pasteSpecParsed) {
		return 0
	}

	delims := []string{"\t"}
	if d, ok := flags.String("d"); ok {
		delims = pasteExpandDelims(d)
		if len(delims) == 0 {
			delims = []string{""}
		}
	}

	serial := flags.Bool("s")

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}

	type fileData struct {
		lines []string
	}

	var stdinScanner *bufio.Scanner
	if ctx.Stdin != nil {
		stdinScanner = bufio.NewScanner(ctx.Stdin)
		stdinScanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	}

	stdinIndices := []int{}
	var inputs []fileData
	for i, f := range files {
		if f == "-" {
			stdinIndices = append(stdinIndices, i)
			inputs = append(inputs, fileData{})
		} else {
			path := resolvePath(ctx, f)
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "paste: %v\n", err)
				return 1
			}
			defer fh.Close()
			var lines []string
			sc := bufio.NewScanner(ContextReader(ctx.CancellationContext(), fh))
			sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for sc.Scan() {
				lines = append(lines, sc.Text())
			}
			inputs = append(inputs, fileData{lines: lines})
		}
	}

	if len(stdinIndices) == 1 && stdinScanner != nil {
		var lines []string
		for stdinScanner.Scan() {
			lines = append(lines, stdinScanner.Text())
		}
		inputs[stdinIndices[0]] = fileData{lines: lines}
		stdinIndices = nil
	}

	delimAt := func(idx int) string {
		if len(delims) == 0 {
			return ""
		}
		return delims[idx%len(delims)]
	}

	if serial {
		for i, inp := range inputs {
			if len(stdinIndices) > 0 && files[i] == "-" {
				continue
			}
			var out strings.Builder
			for j, line := range inp.lines {
				if j > 0 {
					out.WriteString(delimAt(j - 1))
				}
				out.WriteString(line)
			}
			fmt.Fprintln(ctx.Stdout, out.String())
		}
		if serial && len(stdinIndices) > 0 && stdinScanner != nil {
			var lines []string
			for stdinScanner.Scan() {
				lines = append(lines, stdinScanner.Text())
			}
			var out strings.Builder
			for j, line := range lines {
				if j > 0 {
					out.WriteString(delimAt(j - 1))
				}
				out.WriteString(line)
			}
			fmt.Fprintln(ctx.Stdout, out.String())
		}
		return 0
	}

	if len(stdinIndices) > 0 && stdinScanner != nil {
		maxFileLines := 0
		for _, inp := range inputs {
			if len(inp.lines) > maxFileLines {
				maxFileLines = len(inp.lines)
			}
		}

		for {
			anyData := false
			var out strings.Builder
			for j := range inputs {
				if j > 0 {
					out.WriteString(delimAt(j - 1))
				}
				if files[j] == "-" {
					if stdinScanner.Scan() {
						out.WriteString(stdinScanner.Text())
						anyData = true
					}
				} else {
					if len(inputs[j].lines) > 0 {
						out.WriteString(inputs[j].lines[0])
						inputs[j].lines = inputs[j].lines[1:]
						anyData = true
					}
				}
			}
			if !anyData {
				break
			}
			fmt.Fprintln(ctx.Stdout, out.String())
		}
		return 0
	}

	maxLines := 0
	for _, inp := range inputs {
		if len(inp.lines) > maxLines {
			maxLines = len(inp.lines)
		}
	}

	for i := 0; i < maxLines; i++ {
		var out strings.Builder
		for j, inp := range inputs {
			if j > 0 {
				out.WriteString(delimAt(j - 1))
			}
			if i < len(inp.lines) {
				out.WriteString(inp.lines[i])
			}
		}
		fmt.Fprintln(ctx.Stdout, out.String())
	}
	return 0
}
