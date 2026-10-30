package builtins

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

func init() {
	Register(Command{Name: "hexdump", Run: runHexdump})
	Register(Command{Name: "hd", Run: runHD})
}

func runHD(ctx *Context) int {
	hd := *ctx
	hd.CmdName = "hd"
	return runHexdump(&hd)
}

type hexOptions struct {
	formats          []string
	names            []string
	skip, limit      uint64
	limited, verbose bool
	canonical        bool
}

func runHexdump(ctx *Context) int {
	name := ctx.CmdName
	if name == "" {
		name = "hexdump"
	}
	fail := func(err error) int { fmt.Fprintf(ctx.Stderr, "%s: %v\n", name, err); return 1 }
	o := hexOptions{canonical: name == "hd"}
	selected := false
	presets := map[byte]string{
		'b': `"%07.7_ax " 16/1 "%03o " "\n" "%07.7_Ax\n"`,
		'c': `"%07.7_ax " 16/1 "%3_c " "\n" "%07.7_Ax\n"`,
		'd': `"%07.7_ax " 8/2 "%05u " "\n" "%07.7_Ax\n"`,
		'o': `"%07.7_ax " 8/2 "%06o " "\n" "%07.7_Ax\n"`,
		'x': `"%07.7_ax " 8/2 "%04x " "\n" "%07.7_Ax\n"`,
	}
	for i := 0; i < len(ctx.Args); i++ {
		a := ctx.Args[i]
		if a == "--" {
			o.names = append(o.names, ctx.Args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' || a == "-" {
			o.names = append(o.names, a)
			continue
		}
		if a == "--help" {
			fmt.Fprint(ctx.Stdout, "Usage: "+name+" [-bcdoxCv] [-e format] [-f file] [-n length] [-s offset] [file ...]\n")
			return 0
		}
		if strings.HasPrefix(a, "--") {
			return fail(fmt.Errorf("unknown option %s", a))
		}
		for j := 1; j < len(a); j++ {
			ch := a[j]
			switch ch {
			case 'v':
				o.verbose = true
			case 'C':
				if !selected {
					o.canonical = false
				}
				selected = true
				o.canonical = true
				o.formats = append(o.formats, "@canonical")
			case 'b', 'c', 'd', 'o', 'x':
				if !selected {
					o.canonical = false
				}
				selected = true
				o.formats = append(o.formats, presets[ch])
			case 'e', 'f', 'n', 's':
				val := a[j+1:]
				if val == "" {
					i++
					if i >= len(ctx.Args) {
						return fail(fmt.Errorf("option -%c requires an argument", ch))
					}
					val = ctx.Args[i]
				}
				j = len(a)
				switch ch {
				case 'n', 's':
					n, e := odParseByteCount(val)
					if e != odSizeOK {
						return fail(fmt.Errorf("invalid byte count %q", val))
					}
					if ch == 'n' {
						o.limit = n
						o.limited = true
					} else {
						o.skip = n
					}
				case 'e':
					if !selected {
						o.canonical = false
					}
					selected = true
					o.formats = append(o.formats, val)
				case 'f':
					data, err := os.ReadFile(resolvePath(ctx, val))
					if err != nil {
						return fail(err)
					}
					if !selected {
						o.canonical = false
					}
					selected = true
					for _, line := range strings.Split(string(data), "\n") {
						line = strings.TrimSpace(line)
						if line != "" && !strings.HasPrefix(line, "#") {
							o.formats = append(o.formats, line)
						}
					}
				}
			default:
				return fail(fmt.Errorf("unknown option -%c", ch))
			}
		}
	}
	if len(o.formats) == 0 {
		if o.canonical {
			o.formats = []string{"@canonical"}
		} else {
			o.formats = []string{`"%07.7_ax " 8/2 "%04x " "\n" "%07.7_Ax\n"`}
		}
	}
	var formats []hexFormat
	block := 0
	for _, s := range o.formats {
		if s == "@canonical" {
			formats = append(formats, hexFormat{canonical: true, size: 16})
			if block < 16 {
				block = 16
			}
			continue
		}
		f, err := hexParse(s)
		if err != nil {
			return fail(err)
		}
		formats = append(formats, f)
		if f.size > block {
			block = f.size
		}
	}
	if block == 0 {
		return fail(fmt.Errorf("format consumes no input"))
	}
	if block > 1<<20 {
		return fail(fmt.Errorf("format block too large"))
	}
	if len(o.names) == 0 {
		o.names = []string{"-"}
	}
	stdin := ctx.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	// TODO(F05): propagate cancellation through odReader when Context gains it.
	// odReader already concatenates operands; adapt its diagnostic prefix.
	copyCtx := *ctx
	copyCtx.Stderr = hexErrorWriter{ctx.Stderr, name}
	r := &odReader{ctx: &copyCtx, names: o.names, stdin: stdin}
	defer r.closeCur()
	if o.skip > 0 && r.skip(o.skip) > 0 {
		return fail(fmt.Errorf("cannot skip past end of combined input"))
	}
	buf := make([]byte, block)
	prev := make([]byte, block)
	have, star := false, false
	anyData := false
	pos := o.skip
	left := o.limit
	emit := func(s string) bool {
		_, err := io.WriteString(ctx.Stdout, s)
		if err != nil {
			fail(fmt.Errorf("write error: %v", err))
			return false
		}
		return true
	}
	for {
		want := block
		if o.limited {
			if left == 0 {
				break
			}
			if left < uint64(want) {
				want = int(left)
			}
		}
		n := r.read(buf[:want])
		if n == 0 {
			break
		}
		anyData = true
		if o.limited {
			left -= uint64(n)
		}
		clear(buf[n:])
		if !o.verbose && n == block && have && bytes.Equal(buf, prev) {
			if !star {
				if !emit("*\n") {
					return 1
				}
				star = true
			}
		} else {
			star = false
			if n == block {
				copy(prev, buf)
				have = true
			}
			for _, f := range formats {
				if !emit(f.render(buf, n, pos, false, block)) {
					return 1
				}
			}
		}
		pos += uint64(n)
	}
	for _, f := range formats {
		if !anyData {
			break
		}
		if !emit(f.render(nil, 0, pos, true, block)) {
			return 1
		}
	}
	if r.failed {
		return 1
	}
	return 0
}

type hexErrorWriter struct {
	w    io.Writer
	name string
}

func (w hexErrorWriter) Write(p []byte) (int, error) {
	s := strings.Replace(string(p), "od:", w.name+":", 1)
	_, err := io.WriteString(w.w, s)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
