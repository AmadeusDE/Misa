// Behaviour adapted from uutils coreutils od (MIT), commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663, files src/uu/od/src/od.rs,
// parse_formats.rs, parse_inputs.rs, parse_nrofbytes.rs, output_info.rs,
// input_offset.rs, input_decoder.rs and multifile_reader.rs.
// Toybox toys/posix/od.c (0BSD) at 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb
// was consulted for the traditional option set.
package builtins

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

func init() { Register(Command{Name: "od", Run: runOd}) }

type odRadix int

const (
	odRadixOctal odRadix = iota
	odRadixDecimal
	odRadixHex
	odRadixNone
)

type odOptions struct {
	radix      odRadix
	skip       uint64
	limit      uint64
	hasLimit   bool
	endian     binary.ByteOrder
	formats    []odFormat
	width      int
	duplicates bool
	names      []string
}

// odUsage is what `od --help` prints.
func odUsage(name string) string {
	if name == "" {
		name = "od"
	}
	return "Usage: " + name + " [OPTION]... [FILE]...\n\n" +
		"  -A, --address-radix=RADIX   output format for file offsets: d, o, x, n\n" +
		"  -j, --skip-bytes=BYTES      skip BYTES input bytes first\n" +
		"  -N, --read-bytes=BYTES      limit dump to BYTES input bytes\n" +
		"  -t, --format=TYPE           select output format(s)\n" +
		"  -a, -b, -c, -d, -e, -f, -i, -l, -o, -s, -x\n" +
		"                              traditional type selectors\n" +
		"  -v, --output-duplicates     do not use * to mark line suppression\n" +
		"  -w, --width[=BYTES]         output BYTES bytes per line (default 16)\n" +
		"      --endian=big|little     byte order of the input\n" +
		"  -h, --help                  display this help and exit\n" +
		"      --version               output version information and exit\n"
}

func runOd(ctx *Context) int {
	o, err := odParseOptions(ctx)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "od: %s\n", err)
		return 1
	}
	if o == nil {
		return 0
	}

	block := 1
	for _, f := range o.formats {
		if f.size > block {
			block = f.size
		}
	}
	if o.width%block != 0 {
		fmt.Fprintf(ctx.Stderr, "od: warning: invalid width %d; using %d instead\n", o.width, block)
		o.width = block
	}
	printBlock := 1
	for _, f := range o.formats {
		if w := f.width * (block / f.size); w > printBlock {
			printBlock = w
		}
	}
	spacings := make([][16]int, len(o.formats))
	for i, f := range o.formats {
		spacings[i] = odSpacing(f, block, printBlock)
	}

	stdin := ctx.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	r := &odReader{ctx: ctx, names: o.names, stdin: stdin}
	if o.skip > 0 {
		if remaining := r.skip(o.skip); remaining > 0 {
			fmt.Fprintln(ctx.Stderr, "od: cannot skip past end of combined input")
			return 1
		}
	}
	status := odDump(ctx, o, r, block, printBlock*(o.width/block), spacings)
	if status != 0 || r.failed {
		return 1
	}
	return 0
}

// odDump reads the input in width-sized lines and prints them.
func odDump(ctx *Context, o *odOptions, r *odReader, block, printLine int, spacings [][16]int) int {
	buf := make([]byte, o.width)
	prev := make([]byte, o.width)
	havePrev, starred, writeErr := false, false, false
	pos := o.skip
	remaining := o.limit
	emit := func(s string) {
		if _, err := io.WriteString(ctx.Stdout, s); err != nil {
			fmt.Fprintf(ctx.Stderr, "od: write error: %v\n", err)
			writeErr = true
		}
	}
	for {
		want := len(buf)
		if o.hasLimit {
			if remaining == 0 {
				break
			}
			if uint64(want) > remaining {
				want = int(remaining)
			}
		}
		n := r.read(buf[:want])
		if n == 0 {
			break
		}
		if o.hasLimit {
			remaining -= uint64(n)
		}
		for i := n; i < len(buf); i++ {
			buf[i] = 0
		}
		if !o.duplicates && n == len(buf) && havePrev && bytes.Equal(buf, prev) {
			if !starred {
				emit("*\n")
				starred = true
			}
		} else {
			starred = false
			if n == len(buf) {
				copy(prev, buf)
				havePrev = true
			}
			emit(odLine(o, buf, n, pos, block, printLine, spacings))
		}
		pos += uint64(n)
		if writeErr {
			return 1
		}
	}
	if o.radix != odRadixNone && !r.failed {
		emit(o.offsetText(pos) + "\n")
	}
	if writeErr {
		return 1
	}
	return 0
}

// odLine renders every format of one raster line.
func odLine(o *odOptions, buf []byte, n int, pos uint64, block, printLine int, spacings [][16]int) string {
	var b strings.Builder
	offset := o.offsetText(pos)
	for i, f := range o.formats {
		if i == 0 {
			b.WriteString(offset)
		} else if len(offset) > 0 {
			b.WriteString(strings.Repeat(" ", len(offset)))
		}
		lineWidth := 0
		for p := 0; p < n; p += f.size {
			if sp := spacings[i][p%block]; sp > 0 {
				b.WriteString(strings.Repeat(" ", sp))
				lineWidth += sp
			}
			field := f.odField(buf, p, o.endian)
			b.WriteString(field)
			lineWidth += utf8.RuneCountInString(field)
		}
		if f.dump {
			missing := printLine - lineWidth
			if missing < 0 {
				missing = 0
			}
			b.WriteString(strings.Repeat(" ", missing+2))
			b.WriteString(odDumpText(buf[:n]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (o *odOptions) offsetText(pos uint64) string {
	switch o.radix {
	case odRadixDecimal:
		return fmt.Sprintf("%07d", pos)
	case odRadixHex:
		return fmt.Sprintf("%06x", pos)
	case odRadixNone:
		return ""
	}
	return fmt.Sprintf("%07o", pos)
}

// odReader concatenates the operands, stdin included, into one byte stream.
type odReader struct {
	ctx     *Context
	names   []string
	stdin   io.Reader
	next    int
	file    *os.File
	cur     io.Reader
	curName string
	failed  bool
}

func (r *odReader) closeCur() {
	if r.file != nil {
		r.file.Close()
	}
	r.file, r.cur, r.curName = nil, nil, ""
}

func (r *odReader) name() string {
	if r.curName == "" {
		return "standard input"
	}
	return r.curName
}

// openNext moves to the next readable operand, reporting operands that cannot
// be opened and skipping them like od does.
func (r *odReader) openNext() bool {
	for r.next < len(r.names) {
		name := r.names[r.next]
		r.next++
		if name == "-" {
			r.cur, r.curName = r.stdin, "standard input"
			return true
		}
		f, err := os.Open(resolvePath(r.ctx, name))
		if err != nil {
			fmt.Fprintf(r.ctx.Stderr, "od: %s: %s\n", name, fmtOsErr(err))
			r.failed = true
			continue
		}
		r.file, r.cur, r.curName = f, f, name
		return true
	}
	return false
}

// read fills p from the concatenated input. It stops short only at the end of
// the last operand or on a read error.
func (r *odReader) read(p []byte) int {
	total := 0
	for total < len(p) {
		if r.cur == nil && !r.openNext() {
			break
		}
		n, err := r.cur.Read(p[total:])
		total += n
		if err != nil && err != io.EOF {
			fmt.Fprintf(r.ctx.Stderr, "od: %s: %s\n", r.name(), fmtOsErr(err))
			r.failed = true
			r.closeCur()
			continue
		}
		if n == 0 {
			r.closeCur()
		}
	}
	return total
}

// skip discards n bytes of the combined input and returns how many could not be
// skipped.
func (r *odReader) skip(n uint64) uint64 {
	var discard [16 * 1024]byte
	for n > 0 {
		if r.cur == nil && !r.openNext() {
			break
		}
		if consumed, done := r.trySeek(n); done {
			return 0
		} else if consumed > 0 {
			n -= consumed
			continue
		}
		for n > 0 && r.cur != nil {
			chunk := len(discard)
			if uint64(chunk) > n {
				chunk = int(n)
			}
			got, err := r.cur.Read(discard[:chunk])
			n -= uint64(got)
			if err != nil && err != io.EOF {
				fmt.Fprintf(r.ctx.Stderr, "od: %s: %s\n", r.name(), fmtOsErr(err))
				r.failed = true
				r.closeCur()
				break
			}
			if got == 0 {
				r.closeCur()
			}
		}
	}
	return n
}

// trySeek skips bytes of the current operand by seeking, when that is safe.
// A regular file is only seeked while the skip stays inside it, so that a proc
// file reporting size 0 is still skipped by reading. Other special files may be
// positioned past their end, like /dev/null. A regular file shorter than the
// skip is dropped entirely (consumed).
func (r *odReader) trySeek(n uint64) (consumed uint64, done bool) {
	if r.file == nil {
		return 0, false
	}
	st, err := r.file.Stat()
	if err != nil || n > math.MaxInt64 {
		return 0, false
	}
	var size uint64
	if st.Size() > 0 {
		size = uint64(st.Size())
	}
	if st.Mode().IsRegular() {
		if size > 0 && size <= n {
			r.closeCur()
			return size, false
		}
		if size <= n {
			return 0, false
		}
	}
	if _, err := r.file.Seek(int64(n), io.SeekCurrent); err == nil {
		return 0, true
	}
	return 0, false
}

// odParseOptions parses od's command line. It returns nil options when --help
// or --version was handled.
func odParseOptions(ctx *Context) (*odOptions, error) {
	o := &odOptions{radix: odRadixOctal, width: 16, endian: binary.NativeEndian}
	var formats []odFormat
	optionsEnded := false

	// value returns the argument of an option that takes one. attached is the
	// rest of a short option group, if any.
	takeValue := func(rest string, i *int, disp string) (string, error) {
		if rest != "" {
			return rest, nil
		}
		if *i+1 < len(ctx.Args) {
			*i++
			return ctx.Args[*i], nil
		}
		return "", fmt.Errorf("option requires an argument -- '%s'", disp)
	}

	for i := 0; i < len(ctx.Args) && !optionsEnded; i++ {
		arg := ctx.Args[i]
		switch {
		case arg == "--":
			o.names = append(o.names, ctx.Args[i+1:]...)
			optionsEnded = true
			continue
		case strings.HasPrefix(arg, "--"):
			name, inline, hasInline := strings.Cut(arg[2:], "=")
			disp := "--" + name
			needValue := func() (string, error) {
				if hasInline {
					return inline, nil
				}
				if i+1 < len(ctx.Args) {
					i++
					return ctx.Args[i], nil
				}
				return "", fmt.Errorf("option %s requires an argument", disp)
			}
			switch name {
			case "help":
				fmt.Fprint(ctx.Stdout, odUsage(ctx.CmdName))
				return nil, nil
			case "version":
				name := ctx.CmdName
				if name == "" {
					name = "od"
				}
				fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", name)
				return nil, nil
			case "output-duplicates":
				o.duplicates = true
			case "address-radix", "skip-bytes", "read-bytes", "format", "endian":
				val, err := needValue()
				if err != nil {
					return nil, err
				}
				if err := odApplyOption(o, &formats, name, disp, val); err != nil {
					return nil, err
				}
			case "width":
				val := "32"
				if hasInline {
					val = inline
				} else if i+1 < len(ctx.Args) && odAllDigits(ctx.Args[i+1]) {
					i++
					val = ctx.Args[i]
				}
				if err := odApplyWidth(o, disp, val); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unknown option '%s'", arg)
			}
		case len(arg) > 1 && arg[0] == '-':
			group := arg[1:]
			for j := 0; j < len(group); j++ {
				c := group[j]
				rest := group[j+1:]
				switch c {
				case 'h':
					fmt.Fprint(ctx.Stdout, odUsage(ctx.CmdName))
					return nil, nil
				case 'v':
					o.duplicates = true
				case 'a', 'b', 'c', 'd', 'e', 'f', 'i', 'l', 'o', 's', 'x',
					'B', 'D', 'F', 'H', 'I', 'L', 'O', 'X':
					formats = append(formats, odTraditionalFormat(c))
				case 'A', 'j', 'N', 't':
					val, err := takeValue(rest, &i, string(c))
					if err != nil {
						return nil, err
					}
					name := map[byte]string{'A': "address-radix", 'j': "skip-bytes", 'N': "read-bytes", 't': "format"}[c]
					if err := odApplyOption(o, &formats, name, "-"+string(c), val); err != nil {
						return nil, err
					}
					j = len(group)
				case 'w':
					val := rest
					if val == "" && i+1 < len(ctx.Args) && odAllDigits(ctx.Args[i+1]) {
						i++
						val = ctx.Args[i]
					}
					if err := odApplyWidth(o, "-w", val); err != nil {
						return nil, err
					}
					j = len(group)
				default:
					return nil, fmt.Errorf("unknown option '-%c'", c)
				}
			}
		default:
			o.names = append(o.names, arg)
		}
	}
	if o.names == nil {
		o.names = []string{"-"}
	}
	if len(formats) == 0 {
		formats = append(formats, odTraditionalFormat('o'))
	}
	o.formats = formats
	return o, nil
}

func odAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// odApplyOption handles one option that takes a value.
func odApplyOption(o *odOptions, formats *[]odFormat, name, disp, val string) error {
	switch name {
	case "address-radix":
		if val == "" {
			return fmt.Errorf("Radix cannot be empty, and must be one of [o, d, x, n]")
		}
		switch val[0] {
		case 'o':
			o.radix = odRadixOctal
		case 'd':
			o.radix = odRadixDecimal
		case 'x':
			o.radix = odRadixHex
		case 'n':
			o.radix = odRadixNone
		default:
			return fmt.Errorf("invalid %s argument '%s'", disp, val)
		}
	case "skip-bytes", "read-bytes":
		n, sizeErr := odParseByteCount(val)
		if sizeErr != odSizeOK {
			return fmt.Errorf("%s", odSizeMessage(sizeErr, "--"+name, disp, val))
		}
		if name == "skip-bytes" {
			o.skip = n
		} else {
			o.limit, o.hasLimit = n, true
		}
	case "format":
		parsed, err := odParseTypeString(val)
		if err != nil {
			return err
		}
		*formats = append(*formats, parsed...)
	case "endian":
		switch {
		case val != "" && strings.HasPrefix("little", val):
			o.endian = binary.LittleEndian
		case val != "" && strings.HasPrefix("big", val):
			o.endian = binary.BigEndian
		default:
			return fmt.Errorf("invalid --endian argument '%s'", val)
		}
	}
	return nil
}

// odApplyWidth validates the -w argument. An empty argument means -w without a
// value, which od treats as 32.
func odApplyWidth(o *odOptions, disp, val string) error {
	if val == "" {
		o.width = 32
		return nil
	}
	n, sizeErr := odParseWidth(val)
	if sizeErr == odSizeOK && n == 0 {
		return fmt.Errorf("invalid %s argument '%s'", disp, val)
	}
	if sizeErr != odSizeOK {
		return fmt.Errorf("%s", odSizeMessage(sizeErr, "--width", disp, val))
	}
	if n > math.MaxInt32 {
		return fmt.Errorf("%s argument '%s' too large", disp, val)
	}
	o.width = int(n)
	return nil
}

// odTraditionalFormat maps the traditional one-letter type selectors.
func odTraditionalFormat(c byte) odFormat {
	var kind odFormatKind
	size := 0
	switch c {
	case 'a':
		kind = odFmtAscii
	case 'b':
		kind, size = odFmtOct, 1
	case 'c':
		kind = odFmtChar
	case 'd':
		kind, size = odFmtDecU, 2
	case 'D':
		kind, size = odFmtDecU, 4
	case 'e', 'F':
		kind = odFmtDouble
	case 'f':
		kind = odFmtFloat
	case 'H', 'X':
		kind, size = odFmtHex, 4
	case 'h', 'x':
		kind, size = odFmtHex, 2
	case 'i':
		kind, size = odFmtDecS, 4
	case 'I', 'L', 'l':
		kind, size = odFmtDecS, 8
	case 'o', 'B':
		kind, size = odFmtOct, 2
	case 'O':
		kind, size = odFmtOct, 4
	case 's':
		kind, size = odFmtDecS, 2
	}
	switch kind {
	case odFmtAscii, odFmtChar, odFmtFloat, odFmtDouble:
	default:
		if size == 0 {
			size = 4
		}
	}
	switch kind {
	case odFmtAscii, odFmtChar:
		size = 1
	case odFmtFloat:
		size = 4
	case odFmtDouble:
		size = 8
	}
	f, ok := odNewFormat(kind, size)
	if !ok {
		// every traditional selector describes a supported type
		panic("builtins: bad od traditional format")
	}
	return f
}

// odParseTypeString parses one -t argument, which may describe several types.
func odParseTypeString(spec string) ([]odFormat, error) {
	var out []odFormat
	for i := 0; i < len(spec); {
		c := spec[i]
		i++
		var kind odFormatKind
		switch c {
		case 'a':
			kind = odFmtAscii
		case 'c':
			kind = odFmtChar
		case 'd':
			kind = odFmtDecS
		case 'o':
			kind = odFmtOct
		case 'u':
			kind = odFmtDecU
		case 'x':
			kind = odFmtHex
		case 'f':
			kind = odFmtDouble
		default:
			return nil, fmt.Errorf("invalid character '%c' in type string '%s'", c, spec)
		}

		size, variant := 0, byte(0)
		digitStart := i
		for kind != odFmtAscii && kind != odFmtChar && i < len(spec) && spec[i] >= '0' && spec[i] <= '9' {
			i++
		}
		if i > digitStart {
			size = odAtoi(spec[digitStart:i])
			if size <= 0 {
				return nil, fmt.Errorf("invalid number '%s' in type string '%s'", spec[digitStart:i], spec)
			}
		} else if kind == odFmtDouble {
			if i < len(spec) {
				switch spec[i] {
				case 'H', 'B':
					variant, size = spec[i], 2
					i++
				case 'F':
					size = 4
					i++
				case 'D':
					size = 8
					i++
				case 'S':
					size = 2
					i++
				case 'I':
					size = 4
					i++
				case 'L':
					size = 8
					i++
				}
			}
		} else if kind != odFmtAscii && kind != odFmtChar && i < len(spec) {
			switch spec[i] {
			case 'C':
				size = 1
				i++
			case 'S':
				size = 2
				i++
			case 'I':
				size = 4
				i++
			case 'L':
				size = 8
				i++
			}
		}

		switch variant {
		case 'B':
			kind = odFmtBFloat
		case 'H':
			kind = odFmtHalf
		}
		switch kind {
		case odFmtAscii, odFmtChar:
			size = 1
		case odFmtDouble:
			// -t f: 8-byte doubles, -t f2/f4 the narrower widths
			if size == 0 {
				size = 8
			}
			if size == 2 {
				kind = odFmtHalf
			} else if size == 4 {
				kind = odFmtFloat
			}
		default:
			if size == 0 {
				size = 4
			}
		}
		f, ok := odNewFormat(kind, size)
		if !ok {
			return nil, fmt.Errorf("invalid size %d in type string '%s'", size, spec)
		}
		if i < len(spec) && spec[i] == 'z' {
			f.dump = true
			i++
		}
		out = append(out, f)
	}
	return out, nil
}

func odAtoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
		if n > 1<<20 {
			return -1
		}
	}
	return n
}

// odSizeErr classifies a failure to parse a byte count.
type odSizeErr int

const (
	odSizeOK odSizeErr = iota
	odSizeInvalid
	odSizeSuffix
	odSizeTooLarge
)

func odSizeMessage(kind odSizeErr, optName, disp, val string) string {
	switch kind {
	case odSizeSuffix:
		return fmt.Sprintf("invalid suffix in %s argument '%s'", disp, val)
	case odSizeTooLarge:
		return fmt.Sprintf("%s argument '%s' too large", disp, val)
	}
	return fmt.Sprintf("invalid %s argument '%s'", disp, val)
}

// odParseByteCount parses -j/-N arguments: decimal, octal ("0…") or hex
// ("0x…"), each with an optional multiplier suffix.
func odParseByteCount(s string) (uint64, odSizeErr) {
	radix, start := 10, 0
	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		radix, start = 16, 2
	case strings.HasPrefix(s, "0"):
		radix, start = 8, 0
	}
	body := s[start:]
	mult := uint64(1)
	// A suffix is the longest tail that is not a digit of the radix.
	end := len(body)
	for end > 0 && !odDigitOfRadix(body[end-1], radix) {
		end--
	}
	suffix := body[end:]
	digits := body[:end]
	if digits == "" {
		return 0, odSizeInvalid
	}
	switch suffix {
	case "":
	case "b":
		if radix == 16 {
			return 0, odSizeSuffix
		}
		mult = 512
	case "K", "k":
		mult = 1 << 10
	case "m", "M":
		mult = 1 << 20
	case "G":
		mult = 1 << 30
	case "T":
		mult = 1 << 40
	case "P":
		mult = 1 << 50
	case "E":
		if radix == 16 {
			return 0, odSizeSuffix
		}
		mult = 1 << 60
	case "B":
		if radix == 16 {
			return 0, odSizeSuffix
		}
		mult = 1
	case "KB", "kB":
		mult = 1000
	case "MB", "mB":
		mult = 1000 * 1000
	case "GB":
		mult = 1000 * 1000 * 1000
	case "TB":
		mult = 1000 * 1000 * 1000 * 1000
	case "PB":
		mult = 1000 * 1000 * 1000 * 1000 * 1000
	case "EB":
		mult = 1000 * 1000 * 1000 * 1000 * 1000 * 1000
	case "Y", "Z", "R", "Q", "YB", "ZB", "RB", "QB":
		return 0, odSizeTooLarge
	default:
		return 0, odSizeSuffix
	}
	var n uint64
	for i := 0; i < len(digits); i++ {
		d := odDigitValue(digits[i])
		if d < 0 || d >= radix {
			return 0, odSizeInvalid
		}
		if n > (math.MaxUint64-uint64(d))/uint64(radix) {
			return 0, odSizeTooLarge
		}
		n = n*uint64(radix) + uint64(d)
	}
	if mult != 0 && n > math.MaxUint64/mult {
		return 0, odSizeTooLarge
	}
	return n * mult, odSizeOK
}

func odDigitOfRadix(c byte, radix int) bool {
	d := odDigitValue(c)
	return d >= 0 && d < radix
}

func odDigitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// odParseWidth parses -w: a plain decimal number, so a leading zero is not
// octal and no multiplier suffix is accepted.
func odParseWidth(s string) (uint64, odSizeErr) {
	if s == "" {
		return 0, odSizeInvalid
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			if i == 0 {
				return 0, odSizeInvalid
			}
			return 0, odSizeSuffix
		}
		d := uint64(s[i] - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, odSizeTooLarge
		}
		n = n*10 + d
	}
	return n, odSizeOK
}
