// ps — report a snapshot of the current processes.
//
// The option set, the default/-f/-l layouts, the -o field grammar
// (NAME[:width][=title]), stat parsing and the "[comm]" fallback follow toybox
// toys/posix/ps.c (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb).
// Header names and the default pid/tty/time/cmd selection match uutils/procps
// ps (MIT, commit e697667b3f758803e38d8ab4c8f912171beca602). Deliberately
// unsupported there and here: -M/-O/-T/-Z/-n/-P/-s. This is a native Go
// implementation; see docs/CREDITS.md.

package builtins

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

const psSpec = "A(all)eadflwo(fields):p(pid):u:U:g:G:t(tty):k(sort):"

var psSpecParsed *Spec

func init() {
	var err error
	psSpecParsed, err = ParseSpec(psSpec)
	if err != nil {
		panic("builtins: bad ps spec: " + err.Error())
	}
	Register(Command{Name: "ps", Run: runPs})
}

// Seams for tests: the procfs root, the clock, the terminal ps itself runs on
// and the account lookups.
var (
	psProcRoot   = ""
	psNow        = time.Now
	psOwnTTY     = defaultPSOwnTTY
	psLookupUser = defaultPSLookupUser
)

func runPs(ctx *Context) int {
	flags, err := psSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "ps: %v\nTry 'ps --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, psUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "ps (Misa) 0.1.0\n")
		return 0
	}

	for _, list := range flags.AllStrings("o") {
		if strings.EqualFold(strings.TrimSpace(list), "help") {
			fmt.Fprint(ctx.Stdout, psFieldHelp())
			return 0
		}
	}
	cols, err := psOutputColumns(flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "ps: %v\nTry 'ps --help' for more information.\n", err)
		return 1
	}
	sel, err := psParseSelection(flags)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "ps: %v\nTry 'ps --help' for more information.\n", err)
		return 1
	}

	reader := proc.New(psProcRoot)
	snapshot, errs := reader.Snapshot(ctx.CancellationContext(), proc.Options{
		Argv: psHasColumn(cols, "args"),
		Exe:  psHasColumn(cols, "comm"),
	})
	if len(errs) > 0 {
		fmt.Fprintf(ctx.Stderr, "ps: %v\n", errs[0])
		return 1
	}

	now := psNow()
	env := psEnv{Now: now, User: psLookupUser}
	if uptime, err := reader.Uptime(); err == nil {
		env.Boot, env.BootKnown = now.Add(-uptime), true
	}

	procs := psSelect(snapshot, sel, flags)
	if kcols, err := psSortColumns(flags); err != nil {
		fmt.Fprintf(ctx.Stderr, "ps: %v\nTry 'ps --help' for more information.\n", err)
		return 1
	} else if len(kcols) > 0 {
		psSort(procs, kcols, env)
	}
	if len(procs) == 0 {
		return 1
	}

	rows := make([][]string, len(procs))
	for i, p := range procs {
		rows[i] = make([]string, len(cols))
		for j, col := range cols {
			rows[i][j], _ = col.field.value(p, env)
		}
	}
	limit := 0
	if !flags.Bool("w") && ctx.StdoutTTY {
		limit = ctx.Width
		if limit <= 0 {
			limit = 80
		}
	}
	widths := psFit(cols, psColumnWidth(cols, rows), limit)
	if _, err := fmt.Fprint(ctx.Stdout, formatPSRows(cols, widths, rows)); err != nil {
		return 1
	}
	return 0
}

// psUsage is the --help text: the supported options, the supported -o fields
// and an explicit list of the options that are refused instead of faked.
func psUsage() string {
	return "Usage: ps [-Aadeflw] [-o FIELD,...] [-p PID,...] [-u|-U USER,...]\n" +
		"          [-g|-G GROUP,...] [-t TTY,...] [-k FIELD,...]\n\n" +
		"List processes. Without a selection, processes sharing ps's own\n" +
		"controlling terminal are listed.\n\n" +
		"Selection:\n" +
		"  -A, -e            all processes\n" +
		"  -a                processes with a terminal, except session leaders\n" +
		"  -d                all processes except session leaders\n" +
		"  -p, --pid PIDS    process ids, comma separated\n" +
		"  -u USERS          effective user id or name\n" +
		"  -U USERS          real user id or name\n" +
		"  -g GROUPS         effective group id or name\n" +
		"  -G GROUPS         real group id or name\n" +
		"  -t, --tty TTYS    controlling terminals (pts/1, tty1, /dev/pts/1)\n\n" +
		"Output:\n" +
		"  -f                full listing\n" +
		"  -l                long listing\n" +
		"  -o, --fields LIST columns to show, NAME[:width][=title];\n" +
		"                    '-o help' lists the field types\n" +
		"  -k, --sort LIST   sort by fields, -FIELD reverses\n" +
		"  -w                wide: never truncate columns\n\n" +
		"Unsupported (rejected, never faked): -M, -n, -O, -P, -s, -T, -Z, and\n" +
		"thread listing. Exit status: 0 listed, 1 nothing matched or bad usage.\n"
}

// psOutputColumns builds the column list from -o, or from -f/-l, or the default.
func psOutputColumns(flags *ParseResult) ([]psColumn, error) {
	if lists := flags.AllStrings("o"); len(lists) > 0 {
		var cols []psColumn
		for _, list := range lists {
			parsed, err := parsePSColumns(list)
			if err != nil {
				return nil, err
			}
			cols = append(cols, parsed...)
		}
		return cols, nil
	}
	switch {
	case flags.Bool("f"):
		return mustPSColumns("user,pid,ppid,stime,tty,time,args=CMD"), nil
	case flags.Bool("l"):
		return mustPSColumns("s,uid,pid,ppid,pri,ni,nlwp,vsz,tty,time,cmd"), nil
	}
	return mustPSColumns("pid,tty,time,cmd"), nil
}

func mustPSColumns(list string) []psColumn {
	cols, err := parsePSColumns(list)
	if err != nil {
		panic("builtins: bad ps field list " + list + ": " + err.Error())
	}
	return cols
}

func psHasColumn(cols []psColumn, name string) bool {
	for _, col := range cols {
		if col.field.name == name {
			return true
		}
	}
	return false
}

// psSortColumns parses the -k/--sort field list.
func psSortColumns(flags *ParseResult) ([]psColumn, error) {
	lists := flags.AllStrings("k")
	if len(lists) == 0 {
		return nil, nil
	}
	var cols []psColumn
	for _, list := range lists {
		parsed, err := parsePSColumns(list)
		if err != nil {
			return nil, err
		}
		cols = append(cols, parsed...)
	}
	return cols, nil
}

// psSelection holds the explicit process selectors. Any selector restricts the
// listing to the processes that match one of them.
type psSelection struct {
	used              bool
	pids, uids, ruids []int
	gids, rgids       []int
	ttys              []string
}

func psParseSelection(flags *ParseResult) (psSelection, error) {
	var sel psSelection
	for _, value := range flags.AllStrings("p") {
		ids, err := psParseIDs(value, "process id")
		if err != nil {
			return sel, err
		}
		sel.pids = append(sel.pids, ids...)
	}
	// Extra operands are more -p values, as Toybox does.
	for _, arg := range flags.Operands {
		ids, err := psParseIDs(arg, "process id")
		if err != nil {
			return sel, err
		}
		sel.pids = append(sel.pids, ids...)
	}
	users := []struct {
		key   string
		dst   *[]int
		group bool
	}{
		{"u", &sel.uids, false},
		{"U", &sel.ruids, false},
		{"g", &sel.gids, true},
		{"G", &sel.rgids, true},
	}
	for _, u := range users {
		for _, value := range flags.AllStrings(u.key) {
			ids, err := psParseAccounts(value, u.group)
			if err != nil {
				return sel, err
			}
			*u.dst = append(*u.dst, ids...)
		}
	}
	for _, value := range flags.AllStrings("t") {
		for _, name := range strings.Split(value, ",") {
			name = strings.TrimPrefix(strings.TrimSpace(name), "/dev/")
			if name == "" {
				return sel, fmt.Errorf("missing terminal name in %q", value)
			}
			sel.ttys = append(sel.ttys, name)
		}
	}
	sel.used = len(sel.pids)+len(sel.uids)+len(sel.ruids)+len(sel.gids)+len(sel.rgids)+len(sel.ttys) > 0
	return sel, nil
}

func psParseIDs(list, what string) ([]int, error) {
	var ids []int
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("missing %s in %q", what, list)
		}
		id, err := strconv.Atoi(item)
		if err != nil || id < 0 {
			return nil, fmt.Errorf("invalid %s '%s'", what, item)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func psParseAccounts(list string, group bool) ([]int, error) {
	var ids []int
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("missing %s in %q", psAccountWord(group), list)
		}
		if id, err := strconv.Atoi(item); err == nil && id >= 0 {
			ids = append(ids, id)
			continue
		}
		id, ok := psResolveAccount(item, group)
		if !ok {
			return nil, fmt.Errorf("invalid %s name '%s'", psAccountWord(group), item)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func psAccountWord(group bool) string {
	if group {
		return "group"
	}
	return "user"
}

func psResolveAccount(name string, group bool) (int, bool) {
	if group {
		g, err := user.LookupGroup(name)
		if err != nil {
			return 0, false
		}
		id, err := strconv.Atoi(g.Gid)
		return id, err == nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, false
	}
	id, err := strconv.Atoi(u.Uid)
	return id, err == nil
}

// psSelect applies the selectors, or else the -A/-a/-d/default filter.
func psSelect(procs []proc.Process, sel psSelection, flags *ParseResult) []proc.Process {
	var out []proc.Process
	for _, p := range procs {
		if sel.used {
			if psMatches(p, sel) {
				out = append(out, p)
			}
			continue
		}
		if psDefaultMatch(p, flags) {
			out = append(out, p)
		}
	}
	return out
}

func psMatches(p proc.Process, sel psSelection) bool {
	return containsInt(sel.pids, p.PID) ||
		containsInt(sel.uids, int(p.UID)) ||
		containsInt(sel.ruids, int(p.RUID)) ||
		containsInt(sel.gids, int(p.GID)) ||
		containsInt(sel.rgids, int(p.RGID)) ||
		containsString(sel.ttys, p.TTYName)
}

func psDefaultMatch(p proc.Process, flags *ParseResult) bool {
	switch {
	case flags.Bool("A") || flags.Bool("all") || flags.Bool("e"):
		return true
	case flags.Bool("a"):
		return p.TTY != 0 && p.SID != p.PID
	case flags.Bool("d"):
		return p.SID != p.PID
	}
	return p.TTY == psOwnTTY()
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// psSort orders the processes by the -k columns, stably, falling back to the
// PID so the output does not depend on the order the procfs listing returned.
func psSort(procs []proc.Process, cols []psColumn, env psEnv) {
	type keyed struct {
		text string
		num  int64
	}
	keys := make(map[int][]keyed, len(procs))
	for _, p := range procs {
		row := make([]keyed, len(cols))
		for i, col := range cols {
			text, num := col.field.value(p, env)
			row[i] = keyed{text, num}
		}
		keys[p.PID] = row
	}
	sort.SliceStable(procs, func(i, j int) bool {
		a, b := keys[procs[i].PID], keys[procs[j].PID]
		for k, col := range cols {
			cmp := 0
			if col.field.numeric {
				switch {
				case a[k].num < b[k].num:
					cmp = -1
				case a[k].num > b[k].num:
					cmp = 1
				}
			}
			if cmp == 0 {
				cmp = strings.Compare(a[k].text, b[k].text)
			}
			if cmp != 0 {
				if col.reverse {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return procs[i].PID < procs[j].PID
	})
}

var psUserCache = struct {
	sync.Mutex
	names map[uint32]string
}{names: map[uint32]string{}}

// defaultPSLookupUser resolves a uid to a name, or "" when there is none (the
// column then shows the numeric id).
func defaultPSLookupUser(uid uint32) string {
	psUserCache.Lock()
	defer psUserCache.Unlock()
	if name, ok := psUserCache.names[uid]; ok {
		return name
	}
	name := ""
	if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
		name = u.Username
	}
	psUserCache.names[uid] = name
	return name
}

// psPageSizeForPlatform keeps RSS in KiB without depending on the page size of
// a fixture tree; the kernel reports RSS in pages.
func init() {
	psPageSize = uint64(os.Getpagesize())
}
