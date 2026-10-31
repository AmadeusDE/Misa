// free — display the amount of free and used memory in the system.
//
// The layout (a header plus Mem:/Swap:/Total: rows), the option set
// (-b -k -m -g -h --si -t -w -s -c), the human-readable rounding rules and the
// usage-error wording follow uutils procps src/uu/free/src/free.rs (MIT, commit
// e697667b3f758803e38d8ab4c8f912171beca602); see docs/CREDITS.md. The numbers
// come from internal/proc (Linux/Android procfs); elsewhere free reports the
// platform as unsupported instead of inventing figures. Deliberately
// unsupported: -l/--lohi, -v/--committed, -L/--line and the --kilo/--mega/…
// SI unit spellings (--kibi/--mebi/--gibi are the long forms of -k/-m/-g).
// This is a native Go implementation.

package builtins

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

const freeSpec = "b(bytes)k(kibi)m(mebi)g(gibi)h(human)t(total)w(wide)s(seconds):c(count)#(si)"

var freeSpecParsed *Spec

func init() {
	var err error
	freeSpecParsed, err = ParseSpec(freeSpec)
	if err != nil {
		panic("builtins: bad free spec: " + err.Error())
	}
	Register(Command{Name: "free", Run: runFree})
}

// freeProcRoot is the procfs root; tests point it at a fixture tree.
var freeProcRoot = ""

// freeUnit is the unit free prints memory in. level is the power of the base
// the byte count is divided by: 1 for -k, 2 for -m, 3 for -g.
type freeUnit struct {
	bytes bool
	level int
	si    bool
}

// convert turns a kibibyte count from /proc/meminfo into the output unit.
func (u freeUnit) convert(kib uint64) uint64 {
	bytes := kib * 1024
	if u.bytes {
		return bytes
	}
	div := uint64(1)
	for i := 0; i < u.level; i++ {
		if u.si {
			div *= 1000
		} else {
			div *= 1024
		}
	}
	return bytes / div
}

func runFree(ctx *Context) int {
	flags, err := freeSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "free: %v\nTry 'free --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, freeUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", ctx.CmdName)
		return 0
	}
	if len(flags.Operands) > 0 {
		fmt.Fprintf(ctx.Stderr, "free: extra operand '%s'\nTry 'free --help' for more information.\n", flags.Operands[0])
		return 1
	}
	unit, err := freeParseUnit(flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "free: %v\nTry 'free --help' for more information.\n", err)
		return 1
	}
	count, delay, err := freeParseSchedule(flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "free: %v\nTry 'free --help' for more information.\n", err)
		return 1
	}

	reader := proc.New(freeProcRoot)
	cancel := ctx.CancellationContext()
	human, wide, total := flags.Bool("h"), flags.Bool("w"), flags.Bool("t")
	for sample := 0; ; sample++ {
		if sample > 0 {
			fmt.Fprintln(ctx.Stdout)
		}
		mem, err := reader.Meminfo()
		if err != nil {
			printProcError(ctx, "free", "memory information", err)
			return 1
		}
		fmt.Fprint(ctx.Stdout, freeReport(mem, unit, human, wide, total))
		if count >= 0 && sample+1 >= count {
			break
		}
		if err := sampleSleep(cancel, delay); err != nil {
			return ExitStatus(err)
		}
	}
	return 0
}

func freeUsage() string {
	return "Usage: free [-b|-k|-m|-g|-h] [--si] [-t] [-w] [-s SECONDS] [-c COUNT]\n\n" +
		"Display the total, used and free amount of physical memory and swap.\n" +
		"Without a unit flag the numbers are kibibytes.\n\n" +
		"Units:\n" +
		"  -b, --bytes       bytes\n" +
		"  -k, --kibi        kibibytes (default)\n" +
		"  -m, --mebi        mebibytes\n" +
		"  -g, --gibi        gibibytes\n" +
		"  -h, --human       human readable (1024-based: 1.5Gi)\n" +
		"      --si          powers of 1000 instead of 1024\n\n" +
		"Output:\n" +
		"  -t, --total       add a Total: row for RAM plus swap\n" +
		"  -w, --wide        split buff/cache into buffers and cache\n\n" +
		"Repeat:\n" +
		"  -s, --seconds N   sample every N seconds (default 1); without -c it\n" +
		"                    repeats until interrupted\n" +
		"  -c, --count N     print N samples, then exit\n\n" +
		"--kibi, --mebi and --gibi are the long forms of -k, -m and -g.\n\n" +
		"Unsupported (rejected, never faked): -l/--lohi, -v/--committed,\n" +
		"-L/--line, --kilo/--mega/--giga/--tera/--peta. Exit status: 0 printed,\n" +
		"1 bad usage or unreadable data.\n"
}

// freeParseUnit reads the unit flags. They are mutually exclusive, as in the
// references: -b and -m together is a usage error, not a last-one-wins guess.
func freeParseUnit(flags *ParseResult) (freeUnit, error) {
	unit := freeUnit{level: 1, si: flags.Bool("si")}
	seen := ""
	for _, name := range []string{"b", "k", "m", "g"} {
		if !flags.Bool(name) {
			continue
		}
		if seen != "" {
			return freeUnit{}, fmt.Errorf("-%s and -%s are mutually exclusive", seen, name)
		}
		seen = name
		switch name {
		case "b":
			unit.bytes = true
		case "m":
			unit.level = 2
		case "g":
			unit.level = 3
		}
	}
	return unit, nil
}

// freeParseSchedule returns the sample count (-1 for "until cancelled") and the
// interval between samples. As in the reference, -s alone repeats forever,
// -c alone repeats once a second, and no flag at all prints one sample.
func freeParseSchedule(flags *ParseResult) (count int, delay time.Duration, err error) {
	count = 1
	if s, ok := flags.String("s"); ok {
		delay, err = parseSampleSeconds(s)
		if err != nil {
			return 0, 0, err
		}
		if _, explicit := flags.Num("c"); !explicit {
			count = -1
		}
	}
	if n, ok := flags.Num("c"); ok {
		if n <= 0 {
			return 0, 0, fmt.Errorf("count argument must be greater than 0")
		}
		count = int(n)
	}
	if delay == 0 {
		delay = time.Second
	}
	return count, delay, nil
}

// freeReport renders one sample: the header, the Mem: row, the Swap: row and,
// with -t, the Total: row.
func freeReport(mem proc.Meminfo, unit freeUnit, human, wide, total bool) string {
	value := func(kib uint64) string {
		if human {
			return freeHumanize(kib, unit.si)
		}
		return strconv.FormatUint(unit.convert(kib), 10)
	}
	available := mem.Available()
	used := subSat(mem.MemTotal, available)
	cache := addSat(mem.Cached, mem.SReclaimable)

	var b strings.Builder
	if wide {
		fmt.Fprintf(&b, "%-8s%12s%12s%12s%12s%12s%12s%12s\n",
			" ", "total", "used", "free", "shared", "buffers", "cache", "available")
		fmt.Fprintf(&b, "%-8s%12s%12s%12s%12s%12s%12s%12s\n",
			"Mem:", value(mem.MemTotal), value(used), value(mem.MemFree), value(mem.Shmem),
			value(mem.Buffers), value(cache), value(available))
	} else {
		fmt.Fprintf(&b, "%-8s%12s%12s%12s%12s%12s%12s\n",
			" ", "total", "used", "free", "shared", "buff/cache", "available")
		fmt.Fprintf(&b, "%-8s%12s%12s%12s%12s%12s%12s\n",
			"Mem:", value(mem.MemTotal), value(used), value(mem.MemFree), value(mem.Shmem),
			value(addSat(mem.Buffers, cache)), value(available))
	}

	swapUsed := subSat(mem.SwapTotal, mem.SwapFree)
	fmt.Fprintf(&b, "%-8s%12s%12s%12s\n", "Swap:", value(mem.SwapTotal), value(swapUsed), value(mem.SwapFree))
	if total {
		fmt.Fprintf(&b, "%-8s%12s%12s%12s\n", "Total:",
			value(addSat(mem.MemTotal, mem.SwapTotal)),
			value(addSat(used, swapUsed)),
			value(addSat(mem.MemFree, mem.SwapFree)))
	}
	return b.String()
}

// freeHumanize renders a kibibyte count as "1.5Gi" (or "1.6G" with --si),
// rounding to one decimal below 100 units and to whole units above that, which
// is what the reference does.
func freeHumanize(kib uint64, si bool) string {
	units := []string{"B", "K", "M", "G", "T", "P"}
	bytes := kib * 1024
	level := 0
	divisor := uint64(1)
	base := uint64(1024)
	if si {
		base = 1000
	}
	for level < len(units)-1 && divisor*1000 <= bytes {
		divisor *= base
		level++
	}
	if level == 0 {
		return strconv.FormatUint(bytes, 10) + units[0]
	}
	value := float64(bytes) / float64(divisor)
	var number string
	if math.Round(value*10) < 100 {
		number = strconv.FormatFloat(math.Round(value*10)/10, 'f', 1, 64)
	} else {
		number = strconv.FormatUint(uint64(value), 10)
	}
	suffix := units[level]
	if !si {
		suffix += "i"
	}
	return number + suffix
}

func addSat(a, b uint64) uint64 {
	sum := a + b
	if sum < a {
		return math.MaxUint64
	}
	return sum
}

func subSat(a, b uint64) uint64 {
	if b >= a {
		return 0
	}
	return a - b
}
