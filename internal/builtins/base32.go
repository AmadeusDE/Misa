package builtins

import (
	"encoding/base32"
	"fmt"
	"strconv"
	"strings"
)

const base32Spec = "d(decode)w(wrap)#i(ignore-garbage)"

var base32SpecParsed *Spec

func init() {
	var err error
	base32SpecParsed, err = ParseSpec(base32Spec)
	if err != nil {
		panic("builtins: bad base32 spec: " + err.Error())
	}
	Register(Command{Name: "base32", Run: runBase32})
}

func runBase32(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	args := normBaseEncArgs(ctx.Args)
	flags, err := base32SpecParsed.ParseArgs(args)
	if err != nil {
		baseEncFormatParseError(ctx, "base32", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, base32SpecParsed) {
		return 0
	}

	decode := flags.Bool("d")
	wrap := int64(76)
	if v, ok := flags.Num("w"); ok {
		if v < 0 {
			fmt.Fprintf(ctx.Stderr, "base32: invalid wrap size: '%s'\n", strconv.FormatInt(v, 10))
			return 1
		}
		wrap = v
	}
	ignoreGarbage := flags.Bool("i")

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}
	input, ok := baseEncReadFiles(ctx, "base32", files)
	if !ok {
		return 1
	}

	if decode {
		src := strings.ToUpper(string(input))
		if ignoreGarbage {
			var clean strings.Builder
			for _, c := range src {
				if (c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7') || c == '=' {
					clean.WriteRune(c)
				}
			}
			src = clean.String()
		} else {
			src = strings.Map(func(r rune) rune {
				if r == '\n' || r == '\r' {
					return -1
				}
				return r
			}, src)
		}
		if pad := len(src) % 8; pad != 0 {
			src += strings.Repeat("=", 8-pad)
		}
		decoded, err := base32.StdEncoding.DecodeString(src)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "base32: error: invalid input")
			return 1
		}
		ctx.Stdout.Write(decoded)
	} else {
		encoded := base32.StdEncoding.EncodeToString(input)
		if wrap > 0 {
			for len(encoded) > int(wrap) {
				fmt.Fprintln(ctx.Stdout, encoded[:wrap])
				encoded = encoded[wrap:]
			}
		}
		fmt.Fprintln(ctx.Stdout, encoded)
	}
	return 0
}
