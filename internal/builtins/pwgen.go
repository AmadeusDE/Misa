// Behaviour adapted from toybox pwgen.c at commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package builtins

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"strconv"
)

const pwgenSpec = "0(no-numerals)A(no-capitalize)B(ambiguous)Cc(capitalize)h(help)n(numerals)r(remove):s(secure)v1y(symbols)"

const (
	pwgenDefaultLength = 8
	pwgenMaxLength     = 4096
	// pwgenAmbiguous is toybox's list of characters that are easy to confuse.
	pwgenAmbiguous = "0O1lI8B5S2ZD'`.,"
)

// pwgenUsage is printed for -h/--help. It follows toybox's wording.
const pwgenUsage = `usage: pwgen [-cAn0yrsBC1v] [-r CHARS] [LENGTH] [COUNT]

Generate LENGTH (default 8) passwords of COUNT (default 1, or as many as fit
on the screen if standard output is a terminal).

  -c  Capitalize (capitals are always included, so this is a no-op)
  -A  No capital letters
  -n  Numerals (numerals are always included, so this is a no-op)
  -0  No numerals
  -y  Symbols (allow the whole printable range, not just [0-9A-Za-z])
  -r  Do not use any of the characters in CHARS
  -s  Secure (always on: passwords come from the crypto/rand generator)
  -B  No ambiguous characters: 0O1lI8B5S2ZD'` + "`" + `.,
  -v  Avoid vowels
  -1  One password per line
  -C  Print in columns (the default when stdout is a terminal)
  -h  This help text

-c conflicts with -A, -n conflicts with -0, and -C conflicts with -1.
Characters are picked uniformly by rejection sampling, so the passwords have
no modulo bias.
`

var pwgenSpecParsed *Spec

// pwgenRand is the randomness source; tests swap it out.
var pwgenRand io.Reader = rand.Reader

func init() {
	var err error
	pwgenSpecParsed, err = ParseSpec(pwgenSpec)
	if err != nil {
		panic("builtins: bad pwgen spec: " + err.Error())
	}
	Register(Command{Name: "pwgen", Run: runPwgen})
}

func runPwgen(ctx *Context) int {
	flags, err := pwgenSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "pwgen:", err)
		return 1
	}
	if flags.HelpRequested || flags.Bool("h") {
		fmt.Fprint(ctx.Stdout, pwgenUsage)
		return 0
	}
	if flags.VersionRequested {
		name := ctx.CmdName
		if name == "" {
			name = "pwgen"
		}
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", name)
		return 0
	}

	for _, ex := range [][2]string{{"c", "A"}, {"n", "0"}, {"C", "1"}} {
		if flags.Bool(ex[0]) && flags.Bool(ex[1]) {
			fmt.Fprintf(ctx.Stderr, "pwgen: -%s and -%s are mutually exclusive\n", ex[0], ex[1])
			return 1
		}
	}

	ops := flags.Operands
	if len(ops) > 2 {
		fmt.Fprintln(ctx.Stderr, "pwgen: too many arguments")
		return 1
	}

	length := pwgenDefaultLength
	if len(ops) > 0 {
		n, ok := pwgenDecimal(ops[0])
		if !ok || n < 1 || n > pwgenMaxLength {
			fmt.Fprintln(ctx.Stderr, "pwgen: bad length")
			return 1
		}
		length = n
	}

	tty := isStdoutTTY(ctx)
	count := 1
	if len(ops) > 1 {
		n, ok := pwgenDecimal(ops[1])
		if !ok || n < 0 {
			fmt.Fprintln(ctx.Stderr, "pwgen: bad count")
			return 1
		}
		count = n
	} else if tty {
		// Fill the screen the way toybox does.
		w, h := ctx.Width, ctx.Height
		if w <= 0 {
			w = 80
		}
		if h <= 0 {
			h = 24
		}
		if n := w / (length + 1) * (h - 1); n > 0 {
			count = n
		}
	}

	allowed := pwgenAllowed(flags)
	if len(allowed) == 0 {
		fmt.Fprintln(ctx.Stderr, "pwgen: no characters available")
		return 1
	}

	onePerLine := flags.Bool("1") || !(flags.Bool("C") || tty)

	width := ctx.Width
	if width <= 0 {
		width = 80
	}

	src := bufio.NewReaderSize(pwgenRand, 256)
	w := bufio.NewWriter(ctx.Stdout)
	col := 0
	for i := 0; i < count; i++ {
		for j := 0; j < length; j++ {
			c, err := pwgenPick(src, allowed)
			if err != nil {
				w.Flush()
				fmt.Fprintln(ctx.Stderr, "pwgen: cannot read random bytes")
				return 1
			}
			w.WriteByte(c)
		}
		col += length
		switch {
		case onePerLine, i+1 == count, col+length >= width:
			w.WriteByte('\n')
			col = 0
		default:
			w.WriteByte(' ')
			col++
		}
	}
	w.Flush()
	return 0
}

// pwgenDecimal parses a non-negative decimal operand. Anything else (signs,
// spaces, trailing junk, overflow) is reported as not ok.
func pwgenDecimal(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// pwgenAllowed builds the set of characters a password may use, in a fixed
// order so the same random bytes always produce the same password.
func pwgenAllowed(flags *ParseResult) []byte {
	var set []byte
	if flags.Bool("y") {
		for c := byte(33); c <= 126; c++ {
			set = append(set, c)
		}
	} else {
		for c := byte('0'); c <= '9'; c++ {
			set = append(set, c)
		}
		for c := byte('A'); c <= 'Z'; c++ {
			set = append(set, c)
		}
		for c := byte('a'); c <= 'z'; c++ {
			set = append(set, c)
		}
	}

	var drop [256]bool
	dropRange := func(from, to byte) {
		for c := from; c <= to; c++ {
			drop[c] = true
		}
	}
	dropBytes := func(s string) {
		for i := 0; i < len(s); i++ {
			drop[s[i]] = true
		}
	}
	if flags.Bool("A") {
		dropRange('A', 'Z')
	}
	if flags.Bool("0") {
		dropRange('0', '9')
	}
	if flags.Bool("v") {
		dropBytes("aeiouAEIOU")
	}
	if flags.Bool("B") {
		dropBytes(pwgenAmbiguous)
	}
	for _, rm := range flags.AllStrings("r") {
		dropBytes(rm)
	}

	out := set[:0]
	for _, c := range set {
		if !drop[c] {
			out = append(out, c)
		}
	}
	return out
}

// pwgenPick returns a uniformly distributed character of allowed by rejection
// sampling: bytes that would make the modulo range uneven are thrown away.
func pwgenPick(src *bufio.Reader, allowed []byte) (byte, error) {
	n := len(allowed)
	limit := 256 - 256%n
	for {
		b, err := src.ReadByte()
		if err != nil {
			return 0, err
		}
		if int(b) < limit {
			return allowed[int(b)%n], nil
		}
	}
}
