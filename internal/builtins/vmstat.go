// vmstat — report virtual memory statistics.
//
// The column set, the two header lines, the column widths and the width
// compensation of over-long values follow toybox toys/other/vmstat.c (0BSD,
// commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb); -a and -S follow uutils
// procps vmstat (MIT, commit e697667b3f758803e38d8ab4c8f912171beca602). Two
// deliberate fixes against the toybox source: swapped pages are converted from
// pages to KiB once (toybox multiplies pswpin/pswpout by the page size twice)
// and the "since boot" divisor is the uptime, not /proc/uptime's idle seconds.
// An absent counter is printed as "-", never as a zero: the swap, Active/
// Inactive and pg*/pswp* fields can disappear on some kernels, while MemFree/
// Buffers/Cached exist on every kernel that has a procfs. Deliberately
// unsupported, failing as unknown options: -w, -t, -y, -s, -d, -D, -p, -f, -m.
// Data comes from internal/proc (Linux/Android); other platforms are reported
// as unsupported. See docs/CREDITS.md.

package builtins

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

const vmstatSpec = "^n(one-header)a(active)S(unit):"

var vmstatSpecParsed *Spec

func init() {
	var err error
	vmstatSpecParsed, err = ParseSpec(vmstatSpec)
	if err != nil {
		panic("builtins: bad vmstat spec: " + err.Error())
	}
	Register(Command{Name: "vmstat", Run: runVmstat})
}

// Seams for tests: the procfs root and the page size.
var (
	vmstatProcRoot = ""
	vmstatPageSize = os.Getpagesize
)

// vmstatSample is one read of the three procfs files vmstat needs.
type vmstatSample struct {
	stat proc.Stat
	mem  proc.Meminfo
	vm   proc.Vmstat
	up   time.Duration
}

func runVmstat(ctx *Context) int {
	flags, err := vmstatSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "vmstat: %v\nTry 'vmstat --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, vmstatUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", ctx.CmdName)
		return 0
	}
	delay, count, err := vmstatParseOperands(flags.Operands)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "vmstat: %v\nTry 'vmstat --help' for more information.\n", err)
		return 1
	}
	opts, err := vmstatOptionsFor(ctx, flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "vmstat: %v\nTry 'vmstat --help' for more information.\n", err)
		return 1
	}

	reader := proc.New(vmstatProcRoot)
	cancel := ctx.CancellationContext()
	var prev *vmstatSample
	for loop := 0; ; loop++ {
		if loop > 0 {
			if err := sampleSleep(cancel, time.Duration(delay)*time.Second); err != nil {
				return ExitStatus(err)
			}
		}
		cur, ok := vmstatRead(ctx, reader)
		if !ok {
			return 1
		}
		if vmstatWantsHeader(loop, opts) {
			fmt.Fprint(ctx.Stdout, vmstatHeader(opts))
		}
		secs := uint64(delay)
		if loop == 0 {
			// The first line is since boot, so rates use the uptime.
			secs = uint64(cur.up / time.Second)
			if secs == 0 {
				secs = 1
			}
		}
		fmt.Fprint(ctx.Stdout, vmstatRow(vmstatValues(cur, prev, secs, opts)))
		if delay == 0 || (count > 0 && loop+1 >= count) {
			return 0
		}
		prev = cur
	}
}

func vmstatUsage() string {
	return "Usage: vmstat [-n] [-a] [-S UNIT] [DELAY [COUNT]]\n\n" +
		"Report virtual memory statistics. The first line covers the time since\n" +
		"boot; each later line covers the DELAY seconds since the previous one.\n" +
		"Without DELAY vmstat prints one line; without COUNT it repeats until\n" +
		"interrupted.\n\n" +
		"  -n, --one-header  print the header only once\n" +
		"  -a, --active      show inact/active instead of buff/cache\n" +
		"  -S, --unit UNIT   memory unit: k or K (KiB, default), m or M (MiB)\n\n" +
		"Unsupported (rejected, never faked): -w, -t, -y, -s, -d, -D, -p, -f, -m.\n" +
		"Exit status: 0 printed, 1 bad usage or no readable procfs files.\n"
}

// vmstatParseOperands reads [DELAY [COUNT]]. A zero delay prints a single line,
// as in toybox.
func vmstatParseOperands(operands []string) (delay, count int, err error) {
	if len(operands) > 2 {
		return 0, 0, fmt.Errorf("extra operand '%s'", operands[2])
	}
	if len(operands) > 0 {
		delay, err = strconv.Atoi(operands[0])
		if err != nil || delay < 0 {
			return 0, 0, fmt.Errorf("invalid delay '%s'", operands[0])
		}
	}
	if len(operands) > 1 {
		count, err = strconv.Atoi(operands[1])
		if err != nil || count < 1 {
			return 0, 0, fmt.Errorf("invalid count '%s'", operands[1])
		}
	}
	return delay, count, nil
}

// vmstatOptions is the resolved formatting state for one run.
type vmstatOptions struct {
	oneHeader   bool
	active      bool
	unitDiv     uint64 // 1 for KiB, 1024 for MiB
	pageKB      uint64
	headerEvery int // 0 when the header must not repeat
}

func vmstatOptionsFor(ctx *Context, flags *ParseResult) (vmstatOptions, error) {
	o := vmstatOptions{
		oneHeader: flags.Bool("n"),
		active:    flags.Bool("a"),
		unitDiv:   1,
	}
	pageSize := vmstatPageSize()
	if pageSize < 1024 {
		pageSize = 4096
	}
	o.pageKB = uint64(pageSize / 1024)
	if u, ok := flags.String("S"); ok {
		switch u {
		case "k", "K":
		case "m", "M":
			o.unitDiv = 1024
		default:
			return o, fmt.Errorf("-S requires k, K, m or M (default is KiB)")
		}
	}
	if !o.oneHeader && ctx.StdoutTTY {
		rows := ctx.Height
		if rows <= 0 {
			rows = 25
		}
		if rows > 3 {
			o.headerEvery = rows - 3
		}
	}
	return o, nil
}

func vmstatWantsHeader(loop int, o vmstatOptions) bool {
	if loop == 0 {
		return true
	}
	if o.oneHeader || o.headerEvery <= 0 {
		return false
	}
	return loop%o.headerEvery == 0
}

// vmstatHeader is the two header lines: group names, then column names. The
// groups are sized by hand in the reference long before the widths are known,
// so they are not computed here either.
func vmstatHeader(o vmstatOptions) string {
	groups := "procs ------------memory------------ ---swap-- -----io---- --system- ----cpu----"
	if o.active {
		groups = "procs -----------memory---------- ---swap-- -----io---- --system- ----cpu----"
	}
	return groups + "\n" + vmstatPadded(vmstatColumnNames(o)) + "\n"
}

func vmstatColumnNames(o vmstatOptions) []string {
	names := []string{
		"r", "b", "swpd", "free", "buff", "cache", "si", "so",
		"bi", "bo", "in", "cs", "us", "sy", "id", "wa",
	}
	if o.active {
		names[4], names[5] = "inact", "active"
	}
	return names
}

// vmstatWidths is the reference's per-column width in characters.
var vmstatWidths = []int{2, 2, 7, 7, 6, 7, 4, 4, 5, 5, 4, 4, 2, 2, 2, 2}

// vmstatPadded right-aligns each name in its own width, one leading space from
// the second column on. The header never compensates for long values.
func vmstatPadded(values []string) string {
	var b strings.Builder
	for i, v := range values {
		if i > 0 {
			b.WriteByte(' ')
		}
		if pad := vmstatWidths[i] - len(v); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(v)
	}
	return b.String()
}

// vmstatRow renders one data line. Values that no longer fit their column push
// the rest of the line right, which is how the reference keeps the numbers
// readable instead of truncating them.
func vmstatRow(values []string) string {
	var b strings.Builder
	offset, expected := 0, 0
	for i, v := range values {
		pad := 0
		if i > 0 {
			pad = 1
		}
		expected += vmstatWidths[i] + pad
		w := expected - offset - pad
		if w < 0 {
			w = 0
		}
		if pad == 1 {
			b.WriteByte(' ')
		}
		grow := w
		if len(v) > w {
			grow = len(v)
		} else {
			b.WriteString(strings.Repeat(" ", w-len(v)))
		}
		b.WriteString(v)
		offset += pad + grow
	}
	return b.String() + "\n"
}

// vmstatRead takes one snapshot. A reader failure (no procfs, unreadable file)
// is reported once and ends the run.
func vmstatRead(ctx *Context, reader *proc.Reader) (*vmstatSample, bool) {
	up, err := reader.Uptime()
	if err != nil {
		printProcError(ctx, "vmstat", "the system uptime", err)
		return nil, false
	}
	st, err := reader.Stat()
	if err != nil {
		printProcError(ctx, "vmstat", "cpu statistics", err)
		return nil, false
	}
	mem, err := reader.Meminfo()
	if err != nil {
		printProcError(ctx, "vmstat", "memory information", err)
		return nil, false
	}
	vm, err := reader.Vmstat()
	if err != nil {
		printProcError(ctx, "vmstat", "virtual memory statistics", err)
		return nil, false
	}
	return &vmstatSample{stat: st, mem: mem, vm: vm, up: up}, true
}

// vmstatValues turns one or two samples into the sixteen display columns. With
// prev == nil every counter is since boot; otherwise it is the delta since the
// previous line.
func vmstatValues(cur, prev *vmstatSample, secs uint64, o vmstatOptions) []string {
	var old vmstatSample
	if prev != nil {
		old = *prev
	}
	delta := func(a, b uint64) uint64 {
		if a < b {
			return 0
		}
		return a - b
	}
	cpu := cur.stat.CPU.Sub(old.stat.CPU)
	total := cpu.Total()
	percent := func(v uint64) string {
		if total == 0 {
			return "0"
		}
		return strconv.FormatUint((v*100+total/2)/total, 10)
	}
	// perSecond converts a page counter delta into KiB per second.
	perSecond := func(pages uint64) string {
		if secs == 0 {
			secs = 1
		}
		return strconv.FormatUint(vmstatCeilDiv(pages*o.pageKB, secs), 10)
	}
	rate := func(n uint64) string {
		if secs == 0 {
			secs = 1
		}
		return strconv.FormatUint(vmstatCeilDiv(n, secs), 10)
	}
	memKiB := func(kib uint64) string {
		return strconv.FormatUint(kib/o.unitDiv, 10)
	}
	optional := func(counter uint64, missing bool) string {
		if missing {
			return "-"
		}
		return perSecond(counter)
	}
	missing := func(name string) bool {
		for _, m := range cur.vm.Missing {
			if m == name {
				return true
			}
		}
		return false
	}

	running := cur.stat.ProcsRunning
	if running > 0 {
		running-- // do not count vmstat itself
	}
	swapUsed := "0"
	if cur.mem.SwapKnown {
		swapUsed = memKiB(subSat(cur.mem.SwapTotal, cur.mem.SwapFree))
	}
	var buff, cache string
	if o.active {
		buff, cache = "-", "-"
		if cur.mem.InactiveKnown {
			buff = memKiB(cur.mem.Inactive)
		}
		if cur.mem.ActiveKnown {
			cache = memKiB(cur.mem.Active)
		}
	} else {
		buff = memKiB(cur.mem.Buffers)
		cache = memKiB(addSat(cur.mem.Cached, cur.mem.SReclaimable))
	}

	return []string{
		strconv.FormatUint(running, 10),
		strconv.FormatUint(cur.stat.ProcsBlocked, 10),
		swapUsed,
		memKiB(cur.mem.MemFree),
		buff,
		cache,
		optional(delta(cur.vm.PSWPIn, old.vm.PSWPIn), missing("pswpin")),
		optional(delta(cur.vm.PSWPOut, old.vm.PSWPOut), missing("pswpout")),
		optional(delta(cur.vm.PGPGIn, old.vm.PGPGIn), missing("pgpgin")),
		optional(delta(cur.vm.PGPGOut, old.vm.PGPGOut), missing("pgpgout")),
		rate(delta(cur.stat.Intr, old.stat.Intr)),
		rate(delta(cur.stat.Ctxt, old.stat.Ctxt)),
		percent(cpu.User),
		percent(cpu.System),
		percent(cpu.Idle),
		percent(cpu.Iowait),
	}
}

func vmstatCeilDiv(a, b uint64) uint64 {
	if b == 0 {
		return 0
	}
	return (a + b - 1) / b
}
