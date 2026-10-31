// Behaviour adapted from toybox xxd.c (0BSD).
package builtins

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func init() { Register(Command{Name: "xxd", Run: runXXD}) }

type xxdOptions struct {
	cols, group, limit              int
	offset, skip                    int64
	plain, include, reverse, little bool
}

func runXXD(ctx *Context) (code int) {
	originalInput := ctx.Stdin
	ctx, cleanup := streamContext(ctx)
	defer cleanup()
	defer streamResult(ctx, &code)
	out := &trackedOutput{writer: ctx.Stdout}
	ctx.Stdout = out
	o, names, err := xxdParse(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "xxd: %v\n", err)
		return 2
	}
	var input io.Reader = ctx.Stdin
	name := "-"
	if len(names) > 1 {
		filtered := names[:0]
		for _, n := range names {
			if n != "-" && n != "<" {
				filtered = append(filtered, n)
			}
		}
		names = filtered
	}
	if len(names) > 0 && names[0] != "-" && names[0] != "<" {
		name = names[0]
		f, e := os.Open(resolvePath(ctx, name))
		if e != nil {
			fmt.Fprintf(ctx.Stderr, "xxd: %s: %s\n", name, fmtOsErr(e))
			return 2
		}
		defer f.Close()
		originalInput = f
		input = ContextReader(ctx.CancellationContext(), f)
	}
	if input == nil {
		input = strings.NewReader("")
	}
	var data []byte
	if o.skip > 0 {
		o.offset += o.skip
		if seeker, ok := originalInput.(io.Seeker); ok {
			_, _ = seeker.Seek(o.skip, io.SeekStart)
		} else {
			_, _ = io.CopyN(io.Discard, input, o.skip)
		}
	}
	if o.limit > 0 {
		input = io.LimitReader(input, int64(o.limit))
	}
	data, err = io.ReadAll(input)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "xxd: read: %s\n", err)
		return 2
	}
	if o.limit > 0 && len(data) > o.limit {
		data = data[:o.limit]
	}
	if o.reverse {
		return xxdReverse(ctx, data, o)
	}
	if o.include {
		includeCols := o.cols
		if includeCols == 16 {
			includeCols = 12
		}
		xxdInclude(ctx.Stdout, data, includeCols, name == "-")
		if out.err != nil {
			return 1
		}
		return 0
	}
	xxdDump(ctx.Stdout, data, o)
	if out.err != nil {
		return 1
	}
	return 0
}

func xxdParse(args []string) (xxdOptions, []string, error) {
	o := xxdOptions{cols: 16, group: 2}
	groupSet := false
	names := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' || a == "-" {
			names = append(names, a)
			continue
		}
		for j := 1; j < len(a); j++ {
			ch := a[j]
			if strings.ContainsRune("cglos", rune(ch)) {
				v := a[j+1:]
				if v == "" {
					i++
					if i >= len(args) {
						return o, nil, fmt.Errorf("option requires an argument -- '%c'", ch)
					}
					v = args[i]
				}
				n, e := strconv.ParseInt(v, 0, 64)
				if e != nil || n < 0 {
					return o, nil, fmt.Errorf("invalid number '%s'", v)
				}
				switch ch {
				case 'c':
					o.cols = int(n)
				case 'g':
					o.group = int(n)
					groupSet = true
				case 'l':
					o.limit = int(n)
				case 'o':
					o.offset = n
				case 's':
					o.skip = n
				}
				break
			}
			switch ch {
			case 'p':
				o.plain = true
			case 'i':
				o.include = true
			case 'r':
				o.reverse = true
			case 'e':
				o.little = true
			default:
				return o, nil, fmt.Errorf("unknown option -- '%c'", ch)
			}
		}
	}
	if o.cols == 0 {
		o.cols = 30
	}
	if o.group == 0 {
		o.group = o.cols
	}
	if o.little && !groupSet {
		o.group = 4
	}
	if o.plain {
		o.cols = 30
		if o.group == 0 {
			o.group = o.cols
		}
	}
	return o, names, nil
}

func xxdDump(w io.Writer, data []byte, o xxdOptions) {
	cols := o.cols
	if cols < 1 {
		cols = 30
	}
	for pos := 0; pos < len(data); pos += cols {
		if out, ok := w.(*trackedOutput); ok && out.err != nil {
			return
		}
		end := pos + cols
		if end > len(data) {
			end = len(data)
		}
		row := data[pos:end]
		if !o.plain {
			fmt.Fprintf(w, "%08x: ", o.offset+int64(pos))
			hex := bytes.Buffer{}
			j := 1
			for i := 0; i < len(row); {
				if o.little && j == 1 && len(row)-i < o.group {
					for k := 0; k < o.group-(len(row)-i); k++ {
						hex.WriteString("  ")
						j++
					}
				}
				idx := i
				if o.little {
					idx = i + o.group - j
				}
				if idx < 0 || idx >= len(row) {
					idx = i
				}
				fmt.Fprintf(&hex, "%02x", row[idx])
				i++
				j += 2
				if o.group > 0 && i%o.group == 0 {
					hex.WriteByte(' ')
					j = 1
				}
			}
			width := 2*cols + (cols+o.group-1)/o.group + 1
			if o.group <= 0 {
				width = 2*cols + 2
			}
			pad := width - hex.Len()
			if pad < 0 {
				pad = 0
			}
			fmt.Fprintf(w, "%s%*s", hex.String(), pad, "")
			for _, b := range row {
				if b >= 32 && b <= 126 {
					fmt.Fprintf(w, "%c", b)
				} else {
					fmt.Fprint(w, ".")
				}
			}
			fmt.Fprintln(w)
		} else {
			for _, b := range row {
				fmt.Fprintf(w, "%02x", b)
			}
			fmt.Fprintln(w)
		}
	}
	if o.plain && len(data) == 0 {
		fmt.Fprintln(w)
	}
}
func xxdInclude(w io.Writer, data []byte, cols int, stdin bool) {
	if cols < 1 {
		cols = 12
	}
	for i, b := range data {
		if out, ok := w.(*trackedOutput); ok && out.err != nil {
			return
		}
		if i%cols == 0 {
			fmt.Fprint(w, "  ")
		} else {
			fmt.Fprint(w, " ")
		}
		fmt.Fprintf(w, "0x%02x", b)
		if i+1 < len(data) {
			fmt.Fprint(w, ",")
		}
		if (i+1)%cols == 0 {
			fmt.Fprintln(w)
		}
	}
	if len(data)%cols != 0 {
		fmt.Fprintln(w)
	}
	_ = stdin
}

// xxdMaxOffset bounds the zero-filled gap a reverse dump may ask for: the
// output is built in memory, and in-process builtins share Misa's.
const xxdMaxOffset = 256 << 20

func xxdReverse(ctx *Context, data []byte, o xxdOptions) int {
	var out []byte
	pos := int64(0)
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		s := strings.TrimSpace(string(line))
		if !o.plain {
			fields := strings.Fields(s)
			if len(fields) == 0 {
				continue
			}
			n, err := strconv.ParseInt(strings.TrimSuffix(fields[0], ":"), 16, 64)
			if err == nil {
				if n < 0 || n > xxdMaxOffset {
					fmt.Fprintf(ctx.Stderr, "xxd: offset %s out of range\n", fields[0])
					return 2
				}
				pos = n
				s = strings.TrimLeft(strings.TrimPrefix(s, fields[0]), " :\t")
			}
		}
		if pos > int64(len(out)) {
			out = append(out, make([]byte, int(pos)-len(out))...)
		}
		count := 0
		for i := 0; i < len(s); {
			if s[i] == ' ' || s[i] == '\t' || s[i] == '\r' {
				i++
				continue
			}
			if i+1 >= len(s) || !isXXDHex(s[i]) || !isXXDHex(s[i+1]) {
				break
			}
			b, _ := strconv.ParseUint(s[i:i+2], 16, 8)
			if pos == int64(len(out)) {
				out = append(out, byte(b))
			} else if pos < int64(len(out)) {
				out[pos] = byte(b)
			} else {
				out = append(out, make([]byte, int(pos)-len(out))...)
				out = append(out, byte(b))
			}
			pos++
			count++
			i += 2
			if !o.plain && o.cols > 0 && count >= o.cols {
				break
			}
		}
	}
	if _, err := ctx.Stdout.Write(out); err != nil {
		fmt.Fprintf(ctx.Stderr, "xxd: write: %s\n", err)
		return 2
	}
	return 0
}

func xxdAllHex(s string) bool {
	if s == "" {
		return false
	}
	for i := range s {
		if !isXXDHex(s[i]) {
			return false
		}
	}
	return true
}

func isXXDHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
