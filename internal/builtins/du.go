package builtins

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AmadeusDE/Misa/internal/lscore"
)

const duSpec = "s(summarize)h(human-readable)a(all)c(total)k(kilobytes)m(megabytes)b(bytes)d(max-depth)#L(dereference)H(dereference-args)D0(null)l(count-links)A(apparent-size)S(separate-dirs)x(one-file-system)B(block-size):(inodes)(exclude):(exclude-from):(si)t(threshold):P(no-dereference)(files0-from):v(verbose)"

// parseBlockSizeArg parses a GNU-style --block-size value: an optional
// leading count (default 1) followed by an optional unit suffix (b, K/KiB,
// KB, M/MiB, MB, G/GiB, GB, T/TiB, TB — case-insensitive for the letter,
// binary units using powers of 1024, decimal "B"-suffixed units using
// powers of 1000).
func parseBlockSizeArg(s string) (int64, error) {
	sign := int64(1)
	work := s
	if strings.HasPrefix(work, "-") {
		sign = -1
		work = work[1:]
	} else if strings.HasPrefix(work, "+") {
		work = work[1:]
	}
	i := 0
	for i < len(work) && work[i] >= '0' && work[i] <= '9' {
		i++
	}
	numPart, suffix := work[:i], work[i:]

	n := int64(1)
	if numPart != "" {
		v, err := strconv.ParseInt(numPart, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid --block-size argument '%s'", s)
		}
		n = v
	} else if suffix == "" {
		return 0, fmt.Errorf("invalid --block-size argument '%s'", s)
	}

	const ki, mi, gi, ti = 1 << 10, 1 << 20, 1 << 30, 1 << 40
	const k, m, g, t = 1000, 1000 * 1000, 1000 * 1000 * 1000, 1000 * 1000 * 1000 * 1000
	var mult int64
	switch strings.ToLower(suffix) {
	case "":
		mult = 1
	case "b":
		mult = 512
	case "k", "kib":
		mult = ki
	case "kb":
		mult = k
	case "m", "mib":
		mult = mi
	case "mb":
		mult = m
	case "g", "gib":
		mult = gi
	case "gb":
		mult = g
	case "t", "tib":
		mult = ti
	case "tb":
		mult = t
	default:
		if numPart != "" {
			return 0, fmt.Errorf("invalid suffix in --block-size argument '%s'", s)
		}
		return 0, fmt.Errorf("invalid --block-size argument '%s'", s)
	}
	return sign * n * mult, nil
}

var duSpecParsed *Spec

func init() {
	var err error
	duSpecParsed, err = ParseSpec(duSpec)
	if err != nil {
		panic("builtins: bad du spec: " + err.Error())
	}
	Register(Command{Name: "du", Run: runDu})
}

// duOptions bundles du's flags so they don't have to be threaded through the
// recursive walk one parameter at a time.
type duOptions struct {
	all           bool
	human         bool
	si            bool
	useBytes      bool
	apparentSize  bool
	separateDirs  bool
	oneFileSystem bool
	inodes        bool
	countLinks    bool
	derefAll      bool
	derefArgs     bool
	blockSize     int64 // bytes per reported unit; 0 means "use the default"
	maxDepth      int64
	threshold     int64
	hasThreshold  bool
	exclude       []string
	term          string
}

func runDu(ctx *Context) int {
	flags, err := duSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "du:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, duSpecParsed) {
		return 0
	}

	opts := &duOptions{
		all:           flags.Bool("a"),
		human:         flags.Bool("h"),
		si:            flags.Bool("si"),
		separateDirs:  flags.Bool("S"),
		oneFileSystem: flags.Bool("x"),
		inodes:        flags.Bool("inodes"),
		countLinks:    flags.Bool("l"),
		derefAll:      flags.Bool("L"),
		derefArgs:     flags.Bool("H") || flags.Bool("D"),
	}
	opts.useBytes = flags.Bool("b")
	opts.apparentSize = flags.Bool("A") || opts.useBytes
	if opts.si {
		opts.human = true
	}
	summarize := flags.Bool("s")
	total := flags.Bool("c")

	if summarize && opts.all {
		fmt.Fprintln(ctx.Stderr, "du: cannot both summarize and show all entries")
		return 1
	}

	if opts.inodes && opts.apparentSize {
		fmt.Fprintln(ctx.Stderr, "du: warning: options --apparent-size and -b are ineffective with --inodes")
	}
	if tStr, ok := flags.ExplicitString("threshold"); ok {
		if strings.HasPrefix(tStr, " ") || strings.HasPrefix(tStr, "\t") {
			fmt.Fprintf(ctx.Stderr, "du: invalid --threshold argument '%s'\n", tStr)
			return 1
		}
		v, err := parseBlockSizeArg(tStr)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "du: invalid --threshold argument '%s'\n", tStr)
			return 1
		}
		opts.threshold = v
		opts.hasThreshold = true
	} else if v, ok := flags.Num("threshold"); ok {
		opts.threshold = v
		opts.hasThreshold = true
	}

	switch {
	case opts.useBytes:
		opts.blockSize = 1
	case flags.Bool("m"):
		opts.blockSize = 1024 * 1024
	}
	if bsArg, ok := flags.String("B"); ok {
		bs, berr := parseBlockSizeArg(bsArg)
		if berr != nil {
			fmt.Fprintf(ctx.Stderr, "du: %v\n", berr)
			return 1
		}
		opts.blockSize = bs
	}

	opts.exclude = append(opts.exclude, flags.AllStrings("exclude")...)
	if f, ok := flags.String("exclude-from"); ok {
		patterns, err := duReadExcludeFile(ctx, f)
		if err != nil {
			msg := fmtOsErr(err)
			if strings.Contains(err.Error(), "read error") || strings.Contains(err.Error(), "Input/output error") {
				msg = "Input/output error"
			}
			if msg == "Input/output error" {
				fmt.Fprintln(ctx.Stderr, "du: Input/output error")
			} else {
				fmt.Fprintf(ctx.Stderr, "du: %s: %s\n", f, msg)
			}
			return 1
		}
		opts.exclude = append(opts.exclude, patterns...)
	}

	opts.maxDepth = -1
	if v, ok := flags.Num("d"); ok {
		if v < 0 {
			fmt.Fprintf(ctx.Stderr, "du: invalid maximum depth '%d'\nTry 'du --help' for more information.\n", v)
			return 1
		}
		opts.maxDepth = v
	}
	if summarize {
		opts.maxDepth = 0
	}

	targets := flags.Operands
	if f0, ok := flags.String("files0-from"); ok {
		var content []byte
		var err error
		if f0 == "-" {
			if ctx.Stdin != nil {
				content, err = io.ReadAll(ctx.Stdin)
			}
		} else {
			content, err = os.ReadFile(resolvePath(ctx, f0))
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "du: %v\n", err)
			return 1
		}
		lines := strings.Split(string(content), "\x00")
		targets = nil
		for idx, l := range lines {
			if idx > 0 && l == "" && idx < len(lines)-1 {
				fmt.Fprintf(ctx.Stderr, "du: %s:%d: invalid zero-length file name\n", f0, idx+1)
				return 1
			}
			if l != "" {
				targets = append(targets, l)
			}
		}
	}
	if len(targets) == 0 {
		targets = []string{"."}
	}

	opts.term = "\n"
	if flags.Bool("0") {
		opts.term = "\x00"
	}

	var grandTotal int64
	exitCode := 0
	visited := make(map[uint64]bool)

	for _, target := range targets {
		path := resolvePath(ctx, target)

		var startDev uint64
		if st, serr := os.Lstat(path); serr == nil {
			if sysStat, ok := readFSMeta(path, st); ok {
				startDev = sysStat.Dev
			}
		}

		size, err := duWalk(ctx, path, target, 0, opts, visited, startDev)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "du: %v\n", err)
			exitCode = 1
			continue
		}
		grandTotal += size
	}

	if total {
		duPrintValue(ctx, grandTotal, "total", opts)
	}

	return exitCode
}

func duReadExcludeFile(ctx *Context, name string) ([]string, error) {
	path := resolvePath(ctx, name)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			patterns = append(patterns, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return patterns, nil
}

// duExcluded matches path against each pattern the way GNU du's --exclude
// does: try the whole path, then progressively strip the leading "a/"
// component and retry, so a bare pattern like "x" matches any path component.
func duExcluded(patterns []string, path string) bool {
	for _, pat := range patterns {
		p := path
		for {
			if ok, _ := filepath.Match(pat, p); ok {
				return true
			}
			idx := strings.IndexByte(p, '/')
			if idx < 0 {
				break
			}
			p = p[idx+1:]
		}
	}
	return false
}

func duEntryValue(path string, info os.FileInfo, opts *duOptions) int64 {
	if opts.inodes {
		return 1
	}
	if opts.apparentSize {
		// In apparent-size mode, directories don't contribute to the size
		if info.IsDir() {
			return 0
		}
		return info.Size()
	}
	if st, ok := readFSMeta(path, info); ok && !st.lacks(lscore.FieldBlocks) {
		return st.Blocks * 512
	}
	return info.Size()
}

func duBlockSize(opts *duOptions) int64 {
	if opts.blockSize > 0 {
		return opts.blockSize
	}
	return 1024
}

func duFormatValue(value int64, opts *duOptions) string {
	if opts.human {
		if opts.si {
			return humanSizeBase(value, 1000)
		}
		return humanSizeBase(value, 1024)
	}
	if opts.inodes {
		return fmt.Sprintf("%d", value)
	}
	bs := duBlockSize(opts)
	return fmt.Sprintf("%d", (value+bs-1)/bs)
}

// duPassesThreshold reports whether value should be printed under
// --threshold=SIZE: entries smaller than a positive SIZE, or entries greater
// than the absolute value of a negative SIZE, are suppressed.
func duPassesThreshold(value int64, opts *duOptions) bool {
	if !opts.hasThreshold {
		return true
	}
	if opts.threshold >= 0 {
		return value >= opts.threshold
	}
	return value <= -opts.threshold
}

func duPrintValue(ctx *Context, value int64, name string, opts *duOptions) {
	if !duPassesThreshold(value, opts) {
		return
	}
	fmt.Fprintf(ctx.Stdout, "%s\t%s%s", duFormatValue(value, opts), name, opts.term)
}

func duWalk(ctx *Context, absPath, displayPath string, depth int64, opts *duOptions, visited map[uint64]bool, startDev uint64) (int64, error) {
	var info os.FileInfo
	var err error
	if opts.derefAll || (opts.derefArgs && depth == 0) {
		info, err = os.Stat(absPath)
	} else {
		info, err = os.Lstat(absPath)
	}
	if err != nil {
		return 0, err
	}

	if depth > 0 && opts.oneFileSystem {
		if st, ok := readFSMeta(absPath, info); ok && st.Dev != startDev {
			return 0, nil
		}
	}
	if duExcluded(opts.exclude, displayPath) {
		return 0, nil
	}

	if !opts.countLinks {
		if st, ok := readFSMeta(absPath, info); ok && !st.lacks(lscore.FieldInode) {
			if visited[st.Ino] {
				return 0, nil
			}
			visited[st.Ino] = true
		}
	}

	if !info.IsDir() {
		value := duEntryValue(absPath, info, opts)
		if opts.all || depth == 0 {
			duPrintValue(ctx, value, displayPath, opts)
		}
		return value, nil
	}

	realPath := absPath
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		realPath = resolved
	}

	entries, err := os.ReadDir(realPath)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "du: cannot read directory '%s': %s\n", displayPath, fmtOsErr(err))
		value := duEntryValue(absPath, info, opts)
		if opts.maxDepth < 0 || depth <= opts.maxDepth {
			duPrintValue(ctx, value, displayPath, opts)
		}
		return value, err
	}

	var recursiveTotal int64
	var ownTotal int64
	var walkErr error
	for _, e := range entries {
		childAbs := filepath.Join(realPath, e.Name())
		childDisplay := filepath.Join(displayPath, e.Name())

		var childInfo os.FileInfo
		if opts.derefAll {
			childInfo, err = os.Stat(childAbs)
		} else {
			childInfo, err = os.Lstat(childAbs)
		}
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "du: %v\n", err)
			continue
		}

		if opts.oneFileSystem {
			if st, ok := readFSMeta(childAbs, childInfo); ok && st.Dev != startDev {
				continue
			}
		}
		if duExcluded(opts.exclude, childDisplay) {
			continue
		}

		if childInfo.IsDir() {
			sub, err := duWalk(ctx, childAbs, childDisplay, depth+1, opts, visited, startDev)
			recursiveTotal += sub
			if err != nil {
				walkErr = err
				continue
			}
		} else {
			if !opts.countLinks {
				if st, ok := readFSMeta(childAbs, childInfo); ok && !st.lacks(lscore.FieldInode) {
					if visited[st.Ino] {
						continue
					}
					visited[st.Ino] = true
				}
			}
			value := duEntryValue(childAbs, childInfo, opts)
			recursiveTotal += value
			ownTotal += value
			if opts.all && (opts.maxDepth < 0 || depth+1 <= opts.maxDepth) {
				duPrintValue(ctx, value, childDisplay, opts)
			}
		}
	}

	var dirInfo os.FileInfo
	if opts.derefAll || (opts.derefArgs && depth == 0) {
		dirInfo, _ = os.Stat(absPath)
	} else {
		dirInfo, _ = os.Lstat(absPath)
	}
	if dirInfo != nil {
		dirValue := duEntryValue(absPath, dirInfo, opts)
		recursiveTotal += dirValue
		ownTotal += dirValue
	}

	if opts.maxDepth < 0 || depth <= opts.maxDepth {
		if opts.separateDirs {
			duPrintValue(ctx, ownTotal, displayPath, opts)
		} else {
			duPrintValue(ctx, recursiveTotal, displayPath, opts)
		}
	}

	return recursiveTotal, walkErr
}

func humanSize(bytes int64) string {
	return humanSizeBase(bytes, 1024)
}

// humanSizeBase formats bytes using the given scale (1024 for the IEC-ish
// binary K/M/G/... du -h uses, or 1000 for --si's decimal k/M/G/...),
// rounding UP to one decimal place so the displayed size never under-reports
// actual usage, matching GNU du's convention.
func humanSizeBase(bytes int64, base int64) string {
	if bytes == 0 {
		return "0"
	}
	kb := base
	mb := base * base
	gb := base * base * base
	kLetter := "K"
	if base == 1000 {
		kLetter = "k"
	}
	switch {
	case bytes >= gb:
		return ceilDecimal(bytes, gb) + "G"
	case bytes >= mb:
		return ceilDecimal(bytes, mb) + "M"
	case bytes >= kb:
		return ceilDecimal(bytes, kb) + kLetter
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// ceilDecimal divides bytes by unit and rounds the result UP to one decimal
// place, returned as "N.N".
func ceilDecimal(bytes, unit int64) string {
	scaled := bytes * 10
	q := (scaled + unit - 1) / unit
	return fmt.Sprintf("%d.%d", q/10, q%10)
}
