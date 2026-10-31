package builtins

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const base64Spec = "d(decode)w(wrap)#i(ignore-garbage)"

var base64SpecParsed *Spec

func init() {
	var err error
	base64SpecParsed, err = ParseSpec(base64Spec)
	if err != nil {
		panic("builtins: bad base64 spec: " + err.Error())
	}
	Register(Command{Name: "base64", Run: runBase64})
}

func baseEncFormatParseError(ctx *Context, cmdName string, err error) {
	msg := err.Error()
	if strings.Contains(msg, "-w") || strings.Contains(msg, "--wrap") {
		if strings.Contains(msg, "requires an argument") {
			fmt.Fprintf(ctx.Stderr, "%s: %s\nFor more information, try '--help'.\n", cmdName, msg)
			return
		}
		if strings.Contains(msg, "invalid value '") {
			start := strings.Index(msg, "invalid value '") + len("invalid value '")
			end := strings.Index(msg[start:], "'") + start
			val := msg[start:end]
			fmt.Fprintf(ctx.Stderr, "%s: invalid wrap size: '%s'\n", cmdName, val)
			return
		}
	}
	fmt.Fprintln(ctx.Stderr, cmdName+":", msg)
}

// normBaseEncArgs normalizes -D to -d (uutils compatibility).
func normBaseEncArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "-D" {
			out = append(out, "-d")
		} else {
			out = append(out, a)
		}
	}
	return out
}

func baseEncReadFiles(ctx *Context, cmdName string, files []string) ([]byte, bool) {
	var input []byte
	for _, f := range files {
		if f == "-" || f == "" {
			if ctx.Stdin == nil {
				fmt.Fprintln(ctx.Stderr, cmdName+": stdin not available")
				return nil, false
			}
			data, err := io.ReadAll(ctx.Stdin)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "%s: read error: %s\n", cmdName, fmtOsErr(err))
				return nil, false
			}
			input = append(input, data...)
			continue
		}
		path := resolvePath(ctx, f)
		fh, openErr := os.Open(path)
		if openErr != nil {
			fmt.Fprintf(ctx.Stderr, "%s: %s: %s\n", cmdName, f, fmtOsErr(openErr))
			return nil, false
		}
		data, readErr := io.ReadAll(ContextReader(ctx.CancellationContext(), fh))
		fh.Close()
		if readErr != nil {
			fmt.Fprintf(ctx.Stderr, "%s: read error: %s\n", cmdName, fmtOsErr(readErr))
			return nil, false
		}
		input = append(input, data...)
	}
	return input, true
}

func runBase64(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	args := normBaseEncArgs(ctx.Args)
	flags, err := base64SpecParsed.ParseArgs(args)
	if err != nil {
		baseEncFormatParseError(ctx, "base64", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, base64SpecParsed) {
		return 0
	}

	decode := flags.Bool("d")
	wrap := int64(76)
	if v, ok := flags.Num("w"); ok {
		if v < 0 {
			fmt.Fprintf(ctx.Stderr, "base64: invalid wrap size: '%s'\n", strconv.FormatInt(v, 10))
			return 1
		}
		wrap = v
	}
	ignoreGarbage := flags.Bool("i")

	files := flags.Operands
	if len(files) == 0 {
		files = []string{"-"}
	}
	input, ok := baseEncReadFiles(ctx, "base64", files)
	if !ok {
		return 1
	}

	if decode {
		src := string(input)
		if ignoreGarbage {
			var clean strings.Builder
			for _, c := range src {
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
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
		decoded, decErr := decodeBase64Stream(src)
		if decErr != nil {
			fmt.Fprintf(ctx.Stderr, "base64: error: invalid input\n")
			return 1
		}
		ctx.Stdout.Write(decoded)
	} else {
		encoded := base64.StdEncoding.EncodeToString(input)
		if wrap > 0 {
			for len(encoded) > int(wrap) {
				fmt.Fprintln(ctx.Stdout, encoded[:wrap])
				encoded = encoded[wrap:]
			}
			fmt.Fprintln(ctx.Stdout, encoded)
		} else {
			fmt.Fprint(ctx.Stdout, encoded)
		}
	}
	return 0
}

func decodeBase64Stream(src string) ([]byte, error) {
	var result []byte
	for len(src) > 0 {
		end := strings.IndexByte(src, '=')
		if end < 0 {
			if pad := len(src) % 4; pad != 0 {
				src += strings.Repeat("=", 4-pad)
			}
			decoded, err := base64.StdEncoding.DecodeString(src)
			if err != nil {
				return nil, err
			}
			result = append(result, decoded...)
			break
		}
		padEnd := end
		for padEnd < len(src) && src[padEnd] == '=' {
			padEnd++
		}
		chunk := src[:padEnd]
		decoded, err := base64.StdEncoding.DecodeString(chunk)
		if err != nil {
			return nil, err
		}
		result = append(result, decoded...)
		src = src[padEnd:]
	}
	return result, nil
}
