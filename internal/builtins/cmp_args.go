package builtins

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// cmpParseArgs parses cmp command line options following the operand
// permutation rules of uutils diffutils cmp (parse_params in
// https://github.com/uutils/diffutils, src/cmp.rs, MIT licensed).
// Unlike the spec-based ParseArgs, -i/--ignore-initial consumes a
// colon-separated operand and earlier positional operands win over
// later ones, which the generic option parser cannot express.
type cmpParams struct {
	exe        string
	printBytes bool
	verbose    bool
	quiet      bool
	help       bool
	version    bool
	skipA      uint64
	skipB      uint64
	skipASet   bool
	skipBSet   bool
	maxBytes   *uint64
	from       string
	to         string
}

func cmpParseArgs(exe string, args []string) (cmpParams, error) {
	p := cmpParams{exe: exe}
	var posSkipA, posSkipB uint64
	var havePosSkipA, havePosSkipB bool

	positional := func(tok string, hasFrom, hasTo *bool) error {
		if !*hasFrom {
			p.from, *hasFrom = tok, true
			return nil
		}
		if !*hasTo {
			p.to, *hasTo = tok, true
			return nil
		}
		if !havePosSkipA {
			n, ok := cmpParseSkip(tok)
			if !ok {
				return fmt.Errorf("%s: invalid --ignore-initial value '%s'", exe, tok)
			}
			posSkipA, havePosSkipA = n, true
			return nil
		}
		if !havePosSkipB {
			n, ok := cmpParseSkip(tok)
			if !ok {
				return fmt.Errorf("%s: invalid --ignore-initial value '%s'", exe, tok)
			}
			posSkipB, havePosSkipB = n, true
			return nil
		}
		return fmt.Errorf("Usage: %s <from> <to>", exe)
	}

	hasFrom, hasTo := false, false
	i := 0
	for ; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			if err := positional(a, &hasFrom, &hasTo); err != nil {
				return p, err
			}
			continue
		}
		switch a {
		case "-b", "--print-bytes":
			p.printBytes = true
			continue
		case "-l", "--verbose":
			p.verbose = true
			continue
		case "-lb", "-bl":
			p.printBytes = true
			p.verbose = true
			continue
		case "-s", "--quiet", "--silent":
			p.quiet = true
			continue
		case "-h", "--help":
			p.help = true
			return p, nil
		case "--version":
			p.version = true
			return p, nil
		case "-n", "--bytes":
			if i+1 >= len(args) {
				return p, fmt.Errorf("Usage: %s <from> <to>", exe)
			}
			i++
			n, ok := cmpParseMaxBytes(args[i])
			if !ok {
				return p, fmt.Errorf("%s: invalid --bytes value '%s'", exe, args[i])
			}
			p.maxBytes = &n
			continue
		case "-i", "--ignore-initial":
			if i+1 >= len(args) {
				return p, fmt.Errorf("Usage: %s <from> <to>", exe)
			}
			i++
			desc := args[i]
			sa, sb, ok := cmpParseIgnoreInitial(desc)
			if !ok {
				return p, fmt.Errorf("%s: invalid --ignore-initial value '%s'", exe, desc)
			}
			p.skipA, p.skipB, p.skipASet, p.skipBSet = sa, sb, true, true
			continue
		}
		if strings.HasPrefix(a, "--bytes=") {
			val := strings.TrimPrefix(a, "--bytes=")
			n, ok := cmpParseMaxBytes(val)
			if !ok {
				return p, fmt.Errorf("%s: invalid --bytes value '%s'", exe, val)
			}
			p.maxBytes = &n
			continue
		}
		if strings.HasPrefix(a, "--ignore-initial=") {
			desc := strings.TrimPrefix(a, "--ignore-initial=")
			sa, sb, ok := cmpParseIgnoreInitial(desc)
			if !ok {
				return p, fmt.Errorf("%s: invalid --ignore-initial value '%s'", exe, desc)
			}
			p.skipA, p.skipB, p.skipASet, p.skipBSet = sa, sb, true, true
			continue
		}
		return p, fmt.Errorf("unrecognized option '%s'", a)
	}
	rest := args[i:]

	if p.quiet && p.verbose {
		return p, fmt.Errorf("%s: options -l and -s are incompatible", exe)
	}
	if !hasFrom {
		if len(rest) == 0 {
			return p, fmt.Errorf("Usage: %s <from> <to>", exe)
		}
		p.from, hasFrom = rest[0], true
		rest = rest[1:]
	}
	if !hasTo {
		if len(rest) > 0 {
			p.to, hasTo = rest[0], true
			rest = rest[1:]
		} else {
			p.to = "-"
		}
	}
	if !p.skipASet {
		if havePosSkipA {
			p.skipA, p.skipASet = posSkipA, true
		} else if len(rest) > 0 {
			n, ok := cmpParseSkip(rest[0])
			if !ok {
				return p, fmt.Errorf("%s: invalid --ignore-initial value '%s'", exe, rest[0])
			}
			p.skipA, p.skipASet = n, true
			rest = rest[1:]
		}
	}
	if !p.skipBSet {
		if havePosSkipB {
			p.skipB, p.skipBSet = posSkipB, true
		} else if len(rest) > 0 {
			n, ok := cmpParseSkip(rest[0])
			if !ok {
				return p, fmt.Errorf("%s: invalid --ignore-initial value '%s'", exe, rest[0])
			}
			p.skipB, p.skipBSet = n, true
			rest = rest[1:]
		}
	}
	return p, nil
}

// cmpParseMaxBytes parses -n/--bytes values like Rust's u64::from_str:
// out-of-range saturates to math.MaxUint64, other errors are rejected.
func cmpParseMaxBytes(s string) (uint64, bool) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err == nil {
		return n, true
	}
	if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
		return math.MaxUint64, true
	}
	return 0, false
}

// cmpParseIgnoreInitial splits an -i/--ignore-initial value at the first
// colon; without a colon both skip values share the parsed number.
func cmpParseIgnoreInitial(desc string) (uint64, uint64, bool) {
	a, b := desc, desc
	if idx := strings.IndexByte(desc, ':'); idx >= 0 {
		a, b = desc[:idx], desc[idx+1:]
	}
	skipA, ok := cmpParseSkip(a)
	if !ok {
		return 0, 0, false
	}
	skipB, ok := cmpParseSkip(b)
	if !ok {
		return 0, 0, false
	}
	return skipA, skipB, true
}

// cmpSkipMultipliers holds the byte-count suffixes accepted by -i values:
// B-suffixed names are decimal (powers of 1000), bare names binary
// (powers of 1024), and Z/Y saturate like uutils. Saturating arithmetic
// is used for suffixes whose value overflows u64.
var cmpSkipMultipliers = map[string]uint64{
	"kB": 1000,
	"K":  1024,
	"MB": 1000 * 1000,
	"M":  1024 * 1024,
	"GB": 1000 * 1000 * 1000,
	"G":  1024 * 1024 * 1024,
	"TB": 1000 * 1000 * 1000 * 1000,
	"T":  1024 * 1024 * 1024 * 1024,
	"PB": 1000 * 1000 * 1000 * 1000 * 1000,
	"P":  1024 * 1024 * 1024 * 1024 * 1024,
	"EB": 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	"E":  1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	"Z":  math.MaxUint64,
	"ZB": math.MaxUint64,
	"Y":  math.MaxUint64,
	"YB": math.MaxUint64,
}

// cmpParseSkip parses an unsigned decimal byte count with an optional
// magnitude suffix, following parse_skip in uutils diffutils cmp.
func cmpParseSkip(param string) (uint64, bool) {
	digits := 0
	for digits < len(param) && param[digits] >= '0' && param[digits] <= '9' {
		digits++
	}
	num, ok := cmpParseMaxBytes(param[:digits])
	if !ok {
		return 0, false
	}
	suffix := param[digits:]
	if suffix == "" {
		return num, true
	}
	mult, ok := cmpSkipMultipliers[suffix]
	if !ok {
		return 0, false
	}
	if num > math.MaxUint64/mult {
		return math.MaxUint64, true
	}
	return num * mult, true
}
