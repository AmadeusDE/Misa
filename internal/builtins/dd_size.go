// dd size suffixes and x products.
//
// The suffixes follow toybox dd (toys/posix/dd.c, 0BSD) for c/w/b/k/M/G/T/P/E
// and uutils coreutils (src/uu/dd/src/parseargs.rs, MIT) for the kB/KiB forms;
// both at the commits pinned in docs/CREDITS.md.
package builtins

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// ddParseNum parses a count=/skip=/seek= value, remembering whether it named
// bytes ("3B", "2KiB").
func ddParseNum(value string) (ddNumber, error) {
	n, err := ddParseNumber(value)
	if err != nil {
		return ddNumber{}, err
	}
	return ddNumber{value: n, bytes: strings.Contains(value, "B")}, nil
}

// ddParseNumber parses a size the way dd's references do: a decimal or 0x
// hexadecimal number, an optional unit suffix, and "x" products such as 2x512.
func ddParseNumber(value string) (uint64, error) {
	if value == "" {
		return 0, ddInvalidNumber(value)
	}
	total := uint64(1)
	for _, part := range ddSplitProduct(value) {
		factor, err := ddParseFactor(part)
		if errors.Is(err, errDDTooLarge) {
			return 0, errDDTooLarge
		}
		if err != nil {
			return 0, ddInvalidNumber(value)
		}
		product, ok := ddMul(total, factor)
		if !ok {
			return 0, ddInvalidNumber(value)
		}
		total = product
	}
	if total >= math.MaxInt64 {
		return 0, errDDTooLarge
	}
	return total, nil
}

// ddSplitProduct splits "2x512" into its factors. An "x" that belongs to a 0x
// hexadecimal prefix is part of the number, as in toybox's argxarg().
func ddSplitProduct(value string) []string {
	var parts []string
	for {
		from := 0
		if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
			from = 2
		}
		i := strings.IndexByte(value[from:], 'x')
		if i < 0 {
			return append(parts, value)
		}
		parts = append(parts, value[:from+i])
		value = value[from+i+1:]
	}
}

func ddParseFactor(part string) (uint64, error) {
	digits, base := 0, 10
	if strings.HasPrefix(part, "0x") || strings.HasPrefix(part, "0X") {
		base = 0
		for digits = 2; digits < len(part) && isHexDigit(part[digits]); digits++ {
		}
		if digits == 2 {
			return 0, ddInvalidNumber(part)
		}
	} else {
		for digits = 0; digits < len(part) && part[digits] >= '0' && part[digits] <= '9'; digits++ {
		}
		if digits == 0 {
			return 0, ddInvalidNumber(part)
		}
	}
	number, err := strconv.ParseUint(part[:digits], base, 64)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
			return 0, errDDTooLarge
		}
		return 0, ddInvalidNumber(part)
	}
	multiplier, ok := ddSuffix(part[digits:])
	if !ok {
		return 0, ddInvalidNumber(part)
	}
	product, ok := ddMul(number, multiplier)
	if !ok {
		return 0, ddInvalidNumber(part)
	}
	return product, nil
}

// ddSuffix maps a unit suffix to its multiplier. Lowercase b is 512 bytes and
// capital B is one byte, as in the references; "kB"/"MB" are powers of 1000
// and "KiB"/"MiB" powers of 1024.
func ddSuffix(suffix string) (uint64, bool) {
	switch suffix {
	case "":
		return 1, true
	case "c", "C":
		return 1, true
	case "w", "W":
		return 2, true
	case "b":
		return 512, true
	case "B":
		return 1, true
	case "k", "K":
		return 1 << 10, true
	case "M":
		return 1 << 20, true
	case "G":
		return 1 << 30, true
	case "T":
		return 1 << 40, true
	case "P":
		return 1 << 50, true
	case "E":
		return 1 << 60, true
	}
	switch strings.ToLower(suffix) {
	case "kb":
		return 1000, true
	case "kib":
		return 1 << 10, true
	case "mb":
		return 1000 * 1000, true
	case "mib":
		return 1 << 20, true
	case "gb":
		return 1000 * 1000 * 1000, true
	case "gib":
		return 1 << 30, true
	case "tb":
		return 1000 * 1000 * 1000 * 1000, true
	case "tib":
		return 1 << 40, true
	case "pb":
		return 1000 * 1000 * 1000 * 1000 * 1000, true
	case "pib":
		return 1 << 50, true
	case "eb":
		return 1000 * 1000 * 1000 * 1000 * 1000 * 1000, true
	case "eib":
		return 1 << 60, true
	}
	return 0, false
}

func ddMul(a, b uint64) (uint64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if a > math.MaxUint64/b {
		return 0, false
	}
	return a * b, true
}
