package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

const foldSpec = "w(width)#s(spaces)b(bytes)c(characters)"

var foldSpecParsed *Spec

func init() {
	var err error
	foldSpecParsed, err = ParseSpec(foldSpec)
	if err != nil {
		panic("builtins: bad fold spec: " + err.Error())
	}
	Register(Command{Name: "fold", Run: runFold})
}

func runFold(ctx *Context) int {
	args := ctx.Args
	for i, a := range args {
		if len(a) >= 2 && a[0] == '-' && a[1] >= '0' && a[1] <= '9' {
			j := 1
			for j < len(a) && a[j] >= '0' && a[j] <= '9' {
				j++
			}
			expanded := []string{"-w", a[1:j]}
			if j < len(a) {
				expanded = append(expanded, "-"+a[j:])
			}
			args = append(args[:i], append(expanded, args[i+1:]...)...)
			break
		}
	}
	flags, err := foldSpecParsed.ParseArgs(args)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "invalid value '") && (strings.Contains(msg, "-w") || strings.Contains(msg, "--width")) {
			start := strings.Index(msg, "invalid value '") + len("invalid value '")
			end := strings.Index(msg[start:], "'") + start
			if end > start {
				val := msg[start:end]
				fmt.Fprintf(ctx.Stderr, "fold: invalid number of columns: '%s'\n", val)
				return 1
			}
		}
		fmt.Fprintln(ctx.Stderr, "fold:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, foldSpecParsed) {
		return 0
	}

	width := int64(80)
	if w, ok := flags.Num("w"); ok {
		if w == 0 {
			fmt.Fprintf(ctx.Stderr, "fold: invalid number of columns: '0': Result too large\n")
			return 1
		}
		width = w
	}
	breakSpaces := flags.Bool("s")
	byteMode := flags.Bool("b")
	charMode := flags.Bool("c")

	files := flags.Operands
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "fold: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	exitCode := 0
	for _, name := range files {
		var r io.Reader
		if name == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "fold: stdin not available")
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
				fmt.Fprintf(ctx.Stderr, "fold: %s: %s\n", name, fmtOsErr(err))
				exitCode = 1
				continue
			}
			defer f.Close()
			r = f
		}

		data, err := io.ReadAll(r)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "fold: %v\n", err)
			exitCode = 1
			continue
		}

		foldData(ctx, data, int(width), breakSpaces, byteMode, charMode)
	}
	return exitCode
}

func runeColWidth(r rune, col int) int {
	switch r {
	case '\t':
		return (col/8+1)*8 - col
	case '\b':
		if col > 0 {
			return -1
		}
		return 0
	case '\r':
		return -col
	default:
		return runewidth.RuneWidth(r)
	}
}

func foldData(ctx *Context, data []byte, width int, breakSpaces, byteMode, charMode bool) {
	col := 0
	lineStart := 0
	i := 0
	for i < len(data) {
		if data[i] == '\n' {
			ctx.Stdout.Write(data[lineStart : i+1])
			lineStart = i + 1
			col = 0
			i++
			continue
		}

		size := 1
		w := 1
		if !byteMode {
			r, s := utf8.DecodeRune(data[i:])
			size = s
			if charMode && r != '\t' && r != '\b' && r != '\r' {
				w = 1
			} else {
				w = runeColWidth(r, col)
			}
		}

		if col+w > width && width > 0 && i > lineStart {
			cut := i
			if breakSpaces {
				for j := i - 1; j >= lineStart; j-- {
					if data[j] == ' ' || data[j] == '\t' {
						cut = j + 1
						break
					}
				}
			}
			if cut <= lineStart {
				cut = i
			}
			ctx.Stdout.Write(data[lineStart:cut])
			ctx.Stdout.Write([]byte{'\n'})
			lineStart = cut
			col = 0
			i = cut
			continue
		}
		col += w
		if col < 0 {
			col = 0
		}
		i += size
	}

	if lineStart < len(data) {
		ctx.Stdout.Write(data[lineStart:])
	}
}
