// Block sizes, units and percentages for df. The human-readable columns port
// toybox's human_readable and human_readable_long from lib/lib.c (0BSD, toybox
// commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb); the -B column labels port
// to_magnitude_and_suffix and BlockSize::to_header from uutils coreutils
// src/uu/df/src/blocks.rs (MIT, commit fee61ff5d4f87e0f912a52e639942b5f7ad58663),
// which is why `-B 1024` prints "1K-blocks" but `-B 1000` prints "kB".
package builtins

import (
	"fmt"
	"strconv"
)

// blockUnit picks the divisor Toybox uses: human-readable and inode modes
// count bytes or files one by one, an explicit -B wins over the POSIX default,
// and -P falls back to 512-byte blocks.
func (o *dfOptions) blockUnit() uint64 {
	switch {
	case o.human || o.si || o.inodes:
		return 1
	case o.blockSize > 0:
		return uint64(o.blockSize)
	case o.portability && !o.kilo:
		return 512
	default:
		return 1024
	}
}

// sizeLabel names the block-size column: Toybox's 1K/1024/512 wording for the
// implicit sizes, uutils' magnitude labels for -B (numeric with -P, as POSIX
// wants).
func (o *dfOptions) sizeLabel() string {
	if o.blockSize > 0 {
		if o.portability {
			return strconv.FormatInt(o.blockSize, 10) + "-blocks"
		}
		return dfMagnitudeSuffix(uint64(o.blockSize)) + "-blocks"
	}
	switch {
	case o.unit == 512:
		return "512-blocks"
	case o.portability:
		return "1024-blocks"
	default:
		return "1K-blocks"
	}
}

// dfSub is used-space subtraction that cannot wrap when a filesystem reports
// more free blocks than it has.
func dfSub(total, free uint64) uint64 {
	if free > total {
		return 0
	}
	return total - free
}

// dfPercent is the rounded-up percentage of used space out of used plus
// available, or 0% for a filesystem with neither.
func dfPercent(used, avail uint64) uint64 {
	sum := used + avail
	if sum == 0 {
		return 0
	}
	p := used * 100 / sum
	if used*100 != p*sum {
		p++
	}
	return p
}

// humanReadable ports toybox's human_readable_long(dgt=3, unit=0): divide with
// rounding (n + divisor/2) / divisor, stepping through the units until at most
// three digits remain, then print one decimal place when a single digit is
// left. -h divides by 1024 and uppercases the unit, -H divides by 1000.
func humanReadable(n uint64, si bool) string {
	const units = " kMGTPEZY"
	divisor := uint64(1024)
	if si {
		divisor = 1000
	}
	var snap uint64
	unit := 0
	for len(strconv.FormatUint(n, 10)) > 3 {
		snap = n
		n = (n + divisor/2) / divisor
		unit++
	}
	out := strconv.FormatUint(n, 10)
	if unit > 0 {
		if len(out) == 1 {
			// Toybox redoes the rounding to keep "1.2M" instead of "1M".
			num := snap / divisor
			snap -= num * divisor
			snap = ((snap * 100) + 50) / divisor
			out = fmt.Sprintf("%d.%d", num, snap/10)
		}
		u := units[unit]
		if !si && u >= 'a' && u <= 'z' {
			u -= 'a' - 'A'
		}
		out += string(u)
	}
	return out
}

// dfMagnitudeSuffix ports uutils' BlockSize::to_header, i.e.
// to_magnitude_and_suffix(n, iec-or-si, add_tracing_zero=false): a multiple of
// 1024 that is not a multiple of 1000 counts in IEC units ("1K", "4K", "1M"),
// everything else in SI units ("1kB", "128kB", "1.1MB"), and a count below the
// first unit keeps its byte suffix ("111B").
func dfMagnitudeSuffix(n uint64) string {
	base := uint64(1024)
	suffixes := [8]string{"K", "M", "G", "T", "P", "E", "Z", "Y"}
	if n%1024 != 0 || n%1000 == 0 {
		base = 1000
		suffixes = [8]string{"kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	}
	pow, unit := uint64(1), 0
	for unit < len(suffixes) && pow <= (n-1)/(base-1) {
		pow *= base
		unit++
	}
	quot, rem := n/pow, n%pow
	suffix := "B"
	if unit > 0 {
		suffix = suffixes[unit-1]
	}
	if rem == 0 {
		return strconv.FormatUint(quot, 10) + suffix
	}
	tenths := rem / (pow / 10)
	switch {
	case quot >= 100 || tenths+1 == 10 || quot >= 10:
		return strconv.FormatUint(quot+1, 10) + suffix
	case rem%(pow/10) == 0:
		return fmt.Sprintf("%d.%d%s", quot, tenths, suffix)
	default:
		return fmt.Sprintf("%d.%d%s", quot, tenths+1, suffix)
	}
}
