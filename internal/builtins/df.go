// Behaviour adapted from toybox toys/posix/df.c (0BSD, toybox commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb) and uutils coreutils src/uu/df
// (MIT, commit fee61ff5d4f87e0f912a52e639942b5f7ad58663). The column layout,
// the unit selection and the truncating block scaling are toybox's; -x, -T, -B,
// over-mounted filtering and the block-size header labels are uutils'. The
// -h/-H columns port toybox's human_readable_long and human_readable from
// lib/lib.c (0BSD, same commit); the -B header label ports
// to_magnitude_and_suffix from uutils src/uu/df/src/blocks.rs (MIT, same
// commit), which is what makes `-B 1024` print "1K-blocks" and `-B 1000`
// print "1kB-blocks" (uutils test_block_size_1024, test_block_size_with_suffix).
//
// Known differences from toybox: the available count is always statfs's
// Bavail (toybox shows Bfree for uid 0, so a reserve-carrying filesystem shows
// the same free space for every user here), inode counts stay plain numbers
// under -i -h (toybox would human-format them), and a filesystem whose statfs
// fails is dropped from the full listing silently instead of being reported.
package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/AmadeusDE/Misa/internal/mounts"
)

const dfSpec = "a(all)h(human-readable)H(si)i(inodes)k(kilo)P(portability)t(type):x(exclude-type):T(print-type)B(block-size):"

var dfSpecParsed *Spec

// The two side-effecting dependencies, overridable from tests.
var (
	dfListMounts = mounts.List
	dfStatfs     = mounts.Statfs
)

func init() {
	var err error
	dfSpecParsed, err = ParseSpec(dfSpec)
	if err != nil {
		panic("builtins: bad df spec: " + err.Error())
	}
	Register(Command{Name: "df", Run: runDf})
}

// dfOptions holds df's flags plus the unit derived from them.
type dfOptions struct {
	all         bool
	human       bool // -h: 1024-based human sizes
	si          bool // -H: 1000-based human sizes
	inodes      bool
	kilo        bool
	portability bool // -P
	printType   bool // -T
	blockSize   int64
	types       []string
	exclude     []string
	unit        uint64 // bytes per reported block
}

// dfRow is one line of the table.
type dfRow struct {
	device string
	fstype string
	mount  string
	stat   mounts.Stat
}

func runDf(ctx *Context) int {
	flags, err := dfSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "df:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, dfSpecParsed) {
		return 0
	}

	opts := &dfOptions{
		all:         flags.Bool("a"),
		human:       flags.Bool("h"),
		si:          flags.Bool("si"),
		inodes:      flags.Bool("i"),
		kilo:        flags.Bool("k"),
		portability: flags.Bool("P"),
		printType:   flags.Bool("T"),
		types:       dfUnique(flags.AllStrings("type")),
		exclude:     dfUnique(flags.AllStrings("exclude-type")),
	}
	if bs, ok := flags.ExplicitString("B"); ok {
		opts.blockSize, err = parseBlockSizeArg(bs)
		if err == nil && opts.blockSize <= 0 {
			err = fmt.Errorf("invalid --block-size argument '%s'", bs)
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "df: %v\n", err)
			return 1
		}
	}
	if !opts.checkTypeLists(ctx) {
		return 1
	}
	opts.unit = opts.blockUnit()

	if len(flags.Operands) > 0 {
		return dfOperands(ctx, opts, flags.Operands)
	}
	return dfAll(ctx, opts)
}

// checkTypeLists reports every type that is both selected with -t and excluded
// with -x, in the order the -t flags were given.
func (o *dfOptions) checkTypeLists(ctx *Context) bool {
	ok := true
	for _, t := range o.types {
		for _, x := range o.exclude {
			if t == x {
				fmt.Fprintf(ctx.Stderr, "df: file system type '%s' both selected and excluded\n", t)
				ok = false
				break
			}
		}
	}
	return ok
}

// dfUnique drops the repeats the option parser records for a flag that has
// both a short and a long name, keeping the order the flags were given in.
func dfUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := values[:0]
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// accepts applies the -t and -x type lists to one filesystem type.
func (o *dfOptions) accepts(fstype string) bool {
	if len(o.types) > 0 {
		found := false
		for _, t := range o.types {
			if t == fstype {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, x := range o.exclude {
		if x == fstype {
			return false
		}
	}
	return true
}

// dfAll lists every mounted filesystem in mount-table order.
func dfAll(ctx *Context, opts *dfOptions) int {
	table, err := dfListMounts()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "df: cannot read table of mounted file systems: %v\n", err)
		return 1
	}
	rows := make([]dfRow, 0, len(table))
	for i, m := range table {
		if dfOverMounted(table, i) {
			continue
		}
		st, err := dfStatfs(m.Dir)
		if err != nil {
			continue
		}
		if st.Blocks == 0 && !opts.all {
			continue
		}
		if !opts.accepts(m.FSType) {
			continue
		}
		rows = append(rows, dfRow{device: m.Device, fstype: m.FSType, mount: m.Dir, stat: st})
	}
	dfPrint(ctx, opts, rows)
	return 0
}

// dfOverMounted reports whether a later entry hides table[i]: the same mount
// point mounted again from a different device.
func dfOverMounted(table []mounts.Mount, i int) bool {
	for _, later := range table[i+1:] {
		if later.Dir == table[i].Dir && later.Device != table[i].Device {
			return true
		}
	}
	return false
}

// dfOperands prints one row per operand, like Toybox. A mount table that
// cannot be read is only a warning here: the device and type columns fall back
// to the device-id walk instead.
func dfOperands(ctx *Context, opts *dfOptions, operands []string) int {
	table, err := dfListMounts()
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "df: cannot read table of mounted file systems: %v\n", err)
		table = nil
	}
	status := 0
	rows := make([]dfRow, 0, len(operands))
	for _, operand := range operands {
		row, err := dfOperand(ctx, opts, table, operand)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "df: '%s': %s\n", operand, fmtOsErr(err))
			status = 1
			continue
		}
		if row != nil {
			rows = append(rows, *row)
		}
	}
	dfPrint(ctx, opts, rows)
	return status
}

// dfOperand resolves one operand and finds the filesystem behind it. A
// filtered-out operand yields no row and no error.
func dfOperand(ctx *Context, opts *dfOptions, table []mounts.Mount, operand string) (*dfRow, error) {
	path, err := filepath.EvalSymlinks(resolvePath(ctx, operand))
	if err != nil {
		return nil, err
	}
	st, err := dfStatfs(path)
	if err != nil {
		return nil, err
	}
	row := dfRow{device: "-", fstype: "-", mount: dfMountPoint(path), stat: st}
	if m, ok := mounts.Match(table, path); ok {
		row.device, row.fstype, row.mount = m.Device, m.FSType, m.Dir
	}
	if !opts.accepts(row.fstype) {
		return nil, nil
	}
	return &row, nil
}

// dfMountPoint walks up from path while the device number stays the same, so a
// path in an unlisted mount (a chroot, an unmounted device, a missing mount
// table) still reports the directory its filesystem is mounted on.
func dfMountPoint(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return path
	}
	dir := path
	if !info.IsDir() {
		dir = filepath.Dir(path)
	}
	dev, ok := dfDeviceID(dir)
	if !ok {
		return dir
	}
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		if pdev, ok := dfDeviceID(parent); !ok || pdev != dev {
			return dir
		}
		dir = parent
	}
}

func dfDeviceID(path string) (uint64, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}

// dfPrint writes the header and one line per row. Every column but the last is
// measured across the header and the values; the first column is never
// narrower than 14 characters (15 with -h/-H for the "1.2M" style values).
func dfPrint(ctx *Context, opts *dfOptions, rows []dfRow) {
	lines := make([][]string, 0, len(rows)+1)
	lines = append(lines, opts.header())
	for _, r := range rows {
		lines = append(lines, opts.cells(r))
	}
	widths := make([]int, len(lines[0]))
	for _, cells := range lines {
		for i := 0; i < len(cells)-1; i++ {
			widths[i] = max(widths[i], len(cells[i]))
		}
	}
	minimum := 14
	if opts.human || opts.si {
		minimum = 15
	}
	widths[0] = max(widths[0], minimum)

	for _, cells := range lines {
		for i, cell := range cells {
			switch i {
			case 0:
				fmt.Fprintf(ctx.Stdout, "%-*s", widths[0], cell)
			case len(cells) - 1:
				fmt.Fprintf(ctx.Stdout, " %s\n", cell)
			default:
				fmt.Fprintf(ctx.Stdout, " %*s", widths[i], cell)
			}
		}
	}
}

// header builds the header cells for the current mode.
func (o *dfOptions) header() []string {
	cells := []string{"Filesystem", "Size", "Used", "Avail", "Use%", "Mounted on"}
	switch {
	case o.inodes:
		cells[1], cells[2], cells[3], cells[4] = "Inodes", "IUsed", "IFree", "IUse%"
	case o.human || o.si:
		// Toybox keeps the generic names in human-readable mode.
	default:
		cells[1], cells[3] = o.sizeLabel(), "Available"
		if o.portability {
			cells[4] = "Capacity"
		}
	}
	if o.printType {
		cells = append(cells[:1], append([]string{"Type"}, cells[1:]...)...)
	}
	return cells
}

// cells builds the value cells of one row, scaled the way Toybox does: counts
// are multiplied by the fundamental block size and truncated after dividing by
// the unit.
func (o *dfOptions) cells(r dfRow) []string {
	block, total, used, avail := uint64(1), r.stat.Files, dfSub(r.stat.Files, r.stat.Ffree), r.stat.Ffree
	if !o.inodes {
		block = max(r.stat.Bsize, 1)
		total, used, avail = r.stat.Blocks, dfSub(r.stat.Blocks, r.stat.Bfree), r.stat.Bavail
	}
	scale := func(n uint64) uint64 { return block * n / o.unit }
	size, usedBlocks, availBlocks := scale(total), scale(used), scale(avail)

	cells := []string{
		r.device,
		o.number(size),
		o.number(usedBlocks),
		o.number(availBlocks),
		fmt.Sprintf("%d%%", dfPercent(usedBlocks, availBlocks)),
		r.mount,
	}
	if o.printType {
		cells = append(cells[:1], append([]string{r.fstype}, cells[1:]...)...)
	}
	return cells
}

// number formats a scaled count: human-readable for -h/-H, plain digits
// otherwise (inode counts are never condensed).
func (o *dfOptions) number(scaled uint64) string {
	if !o.inodes && (o.human || o.si) {
		return humanReadable(scaled, o.si)
	}
	return strconv.FormatUint(scaled, 10)
}
