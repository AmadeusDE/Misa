// getconf: get configuration values. Interface and name set follow Toybox
// toys/posix/getconf.c (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb).
//
// Every name is answered from one of three sources, never from a guess: a
// runtime probe (os.Getpagesize, runtime.NumCPU, getrlimit, Linux sysfs), the
// per-platform constant table in getconf_platform_*.go, or a POSIX.1-2008
// constant. A name this build cannot answer prints "undefined".
// See docs/plans/getconf.md.
package builtins

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const getconfUsage = `usage: getconf -a [PATH] | -l | NAME [PATH]

Get system configuration values. Values from pathconf(3) require a path.

-a	Show all (defaults to "/" if no path given)
-l	List available value names (grouped by source)
`

func init() { Register(Command{Name: "getconf", Run: runGetconf}) }

// confKind is the source a group of names comes from, in Toybox's order.
type confKind int

const (
	confSysconf confKind = iota
	confPathconf
	confConfstr
	confLimits
	confMisc
)

// confValue returns a name's value and whether this build can determine it.
// A false second result is printed as "undefined".
type confValue func(env *confEnv) (string, bool)

type confEntry struct {
	name  string
	value confValue
}

type confGroup struct {
	kind    confKind
	title   string
	entries []confEntry
}

// confGroups is the single source of truth for -a, -l and single lookups, so
// the three modes cannot disagree. Order and spelling follow Toybox's
// sysconfs[], pathconfs[], confstrs[], limits[] and others[].
var confGroups = []confGroup{
	{confSysconf, "sysconf(3)", sysconfEntries()},
	{confPathconf, "pathconf(3)", pathconfEntries()},
	{confConfstr, "confstr(3)", confstrEntries()},
	{confLimits, "<limits.h>", limitsEntries()},
	{confMisc, "Misc", miscEntries()},
}

// confEnv carries the invocation's state: the path a pathconf(3) name was asked
// about and the lazily read CPU cache table. The path is only checked for
// presence, never opened: the pathconf values are platform maxima, not
// per-filesystem probes (see docs/plans/getconf.md).
type confEnv struct {
	path  string
	cache map[string]string
}

// getconfLimits is the per-platform constant table, one file per platform (see
// getconf_platform_*.go). A negative value means this build cannot state the
// platform's limit, so the name reports "undefined" instead of a guess.
type getconfLimits struct {
	argMax       int64
	clkTck       int64
	hostNameMax  int64
	lineMax      int64
	iovMax       int64
	loginNameMax int64
	ngroupsMax   int64
	openMax      int64
	pathMax      int64
	nameMax      int64
	linkMax      int64
	pipeBuf      int64
	maxCanon     int64
	maxInput     int64
	symlinkMax   int64
	symloopMax   int64
	ttyNameMax   int64
	tzNameMax    int64
	vdisable     int64
	fileSizeBits int64
	confstrPath  string
}

func undefined(*confEnv) (string, bool) { return "", false }

func constant(s string) confValue {
	return func(*confEnv) (string, bool) { return s, true }
}

// platformValue reports a per-platform constant. A negative value in the
// platform table means the platform documents nothing this build can state, so
// the name reports "undefined" instead of a guess.
func platformValue(v int64) confValue {
	if v < 0 {
		return undefined
	}
	return constant(strconv.FormatInt(v, 10))
}

// platformString is platformValue for string values (the confstr PATH).
func platformString(s string) confValue {
	if s == "" {
		return undefined
	}
	return constant(s)
}

// measured turns a runtime probe into a value; a probe that cannot answer
// reports "undefined".
func measured(probe func() (int64, bool)) confValue {
	return func(*confEnv) (string, bool) {
		v, ok := probe()
		if !ok {
			return "", false
		}
		return strconv.FormatInt(v, 10), true
	}
}

// cacheValue reports one of the CPU cache names the platform's probe knows
// about; caches the machine does not have stay undefined.
func cacheValue(name string) confValue {
	return func(env *confEnv) (string, bool) {
		if env.cache == nil {
			env.cache = platformCacheInfo()
			if env.cache == nil {
				env.cache = map[string]string{}
			}
		}
		v, ok := env.cache[name]
		return v, ok
	}
}

func probePageSize() (int64, bool) { return int64(os.Getpagesize()), true }

func probeCPUCount() (int64, bool) { return int64(runtime.NumCPU()), true }

// probeOpenMax is the soft RLIMIT_NOFILE, the per-process descriptor cap POSIX
// calls OPEN_MAX. The platform table's OPEN_MAX is only a fallback for when the
// call fails.
func probeOpenMax() (int64, bool) {
	if v, ok := platformOpenMax(); ok {
		return v, true
	}
	return getconfPlatform.openMax, getconfPlatform.openMax >= 0
}

// sysconfEntries is Toybox's sysconfs[].
func sysconfEntries() []confEntry {
	var out []confEntry
	add := func(name string, v confValue) { out = append(out, confEntry{name, v}) }
	addAll := func(v confValue, names ...string) {
		for _, n := range names {
			add(n, v)
		}
	}

	// POSIX option tests. Answering one means asking a C library whether the
	// option is supported; this build has no C library to ask, so they report
	// "undefined" rather than a made-up -1 or 1.
	addAll(undefined,
		"_POSIX_ADVISORY_INFO", "_POSIX_BARRIERS", "_POSIX_ASYNCHRONOUS_IO",
		"_POSIX_CLOCK_SELECTION", "_POSIX_CPUTIME", "_POSIX_FSYNC", "_POSIX_IPV6",
		"_POSIX_JOB_CONTROL", "_POSIX_MAPPED_FILES", "_POSIX_MEMLOCK",
		"_POSIX_MEMLOCK_RANGE", "_POSIX_MEMORY_PROTECTION",
		"_POSIX_MESSAGE_PASSING", "_POSIX_MONOTONIC_CLOCK",
		"_POSIX_PRIORITY_SCHEDULING", "_POSIX_RAW_SOCKETS",
		"_POSIX_READER_WRITER_LOCKS", "_POSIX_REALTIME_SIGNALS", "_POSIX_REGEXP",
		"_POSIX_SAVED_IDS", "_POSIX_SEMAPHORES", "_POSIX_SHARED_MEMORY_OBJECTS",
		"_POSIX_SHELL", "_POSIX_SPAWN", "_POSIX_SPIN_LOCKS",
		"_POSIX_SPORADIC_SERVER", "_POSIX_SS_REPL_MAX", "_POSIX_SYNCHRONIZED_IO",
		"_POSIX_THREAD_ATTR_STACKADDR", "_POSIX_THREAD_ATTR_STACKSIZE",
		"_POSIX_THREAD_CPUTIME", "_POSIX_THREAD_PRIO_INHERIT",
		"_POSIX_THREAD_PRIO_PROTECT", "_POSIX_THREAD_PRIORITY_SCHEDULING",
		"_POSIX_THREAD_PROCESS_SHARED", "_POSIX_THREAD_ROBUST_PRIO_INHERIT",
		"_POSIX_THREAD_ROBUST_PRIO_PROTECT", "_POSIX_THREAD_SAFE_FUNCTIONS",
		"_POSIX_THREAD_SPORADIC_SERVER", "_POSIX_THREADS", "_POSIX_TIMEOUTS",
		"_POSIX_TIMERS", "_POSIX_TRACE", "_POSIX_TRACE_EVENT_FILTER",
		"_POSIX_TRACE_EVENT_NAME_MAX", "_POSIX_TRACE_INHERIT", "_POSIX_TRACE_LOG",
		"_POSIX_TRACE_NAME_MAX", "_POSIX_TRACE_SYS_MAX",
		"_POSIX_TRACE_USER_EVENT_MAX", "_POSIX_TYPED_MEMORY_OBJECTS")
	// POSIX.1-2008.
	add("_POSIX_VERSION", constant("200809"))
	addAll(undefined,
		"_POSIX_V7_ILP32_OFF32", "_POSIX_V7_ILP32_OFFBIG",
		"_POSIX_V7_LP64_OFF64", "_POSIX_V7_LPBIG_OFFBIG")

	// POSIX.2.
	addAll(undefined,
		"POSIX2_C_BIND", "POSIX2_C_DEV", "POSIX2_CHAR_TERM", "POSIX2_FORT_DEV",
		"POSIX2_FORT_RUN", "POSIX2_LOCALEDEF", "POSIX2_PBS",
		"POSIX2_PBS_ACCOUNTING", "POSIX2_PBS_CHECKPOINT", "POSIX2_PBS_LOCATE",
		"POSIX2_PBS_MESSAGE", "POSIX2_PBS_TRACK", "POSIX2_SW_DEV", "POSIX2_UPE")
	add("POSIX2_VERSION", constant("200809"))

	// X/Open.
	addAll(undefined,
		"_XOPEN_CRYPT", "_XOPEN_ENH_I18N", "_XOPEN_REALTIME",
		"_XOPEN_REALTIME_THREADS", "_XOPEN_SHM", "_XOPEN_STREAMS", "_XOPEN_UNIX",
		"_XOPEN_UUCP")
	add("_XOPEN_VERSION", constant("700"))

	// Names with no standard prefix.
	p := getconfPlatform
	addAll(undefined,
		"AIO_LISTIO_MAX", "AIO_MAX", "AIO_PRIO_DELTA_MAX")
	add("ARG_MAX", platformValue(p.argMax))
	add("ATEXIT_MAX", undefined)
	addAll(undefined,
		"BC_BASE_MAX", "BC_DIM_MAX", "BC_SCALE_MAX", "BC_STRING_MAX")
	add("CHILD_MAX", measured(platformChildMax))
	add("CLK_TCK", platformValue(p.clkTck))
	add("COLL_WEIGHTS_MAX", undefined)
	add("DELAYTIMER_MAX", undefined)
	add("EXPR_NEST_MAX", undefined)
	add("HOST_NAME_MAX", platformValue(p.hostNameMax))
	add("IOV_MAX", platformValue(p.iovMax))
	add("LINE_MAX", platformValue(p.lineMax))
	add("LOGIN_NAME_MAX", platformValue(p.loginNameMax))
	add("NGROUPS_MAX", platformValue(p.ngroupsMax))
	addAll(undefined, "MQ_OPEN_MAX", "MQ_PRIO_MAX")
	addAll(measured(probeCPUCount), "NPROCESSORS_CONF", "NPROCESSORS_ONLN")
	add("OPEN_MAX", measured(probeOpenMax))
	addAll(measured(probePageSize), "PAGE_SIZE", "PAGESIZE")
	addAll(undefined,
		"RAW_SOCKETS", "RE_DUP_MAX", "RTSIG_MAX", "SEM_NSEMS_MAX",
		"SEM_VALUE_MAX", "SIGQUEUE_MAX", "STREAM_MAX")
	add("SYMLOOP_MAX", platformValue(p.symloopMax))
	add("TIMER_MAX", undefined)
	add("TTY_NAME_MAX", platformValue(p.ttyNameMax))
	add("TZNAME_MAX", platformValue(p.tzNameMax))
	add("UIO_MAXIOV", platformValue(p.iovMax))

	// CPU cache geometry: measured from sysfs where the platform has it (see
	// getconf_platform_sysfs.go), undefined elsewhere.
	for _, set := range []string{"LEVEL1_ICACHE", "LEVEL1_DCACHE", "LEVEL2_CACHE", "LEVEL3_CACHE", "LEVEL4_CACHE"} {
		for _, part := range []string{"SIZE", "ASSOC", "LINESIZE"} {
			name := set + "_" + part
			add(name, cacheValue(name))
		}
	}

	// Aliases Toybox spells out by hand.
	addAll(undefined, "_AVPHYS_PAGES", "_PHYS_PAGES")
	addAll(measured(probeCPUCount), "_NPROCESSORS_CONF", "_NPROCESSORS_ONLN")
	addAll(undefined,
		"PTHREAD_DESTRUCTOR_ITERATIONS", "PTHREAD_KEYS_MAX",
		"PTHREAD_STACK_MIN", "PTHREAD_THREADS_MAX")

	return out
}

// pathconfEntries is Toybox's pathconfs[]. The values are the platform maxima
// from the platform table, not a per-filesystem probe (see docs/plans/getconf.md).
func pathconfEntries() []confEntry {
	p := getconfPlatform
	return []confEntry{
		{"ASYNC_IO", undefined},
		{"CHOWN_RESTRICTED", constant("1")},
		{"FILESIZEBITS", platformValue(p.fileSizeBits)},
		{"LINK_MAX", platformValue(p.linkMax)},
		{"MAX_CANON", platformValue(p.maxCanon)},
		{"MAX_INPUT", platformValue(p.maxInput)},
		{"NAME_MAX", platformValue(p.nameMax)},
		{"NO_TRUNC", constant("1")},
		{"PATH_MAX", platformValue(p.pathMax)},
		{"PIPE_BUF", platformValue(p.pipeBuf)},
		{"PRIO_IO", undefined},
		{"SYMLINK_MAX", platformValue(p.symlinkMax)},
		{"SYNC_IO", undefined},
		{"_POSIX_VDISABLE", platformValue(p.vdisable)},
	}
}

// confstrEntries is Toybox's confstrs[].
func confstrEntries() []confEntry {
	return []confEntry{
		{"PATH", platformString(getconfPlatform.confstrPath)},
		// V7_ENV would be a list of environment variables for the POSIX.1-2008
		// (SUSv4) option groups; there is no C library here to ask.
		{"V7_ENV", undefined},
	}
}

// plainCharSigned reports whether the target ABI's plain char is signed. ARM,
// AArch64 and PowerPC make it unsigned, except on Apple platforms, whose ABI
// keeps it signed. CHAR_MIN and CHAR_MAX follow it, as in <limits.h>.
func plainCharSigned() bool {
	if runtime.GOOS == "darwin" {
		return true
	}
	switch runtime.GOARCH {
	case "arm", "arm64", "ppc64", "ppc64le":
		return false
	}
	return true
}

// maxSSize is the largest ssize_t, which is one bit narrower than int.
func maxSSize() int64 {
	if strconv.IntSize == 64 {
		return math.MaxInt64
	}
	return math.MaxInt32
}

// maxULong is ULONG_MAX: long is pointer sized on every platform Misa builds
// for (LP64), so it matches the Go int width.
func maxULong() string {
	if strconv.IntSize == 64 {
		return strconv.FormatUint(math.MaxUint64, 10)
	}
	return strconv.FormatUint(math.MaxUint32, 10)
}

// limitsEntries is Toybox's limits[]: the POSIX.1-2008 minimums and the C
// integer limits, which are the same everywhere Misa runs except for the width
// of long and the signedness of plain char.
func limitsEntries() []confEntry {
	var out []confEntry
	add := func(name string, v confValue) { out = append(out, confEntry{name, v}) }
	addInt := func(name string, v int64) { add(name, constant(strconv.FormatInt(v, 10))) }
	addAllInt := func(v int64, names ...string) {
		for _, n := range names {
			addInt(n, v)
		}
	}

	// POSIX.1-2008 minimum values.
	addAllInt(2, "_POSIX_AIO_LISTIO_MAX")
	addAllInt(1, "_POSIX_AIO_MAX")
	addAllInt(4096, "_POSIX_ARG_MAX")
	addAllInt(25, "_POSIX_CHILD_MAX")
	addAllInt(32, "_POSIX_DELAYTIMER_MAX")
	addAllInt(255, "_POSIX_HOST_NAME_MAX")
	addAllInt(8, "_POSIX_LINK_MAX")
	addAllInt(9, "_POSIX_LOGIN_NAME_MAX")
	addAllInt(255, "_POSIX_MAX_CANON")
	addAllInt(255, "_POSIX_MAX_INPUT")
	addAllInt(14, "_POSIX_NAME_MAX")
	addAllInt(8, "_POSIX_NGROUPS_MAX")
	addAllInt(20, "_POSIX_OPEN_MAX")
	addAllInt(256, "_POSIX_PATH_MAX")
	addAllInt(512, "_POSIX_PIPE_BUF")
	addAllInt(255, "_POSIX_RE_DUP_MAX")
	addAllInt(8, "_POSIX_RTSIG_MAX")
	addAllInt(256, "_POSIX_SEM_NSEMS_MAX")
	addAllInt(32767, "_POSIX_SEM_VALUE_MAX")
	addAllInt(32, "_POSIX_SIGQUEUE_MAX")
	addAllInt(32767, "_POSIX_SSIZE_MAX")
	addAllInt(8, "_POSIX_STREAM_MAX")
	addAllInt(255, "_POSIX_SYMLINK_MAX")
	addAllInt(8, "_POSIX_SYMLOOP_MAX")
	addAllInt(4, "_POSIX_THREAD_DESTRUCTOR_ITERATIONS")
	addAllInt(128, "_POSIX_THREAD_KEYS_MAX")
	addAllInt(64, "_POSIX_THREAD_THREADS_MAX")
	addAllInt(32, "_POSIX_TIMER_MAX")
	addAllInt(9, "_POSIX_TTY_NAME_MAX")
	addAllInt(6, "_POSIX_TZNAME_MAX")

	// <limits.h> integers, computed from Go where Go has the same width.
	if plainCharSigned() {
		addInt("CHAR_MAX", math.MaxInt8)
		addInt("CHAR_MIN", math.MinInt8)
	} else {
		addInt("CHAR_MAX", math.MaxUint8)
		addInt("CHAR_MIN", 0)
	}
	addInt("INT_MAX", math.MaxInt32)
	addInt("INT_MIN", math.MinInt32)
	addInt("SCHAR_MAX", math.MaxInt8)
	addInt("SCHAR_MIN", math.MinInt8)
	addInt("SHRT_MAX", math.MaxInt16)
	addInt("SHRT_MIN", math.MinInt16)
	addInt("SSIZE_MAX", maxSSize())
	addInt("UCHAR_MAX", math.MaxUint8)
	addInt("UINT_MAX", math.MaxUint32)
	add("ULONG_MAX", constant(maxULong()))
	addInt("USHRT_MAX", math.MaxUint16)
	addInt("CHAR_BIT", 8)
	// LONG_BIT is 8*sizeof(long); long is pointer sized on every platform Misa
	// builds for (LP64), which is what strconv.IntSize counts in bits.
	addInt("LONG_BIT", int64(strconv.IntSize))
	addInt("WORD_BIT", 32)

	// POSIX.2 (the standard's values for its utilities).
	addAllInt(99, "POSIX2_BC_BASE_MAX")
	addAllInt(2048, "POSIX2_BC_DIM_MAX")
	addAllInt(99, "POSIX2_BC_SCALE_MAX")
	addAllInt(1000, "POSIX2_BC_STRING_MAX")
	addAllInt(14, "POSIX2_CHARCLASS_NAME_MAX")
	addAllInt(2, "POSIX2_COLL_WEIGHTS_MAX")
	addAllInt(32, "POSIX2_EXPR_NEST_MAX")
	addAllInt(2048, "POSIX2_LINE_MAX")
	addAllInt(255, "POSIX2_RE_DUP_MAX")

	return out
}

// miscEntries is Toybox's others[]. LFS_CFLAGS turns on large file support for
// 32-bit kernel builds; the other two print an empty line.
func miscEntries() []confEntry {
	lfsCFlags := ""
	if strconv.IntSize == 32 {
		lfsCFlags = "-D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64"
	}
	return []confEntry{
		{"LFS_CFLAGS", constant(lfsCFlags)},
		{"LFS_LDFLAGS", constant("")},
		{"LFS_LIBS", constant("")},
	}
}

// lookupConf finds a name group by group, so an earlier group wins, as in
// Toybox's search.
func lookupConf(name string) (confKind, confEntry, bool) {
	for _, g := range confGroups {
		for _, e := range g.entries {
			if e.name == name {
				return g.kind, e, true
			}
		}
	}
	return 0, confEntry{}, false
}

func confValueString(env *confEnv, e confEntry) string {
	v, ok := e.value(env)
	if !ok {
		return "undefined"
	}
	return v
}

func getconfExtraOperand(ctx *Context, name string) int {
	fmt.Fprintf(ctx.Stderr, "getconf: extra operand '%s'\n", name)
	return 1
}

// getconfUsageError reports a usage error the way Toybox's help_exit does: the
// message, then the usage text, both on stderr, and a failing exit status.
func getconfUsageError(ctx *Context, msg string) int {
	fmt.Fprintf(ctx.Stderr, "getconf: %s\n", msg)
	fmt.Fprint(ctx.Stderr, getconfUsage)
	return 1
}

func runGetconf(ctx *Context) int {
	var showAll, list, help bool
	var operands []string
	for i := 0; i < len(ctx.Args); i++ {
		a := ctx.Args[i]
		if a == "--" {
			operands = append(operands, ctx.Args[i+1:]...)
			break
		}
		switch {
		case a == "--help":
			help = true
		case strings.HasPrefix(a, "--"):
			fmt.Fprintf(ctx.Stderr, "getconf: unknown option '%s'\n", a)
			return 1
		case len(a) > 1 && a[0] == '-':
			for _, c := range a[1:] {
				switch c {
				case 'a':
					showAll = true
				case 'l':
					list = true
				default:
					fmt.Fprintf(ctx.Stderr, "getconf: unknown option '-%c'\n", c)
					return 1
				}
			}
		default:
			operands = append(operands, a)
		}
	}
	if help {
		fmt.Fprint(ctx.Stdout, getconfUsage)
		return 0
	}
	// Toybox's spec is >2al: at most two operands.
	if len(operands) > 2 {
		return getconfExtraOperand(ctx, operands[2])
	}

	env := &confEnv{path: "/"}
	switch {
	case showAll:
		// -a wins over -l, as in Toybox.
		if len(operands) > 1 {
			return getconfExtraOperand(ctx, operands[1])
		}
		if len(operands) == 1 {
			env.path = operands[0]
		}
		for _, g := range confGroups {
			for _, e := range g.entries {
				fmt.Fprintf(ctx.Stdout, "%-34s %s\n", e.name, confValueString(env, e))
			}
		}
		return 0
	case list:
		for _, g := range confGroups {
			fmt.Fprintf(ctx.Stdout, "%s\n", g.title)
			for _, e := range g.entries {
				fmt.Fprintf(ctx.Stdout, "  %s\n", e.name)
			}
		}
		return 0
	}

	if len(operands) == 0 {
		fmt.Fprint(ctx.Stdout, getconfUsage)
		return 0
	}

	name := operands[0]
	// Toybox accepts CS_PATH, the name autoconf uses, for the confstr PATH.
	if name == "CS_PATH" {
		name = "PATH"
	}
	kind, entry, ok := lookupConf(name)
	if !ok {
		fmt.Fprintf(ctx.Stderr, "getconf: bad '%s'\n", name)
		return 1
	}
	if kind == confPathconf {
		if len(operands) != 2 {
			return getconfUsageError(ctx, name+" needs a path")
		}
		env.path = operands[1]
	} else if len(operands) != 1 {
		return getconfUsageError(ctx, name+" does not take a path")
	}
	fmt.Fprintf(ctx.Stdout, "%s\n", confValueString(env, entry))
	return 0
}
