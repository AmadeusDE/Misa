package builtins

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

const trSpec = "^d(delete)s(squeeze-repeats)c(complement)C(complement-upper)t(truncate-set1)"

var trSpecParsed *Spec

func init() {
	var err error
	trSpecParsed, err = ParseSpec(trSpec)
	if err != nil {
		panic("builtins: bad tr spec: " + err.Error())
	}
	Register(Command{Name: "tr", Run: runTr})
}

func parseTrChar(spec string, i int) (byte, int, string) {
	if i >= len(spec) {
		return 0, 0, ""
	}
	if spec[i] == '\\' && i+1 < len(spec) {
		switch spec[i+1] {
		case 'n':
			return '\n', 2, ""
		case 't':
			return '\t', 2, ""
		case 'r':
			return '\r', 2, ""
		case 'a':
			return '\a', 2, ""
		case 'b':
			return '\b', 2, ""
		case 'f':
			return '\f', 2, ""
		case 'v':
			return '\v', 2, ""
		case '\\':
			return '\\', 2, ""
		case '0', '1', '2', '3', '4', '5', '6', '7':
			idx := i + 1
			d0 := spec[idx]
			val := int(d0 - '0')
			consumed := 2
			idx++
			var d1, d2 byte
			if idx < len(spec) && spec[idx] >= '0' && spec[idx] <= '7' {
				d1 = spec[idx]
				val = val*8 + int(d1-'0')
				consumed++
				idx++
				if idx < len(spec) && spec[idx] >= '0' && spec[idx] <= '7' {
					d2 = spec[idx]
					val = val*8 + int(d2-'0')
					consumed++
					if val > 255 {
						v1 := (int(d0-'0') * 8) + int(d1-'0')
						warn := fmt.Sprintf("tr: warning: the ambiguous octal escape \\%c%c%c is being interpreted as the 2-byte sequence \\0%c%c, %c\n", d0, d1, d2, d0, d1, d2)
						return byte(v1), consumed - 1, warn
					}
				}
			}
			return byte(val), consumed, ""
		default:
			return spec[i+1], 2, ""
		}
	}
	return spec[i], 1, ""
}

func expandTrSet(spec string, warnings io.Writer) ([]byte, error) {
	var out []byte
	i := 0
	for i < len(spec) {
		if strings.HasPrefix(spec[i:], "[:") {
			end := strings.Index(spec[i:], ":]")
			if end < 2 {
				return nil, fmt.Errorf("invalid character class")
			}
			className := spec[i+2 : i+end]
			switch className {
			case "upper":
				for c := byte('A'); c <= 'Z'; c++ {
					out = append(out, c)
				}
			case "lower":
				for c := byte('a'); c <= 'z'; c++ {
					out = append(out, c)
				}
			case "digit":
				for c := byte('0'); c <= '9'; c++ {
					out = append(out, c)
				}
			case "alpha":
				for c := byte('A'); c <= 'Z'; c++ {
					out = append(out, c)
				}
				for c := byte('a'); c <= 'z'; c++ {
					out = append(out, c)
				}
			case "alnum":
				for c := byte('A'); c <= 'Z'; c++ {
					out = append(out, c)
				}
				for c := byte('a'); c <= 'z'; c++ {
					out = append(out, c)
				}
				for c := byte('0'); c <= '9'; c++ {
					out = append(out, c)
				}
			case "xdigit":
				for c := byte('0'); c <= '9'; c++ {
					out = append(out, c)
				}
				for c := byte('A'); c <= 'F'; c++ {
					out = append(out, c)
				}
				for c := byte('a'); c <= 'f'; c++ {
					out = append(out, c)
				}
			case "space":
				out = append(out, ' ', '\t', '\n', '\r', '\f', '\v')
			case "blank":
				out = append(out, ' ', '\t')
			case "cntrl":
				for c := byte(0); c <= 31; c++ {
					out = append(out, c)
				}
				out = append(out, 127)
			case "graph":
				for c := byte(33); c <= 126; c++ {
					out = append(out, c)
				}
			case "print":
				for c := byte(32); c <= 126; c++ {
					out = append(out, c)
				}
			case "punct":
				for _, c := range []byte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~") {
					out = append(out, c)
				}
			default:
				return nil, fmt.Errorf("invalid character class '%s'", className)
			}
			i += end + 2
		} else if spec[i] == '[' && strings.Contains(spec[i:], "*") {
			end := strings.IndexByte(spec[i:], ']')
			star := strings.IndexByte(spec[i:], '*')
			if end > 0 && star > 0 && star < end {
				charByte := spec[i+1]
				if star == i+1 {
					charByte = '*'
				}
				countStr := spec[i+star+1 : i+end]
				count := 0
				if countStr == "" {
					count = 1
				} else {
					n, err := strconv.ParseInt(countStr, 8, 64)
					if err != nil {
						n, err = strconv.ParseInt(countStr, 10, 64)
					}
					if err != nil || n < 0 {
						return nil, fmt.Errorf("invalid repeat count '%s' in [c*n] construct", countStr)
					}
					count = int(n)
				}
				for k := 0; k < count; k++ {
					out = append(out, charByte)
				}
				i += end + 1
			} else {
				out = append(out, spec[i])
				i++
			}
		} else {
			c1, n1, w1 := parseTrChar(spec, i)
			if w1 != "" && warnings != nil {
				fmt.Fprint(warnings, w1)
			}
			if i+n1 < len(spec) && spec[i+n1] == '-' && i+n1+1 < len(spec) {
				c2, n2, w2 := parseTrChar(spec, i+n1+1)
				if w2 != "" && warnings != nil {
					fmt.Fprint(warnings, w2)
				}
				lo, hi := c1, c2
				if lo > hi {
					lo, hi = hi, lo
				}
				for c := lo; c <= hi; c++ {
					out = append(out, c)
				}
				i += n1 + 1 + n2
			} else {
				out = append(out, c1)
				i += n1
			}
		}
	}
	return out, nil
}

func runTr(ctx *Context) (code int) {
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	if ctx.CancellationContext().Err() != nil {
		return 0
	}
	flags, err := trSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tr:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, trSpecParsed) {
		return 0
	}

	deleteMode := flags.Bool("d")
	squeeze := flags.Bool("s")
	complement := flags.Bool("c") || flags.Bool("C")
	truncate := flags.Bool("t")

	operands := flags.Operands
	if len(operands) < 1 {
		fmt.Fprintln(ctx.Stderr, "tr: missing operand")
		return 1
	}
	if deleteMode && !squeeze && len(operands) > 1 {
		fmt.Fprintf(ctx.Stderr, "tr: extra operand '%s'\n", operands[1])
		return 1
	}
	if len(operands) > 2 {
		fmt.Fprintf(ctx.Stderr, "tr: extra operand '%s'\n", operands[2])
		return 1
	}
	if complement && truncate {
		fmt.Fprintln(ctx.Stderr, "tr: when translating with complemented character classes,\nstring2 must map all characters in the domain to one")
		return 1
	}
	if deleteMode && squeeze && len(operands) < 2 {
		fmt.Fprintln(ctx.Stderr, "tr: Two strings must be given when deleting and squeezing.")
		return 1
	}
	if !deleteMode && !squeeze && len(operands) < 2 && !truncate {
		fmt.Fprintln(ctx.Stderr, "tr: missing operand after set1")
		return 1
	}

	set1, err := expandTrSet(operands[0], ctx.Stderr)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "tr:", err)
		return 1
	}

	if complement {
		full := make([]bool, 256)
		for _, c := range set1 {
			full[c] = true
		}
		var inv []byte
		for i := 0; i < 256; i++ {
			if !full[byte(i)] {
				inv = append(inv, byte(i))
			}
		}
		set1 = inv
	}

	inSet1 := make([]bool, 256)
	for _, c := range set1 {
		inSet1[c] = true
	}

	var transMap [256]byte
	for i := range transMap {
		transMap[i] = byte(i)
	}

	var set2 []byte
	if len(operands) > 1 {
		set2, err = expandTrSet(operands[1], ctx.Stderr)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, "tr:", err)
			return 1
		}
		if truncate && len(set1) > len(set2) {
			set1 = set1[:len(set2)]
			inSet1 = make([]bool, 256)
			for _, c := range set1 {
				inSet1[c] = true
			}
		}
		if len(set2) > 0 && !deleteMode {
			for i, c := range set1 {
				rep := set2[len(set2)-1]
				if i < len(set2) {
					rep = set2[i]
				}
				transMap[c] = rep
			}
		}
	}

	input, err := io.ReadAll(ctx.Stdin)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "tr: %v\n", err)
		return 1
	}

	var out []byte
	var lastOut byte
	first := true

	for _, b := range input {
		if deleteMode && inSet1[b] {
			continue
		}

		c := transMap[b]

		if squeeze {
			squeezeSet := inSet1
			if len(set2) > 0 {
				squeezeSet = make([]bool, 256)
				for _, s := range set2 {
					squeezeSet[s] = true
				}
			}
			if !first && c == lastOut && squeezeSet[c] {
				continue
			}
		}

		out = append(out, c)
		lastOut = c
		first = false
	}

	ctx.Stdout.Write(out)
	return 0
}
