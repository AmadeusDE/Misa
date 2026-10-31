// The summary lines dd prints on standard error.
//
// The wording and the byte magnitudes follow uutils coreutils dd
// (src/uu/dd/src/progress.rs and numbers.rs, MIT) at the commit pinned in
// docs/CREDITS.md: "N+M records in", "N+M records out" and
// "X bytes (1.2 MB, 1.1 MiB) copied, T s, R/s".
package builtins

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"time"
)

var ddSIBase = [...]uint64{1, 1e3, 1e6, 1e9, 1e12, 1e15, 1e18}
var ddIECBase = [...]uint64{1, 1 << 10, 1 << 20, 1 << 30, 1 << 40, 1 << 50, 1 << 60}

var ddSISuffix = [...]string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}
var ddIECSuffix = [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

func ddPrintSummary(w io.Writer, stats ddStats, elapsed time.Duration, status ddStatus) {
	if status.records() {
		fmt.Fprintf(w, "%d+%d records in\n%d+%d records out\n", stats.inFull, stats.inPart, stats.outFull, stats.outPart)
		if stats.truncated > 0 {
			fmt.Fprintf(w, "%d truncated %s\n", stats.truncated, ddPlural(stats.truncated, "record", "records"))
		}
	}
	if !status.xfer() {
		return
	}
	fmt.Fprintf(w, "%s\n", ddTransferLine(stats, elapsed))
}

// ddTransferLine is the "bytes copied, seconds, rate" line: the summary's last
// line and, without its newline, the status=progress line.
func ddTransferLine(stats ddStats, elapsed time.Duration) string {
	return fmt.Sprintf("%s, %s s, %s/s",
		ddBytesCopied(stats.bytes),
		strconv.FormatFloat(elapsed.Seconds(), 'g', -1, 64),
		ddMagnitude(ddRate(stats.bytes, elapsed), true))
}

// ddPlural picks the singular or plural word for the summary lines.
func ddPlural(n uint64, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ddBytesCopied is the "N bytes copied" part of the summary line, with the
// decimal magnitude added once it says more than the byte count itself.
func ddBytesCopied(n uint64) string {
	switch {
	case n == 1:
		return "1 byte copied"
	case n < 1000:
		return fmt.Sprintf("%d bytes copied", n)
	case n < 1024:
		return fmt.Sprintf("%d bytes (%s) copied", n, ddMagnitude(n, true))
	default:
		return fmt.Sprintf("%d bytes (%s, %s) copied", n, ddMagnitude(n, true), ddMagnitude(n, false))
	}
}

// ddRate is the throughput in bytes per second, never dividing by zero.
func ddRate(bytes uint64, elapsed time.Duration) uint64 {
	millis := uint64(max(1, elapsed.Milliseconds()))
	return 1000 * bytes / millis
}

// ddMagnitude renders n as at most five characters, such as "999 B", "1.0 kB"
// or "34 GiB", in decimal (si) or binary units.
func ddMagnitude(n uint64, si bool) string {
	bases, suffixes := ddIECBase[:], ddIECSuffix[:]
	if si {
		bases, suffixes = ddSIBase[:], ddSISuffix[:]
	}
	i := 0
	for i+1 < len(bases) && bases[i+1]-bases[i] < n {
		i++
	}
	quotient := float64(n) / float64(bases[i])
	if quotient < 10 {
		return fmt.Sprintf("%.1f %s", quotient, suffixes[i])
	}
	return fmt.Sprintf("%.0f %s", math.Round(quotient), suffixes[i])
}
