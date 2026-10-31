// Behaviour adapted from toybox fallocate.c and tests/fallocate.test (0BSD).
package builtins

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const fallocateSpec = "o(offset):l(length):"

// errFallocateUnsupported is the honest result on platforms without a Go
// binding for an allocation syscall.
var errFallocateUnsupported = errors.New("not supported on this platform")

var fallocateSpecParsed *Spec

func init() {
	var err error
	fallocateSpecParsed, err = ParseSpec(fallocateSpec)
	if err != nil {
		panic("builtins: bad fallocate spec: " + err.Error())
	}
	Register(Command{Name: "fallocate", Run: runFallocate})
}

func runFallocate(ctx *Context) int {
	flags, err := fallocateSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "fallocate:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, fallocateSpecParsed) {
		return 0
	}
	switch len(flags.Operands) {
	case 0:
		fmt.Fprintln(ctx.Stderr, "fallocate: missing operand")
		return 1
	case 1:
	default:
		fmt.Fprintf(ctx.Stderr, "fallocate: extra operand '%s'\n", flags.Operands[1])
		return 1
	}
	lengthArg, ok := flags.ExplicitString("l")
	if !ok {
		fmt.Fprintln(ctx.Stderr, "fallocate: -l LEN is required")
		return 1
	}
	length, err := atolx(lengthArg)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "fallocate: %v\n", err)
		return 1
	}
	offset := int64(0)
	if offsetArg, ok := flags.ExplicitString("o"); ok {
		offset, err = atolx(offsetArg)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "fallocate: %v\n", err)
			return 1
		}
	}

	name := flags.Operands[0]
	if !fallocateSupported {
		fmt.Fprintf(ctx.Stderr, "fallocate: %s: %s\n", name, errFallocateUnsupported)
		return 1
	}
	file, err := os.OpenFile(resolvePath(ctx, name), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "fallocate: %s: %s\n", name, fmtOsErr(err))
		return 1
	}
	defer file.Close()
	if err := fallocateFile(file, offset, length); err != nil {
		fmt.Fprintf(ctx.Stderr, "fallocate: %s: %s\n", name, fmtOsErr(err))
		return 1
	}
	return 0
}

// atolx parses a size the way toybox's lib/lib.c atolx does: decimal digits, an
// optional 0x hexadecimal form, an optional suffix from "cwbkmgtpe" (b is 512,
// the rest are 1024 steps) with a trailing 'd' switching to 1000 steps, and
// surrounding whitespace.
func atolx(raw string) (int64, error) {
	s := strings.TrimLeft(raw, " \t\n\v\f\r")
	base := 10
	if strings.Contains(strings.ToLower(s), "0x") {
		base = 0
	}
	digits := numericPrefix(s, base)
	if digits == "" || digits == "-" {
		return 0, fmt.Errorf("not an integer: %s", raw)
	}
	value, err := strconv.ParseInt(digits, base, 64)
	if err != nil {
		return 0, fmt.Errorf("not an integer: %s", raw)
	}
	rest := s[len(digits):]
	if rest != "" {
		index := strings.IndexByte("cwbkmgtpe", byte(strings.ToLower(rest[:1])[0]))
		if index >= 0 {
			rest = rest[1:]
			shift := index - 2
			switch {
			case shift == -1:
				value, err = mulOverflow(value, 2)
			case shift == 0:
				value, err = mulOverflow(value, 512)
			case shift > 0:
				multiplier := int64(1) << (shift * 10)
				if strings.HasPrefix(strings.ToLower(rest), "d") {
					rest = rest[1:]
					multiplier = 1
					for i := 0; i < shift; i++ {
						multiplier *= 1000
					}
				}
				value, err = mulOverflow(value, multiplier)
			}
			if err != nil {
				return 0, fmt.Errorf("not an integer: %s", raw)
			}
		}
	}
	if strings.TrimLeft(rest, " \t\n\v\f\r") != "" {
		return 0, fmt.Errorf("not an integer: %s", raw)
	}
	return value, nil
}

// numericPrefix returns the longest prefix strconv.ParseInt accepts for base.
func numericPrefix(s string, base int) string {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if base == 0 {
		if i+1 < len(s) && s[i] == '0' && (s[i+1] == 'x' || s[i+1] == 'X') {
			start := i + 2
			i = start
			for i < len(s) && isHexDigit(s[i]) {
				i++
			}
			if i == start {
				return ""
			}
			return s[:i]
		}
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return ""
	}
	return s[:i]
}

func mulOverflow(value, multiplier int64) (int64, error) {
	if multiplier != 0 && value > 0 && value > math.MaxInt64/multiplier {
		return 0, fmt.Errorf("value too large")
	}
	if multiplier != 0 && value < 0 && value < math.MinInt64/multiplier {
		return 0, fmt.Errorf("value too large")
	}
	return value * multiplier, nil
}
