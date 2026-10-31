package builtins

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const truncateSpec = "s(size):c(no-create)r(reference):o(io-blocks)"

var truncateSpecParsed *Spec

func init() {
	var err error
	truncateSpecParsed, err = ParseSpec(truncateSpec)
	if err != nil {
		panic("builtins: bad truncate spec: " + err.Error())
	}
	Register(Command{Name: "truncate", Run: runTruncate})
}

func runTruncate(ctx *Context) int {
	flags, err := truncateSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "truncate:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, truncateSpecParsed) {
		return 0
	}

	sizeStr, hasSz := flags.String("s")
	refFile, hasRef := flags.String("r")
	ioBlocks := flags.Bool("o")

	// Check parameter combinations
	if !hasSz && !hasRef {
		fmt.Fprintln(ctx.Stderr, "truncate: error: the following required arguments were not provided:\n  --size <SIZE>\n  --reference <RFILE>")
		return 1
	}

	// Check if size is absolute (doesn't start with +, -, etc.)
	isAbsoluteSize := true
	if hasSz {
		trimmed := strings.TrimLeft(sizeStr, " \t")
		if len(trimmed) > 0 {
			switch trimmed[0] {
			case '+', '-', '<', '>', '/', '%':
				isAbsoluteSize = false
			}
		}
	}

	// Can't combine --size and --reference if size is absolute
	if hasSz && hasRef && isAbsoluteSize {
		fmt.Fprintln(ctx.Stderr, "truncate: cannot combine --size and --reference")
		return 1
	}
	if ioBlocks && !hasSz {
		fmt.Fprintln(ctx.Stderr, "truncate: --io-blocks requires --size")
		return 1
	}
	if len(flags.Operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "truncate: missing operand")
		return 1
	}

	var refSize int64
	if hasRef {
		rp := resolvePath(ctx, refFile)
		ri, err := os.Stat(rp)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "truncate: cannot stat '%s': %s\n", refFile, fmtOsErr(err))
			return 1
		}
		refSize = ri.Size()
	}

	noCreate := flags.Bool("c")
	exitCode := 0

	for _, name := range flags.Operands {
		path := resolvePath(ctx, name)

		info, statErr := os.Stat(path)
		if os.IsNotExist(statErr) {
			if noCreate {
				continue
			}
			f, err := os.Create(path)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "truncate: cannot open '%s' for writing: %s\n", name, fmtOsErr(err))
				exitCode = 1
				continue
			}
			f.Close()
			info, _ = os.Stat(path)
		} else if statErr != nil {
			fmt.Fprintf(ctx.Stderr, "truncate: cannot open '%s' for writing: %s\n", name, fmtOsErr(statErr))
			exitCode = 1
			continue
		}

		if info != nil && (info.Mode()&os.ModeNamedPipe != 0 || info.Mode()&os.ModeSocket != 0 || info.Mode()&os.ModeDevice != 0) {
			fmt.Fprintf(ctx.Stderr, "truncate: cannot open '%s' for writing: No such device or address\n", name)
			exitCode = 1
			continue
		}

		var size int64
		if hasRef && !hasSz {
			size = refSize
		} else {
			base := info.Size()
			if hasRef {
				base = refSize
			}
			var err error
			size, err = parseTruncateSize(sizeStr, base)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "truncate: %v\n", err)
				exitCode = 1
				continue
			}
			if ioBlocks {
				blkSize := int64(512)
				if st, ok := readFSMeta(path, info); ok && st.Blksize > 0 {
					blkSize = int64(st.Blksize)
				}
				// Check for overflow in multiplication
				if size > 0 && blkSize > 0 && size > math.MaxInt64/blkSize {
					fmt.Fprintf(ctx.Stderr, "truncate: Invalid number: Value too large for defined data type\n")
					exitCode = 1
					continue
				}
				size *= blkSize
			}
		}
		if size < 0 {
			size = 0
		}

		if err := os.Truncate(path, size); err != nil {
			fmt.Fprintf(ctx.Stderr, "truncate: cannot open '%s' for writing: %s\n", name, fmtOsErr(err))
			exitCode = 1
		}
	}
	return exitCode
}

func parseTruncateSize(rawSize string, current int64) (int64, error) {
	s := strings.TrimLeft(rawSize, " \t")

	mode := byte(0)
	if len(s) > 0 {
		switch s[0] {
		case '<', '>', '/', '%':
			mode = s[0]
			s = s[1:]
		}
	}

	// Trim spaces after mode
	s = strings.TrimLeft(s, " \t")

	relative := byte(0)
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		relative = s[0]
		s = s[1:]
	}

	// Check for mixing relative modifiers with mode modifiers
	if mode != 0 && relative != 0 {
		return 0, fmt.Errorf("Invalid number: '%s'", rawSize)
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("Invalid number: '%s'", rawSize)
	}

	numStr, multiplier := parseSizeSuffix(s)
	suffix := s[len(numStr):]

	if strings.EqualFold(suffix, "b") && multiplier == 512 {
		return 0, fmt.Errorf("Invalid number: '%s'", rawSize)
	}

	if numStr == s {
		for i := len(s) - 1; i >= 0; i-- {
			if s[i] >= '0' && s[i] <= '9' {
				if i < len(s)-1 {
					return 0, fmt.Errorf("Invalid number: '%s': Value too large for defined data type", rawSize)
				}
				break
			}
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("Invalid number: '%s'", rawSize)
	}

	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		ne, ok := err.(*strconv.NumError)
		if ok && ne.Err == strconv.ErrRange {
			return 0, fmt.Errorf("Invalid number: '%s': Value too large for defined data type", rawSize)
		}
		return 0, fmt.Errorf("Invalid number: '%s'", rawSize)
	}
	if multiplier == math.MaxInt64 || (multiplier > 1 && n > 0 && n > math.MaxInt64/multiplier) {
		return 0, fmt.Errorf("Invalid number: '%s': Value too large for defined data type", rawSize)
	}
	n *= multiplier

	switch relative {
	case '+':
		// Check for overflow
		if n > 0 && current > math.MaxInt64-n {
			return 0, fmt.Errorf("Invalid number: '%s': Value too large for defined data type", rawSize)
		}
		n = current + n
	case '-':
		// Check for underflow
		if n > 0 && current < math.MinInt64+n {
			return 0, fmt.Errorf("Invalid number: '%s': Value too large for defined data type", rawSize)
		}
		n = current - n
	}

	switch mode {
	case '<':
		if current < n {
			return current, nil
		}
		return n, nil
	case '>':
		if current > n {
			return current, nil
		}
		return n, nil
	case '/':
		if n <= 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return (current / n) * n, nil
	case '%':
		if n <= 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return ((current + n - 1) / n) * n, nil
	default:
		return n, nil
	}
}
