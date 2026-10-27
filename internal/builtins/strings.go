package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const stringsSpec = "f(print-file-name)n(bytes)#t(radix):o"

var stringsSpecParsed *Spec

func init() {
	var err error
	stringsSpecParsed, err = ParseSpec(stringsSpec)
	if err != nil {
		panic("builtins: bad strings spec: " + err.Error())
	}
	Register(Command{Name: "strings", Run: runStrings})
}

func runStrings(ctx *Context) int {
	flags, err := stringsSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "strings:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, stringsSpecParsed) {
		return 0
	}

	minLen := int64(4)
	if v, ok := flags.Num("n"); ok {
		if v < 1 {
			fmt.Fprintln(ctx.Stderr, "strings: invalid minimum length")
			return 1
		}
		minLen = v
	}

	showFile := flags.Bool("f")

	offsetRadix := ""
	if flags.Bool("o") {
		offsetRadix = "o"
	}
	if t, ok := flags.String("t"); ok {
		offsetRadix = t
	}

	files := flags.Operands
	if len(files) == 0 {
		if ctx.Stdin == nil {
			fmt.Fprintln(ctx.Stderr, "strings: missing operand")
			return 1
		}
		files = []string{"-"}
	}

	exitCode := 0
	for _, f := range files {
		var r io.Reader
		name := f
		if f == "-" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, "strings: stdin not available")
				exitCode = 1
				continue
			}
			r = ctx.Stdin
		} else {
			path := resolvePath(ctx, f)
			fh, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "strings: %v\n", err)
				exitCode = 1
				continue
			}
			defer fh.Close()
			r = fh
		}
		prefix := ""
		if showFile {
			prefix = name + ": "
		}
		stringsExtract(ctx, r, int(minLen), prefix, offsetRadix)
	}
	return exitCode
}

func stringsExtract(ctx *Context, r io.Reader, minLen int, prefix string, offsetRadix string) {
	br := bufio.NewReader(r)
	var cur []byte
	offset := int64(0)
	startOffset := int64(0)

	flush := func() {
		if len(cur) >= minLen {
			if offsetRadix != "" {
				var fmtStr string
				switch offsetRadix {
				case "o":
					fmtStr = "%7o "
				case "x":
					fmtStr = "%7x "
				default:
					fmtStr = "%7d "
				}
				fmt.Fprintf(ctx.Stdout, "%s"+fmtStr+"%s\n", prefix, startOffset, cur)
			} else {
				fmt.Fprintf(ctx.Stdout, "%s%s\n", prefix, cur)
			}
		}
		cur = cur[:0]
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			flush()
			return
		}
		if b >= 32 && b < 127 || b == '\t' {
			if len(cur) == 0 {
				startOffset = offset
			}
			cur = append(cur, b)
		} else {
			flush()
		}
		offset++
	}
}
