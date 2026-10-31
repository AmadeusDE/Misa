// ps -o column table and formatting.
//
// Field names, default titles, declared widths (positive: right justified,
// negative: left justified), the default/-f/-l layouts and the "[comm]"
// fallback for an unreadable command line follow toybox toys/posix/ps.c
// (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb). Header names and
// the pid/tty/time/cmd default match uutils/procps ps (MIT, commit
// e697667b3f758803e38d8ab4c8f912171beca602). No other implementation was
// consulted; see docs/CREDITS.md.

package builtins

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

// psEnv is the snapshot-wide context a column needs to render.
type psEnv struct {
	Now  time.Time
	Boot time.Time
	// BootKnown is false when the uptime file was unreadable, so the start and
	// elapsed columns report "?" instead of inventing a boot time.
	BootKnown bool
	User      func(uid uint32) string
}

// psField is one -o column type.
type psField struct {
	name    string // canonical lowercase name accepted by -o
	title   string // default header text
	width   int    // declared width; >0 right justified, <0 left justified
	numeric bool   // sort by the numeric key, and never truncate the value
	help    string // one-line description for -o help
}

// psFieldTable lists the supported columns in help order.
var psFieldTable = []psField{
	{"pid", "PID", 6, true, "process id"},
	{"ppid", "PPID", 6, true, "parent process id"},
	{"pgid", "PGID", 6, true, "process group id"},
	{"sid", "SID", 6, true, "session id"},
	{"uid", "UID", 5, true, "effective user id"},
	{"user", "USER", -8, false, "effective user name"},
	{"gid", "GID", 5, true, "effective group id"},
	{"tty", "TTY", -8, false, "controlling terminal, ? for none"},
	{"stat", "STAT", -5, false, "state plus <, N, s, l and + flags"},
	{"s", "S", -1, false, "one-letter process state"},
	{"ni", "NI", 3, true, "niceness"},
	{"pri", "PRI", 3, true, "39 minus the kernel priority"},
	{"nlwp", "NLWP", 4, true, "thread count"},
	{"rss", "RSS", 6, true, "resident set size in KiB"},
	{"vsz", "VSZ", 7, true, "virtual size in KiB"},
	{"stime", "STIME", 5, true, "start time (HH:MM)"},
	{"time", "TIME", 8, true, "CPU time ([[dd-]hh:]mm:ss)"},
	{"etime", "ELAPSED", 8, true, "time since start ([[dd-]hh:]mm:ss)"},
	{"comm", "COMM", -15, false, "executable name, [comm] when unavailable"},
	{"args", "ARGS", -27, false, "command line, [comm] when unavailable"},
	{"cmd", "CMD", -15, false, "thread name"},
}

// psFieldAliases maps accepted alternative spellings to canonical names.
var psFieldAliases = map[string]string{
	"nice":    "ni",
	"elapsed": "etime",
	"uname":   "user",
	"vsize":   "vsz",
}

// psColumn is one selected column: a field plus its title and width.
type psColumn struct {
	field   psField
	title   string
	width   int
	reverse bool // sort descending; set by a leading '-' in a field list
}

// lookupPSField resolves a field name case-insensitively, following aliases.
func lookupPSField(name string) (psField, bool) {
	name = strings.ToLower(name)
	if canonical, ok := psFieldAliases[name]; ok {
		name = canonical
	}
	for _, f := range psFieldTable {
		if f.name == name {
			return f, true
		}
	}
	return psField{}, false
}

// parsePSColumns parses a comma separated field list of
// [FIELD][:width][=title] items.
func parsePSColumns(list string) ([]psColumn, error) {
	var cols []psColumn
	for _, item := range strings.Split(list, ",") {
		if strings.TrimSpace(item) == "" {
			return nil, fmt.Errorf("empty field in %q", list)
		}
		spec, title, hasTitle := strings.Cut(item, "=")
		name, width, _ := strings.Cut(spec, ":")
		reverse := false
		if len(name) > 1 && (name[0] == '-' || name[0] == '+') {
			reverse = name[0] == '-'
			name = name[1:]
		}
		field, ok := lookupPSField(name)
		if !ok {
			return nil, fmt.Errorf("unknown field '%s'", name)
		}
		col := psColumn{field: field, title: field.title, width: field.width, reverse: reverse}
		if hasTitle {
			col.title = title
		}
		if width != "" {
			n, err := strconv.Atoi(width)
			if err != nil || n == 0 || n < -999 || n > 999 {
				return nil, fmt.Errorf("bad field width '%s'", width)
			}
			if field.width < 0 && n > 0 {
				n = -n
			}
			col.width = n
		}
		cols = append(cols, col)
	}
	return cols, nil
}

// psFieldHelp renders the -o help text.
func psFieldHelp() string {
	var b strings.Builder
	b.WriteString("Field types for ps -o (NAME[:width][=title]):\n\n")
	for _, f := range psFieldTable {
		fmt.Fprintf(&b, "  %-8s %s\n", f.name, f.help)
	}
	b.WriteString("\nAliases: nice=ni, elapsed=etime, uname=user, vsize=vsz\n")
	return b.String()
}

// value renders one column for one process. The key is the numeric sort value
// (seconds for the clock columns); columns that are not numeric return 0 and
// sort by their text.
func (f psField) value(p proc.Process, env psEnv) (text string, key int64) {
	switch f.name {
	case "pid":
		return strconv.Itoa(p.PID), int64(p.PID)
	case "ppid":
		return strconv.Itoa(p.PPID), int64(p.PPID)
	case "pgid":
		return strconv.Itoa(p.PGID), int64(p.PGID)
	case "sid":
		return strconv.Itoa(p.SID), int64(p.SID)
	case "uid":
		return strconv.FormatUint(uint64(p.UID), 10), int64(p.UID)
	case "user":
		return psUserName(env, p.UID), int64(p.UID)
	case "gid":
		return strconv.FormatUint(uint64(p.GID), 10), int64(p.GID)
	case "tty":
		return p.TTYName, 0
	case "stat":
		return psStatString(p), 0
	case "s":
		return string(p.State), 0
	case "ni":
		return strconv.Itoa(int(p.Nice)), int64(p.Nice)
	case "pri":
		return strconv.Itoa(39 - int(p.Priority)), int64(39 - p.Priority)
	case "nlwp":
		return strconv.Itoa(p.Threads), int64(p.Threads)
	case "rss":
		kb := int64(p.RSS) * int64(psPageSize) / 1024
		return strconv.FormatInt(kb, 10), kb
	case "vsz":
		kb := int64(p.VSZ / 1024)
		return strconv.FormatInt(kb, 10), kb
	case "stime":
		if !env.BootKnown {
			return "?", 0
		}
		start := psStartTime(p, env)
		return start.Format("15:04"), start.Unix()
	case "time":
		secs := int64(p.CPUTicks() / proc.ClockTicks)
		return psClock(secs, 3), secs
	case "etime":
		if !env.BootKnown {
			return "?", 0
		}
		secs := int64(env.Now.Sub(psStartTime(p, env)) / time.Second)
		if secs < 0 {
			secs = 0
		}
		return psClock(secs, 2), secs
	case "comm":
		if p.Exe != "" {
			return path.Base(p.Exe), 0
		}
		return "[" + p.Comm + "]", 0
	case "args":
		if len(p.Argv) > 0 {
			return strings.Join(p.Argv, " "), 0
		}
		return "[" + p.Comm + "]", 0
	case "cmd":
		return p.Comm, 0
	}
	return "", 0
}

// psPageSize is the page size RSS is reported in; a variable so tests can pin it.
var psPageSize = uint64(4096)

func psUserName(env psEnv, uid uint32) string {
	if env.User != nil {
		if name := env.User(uid); name != "" {
			return name
		}
	}
	return strconv.FormatUint(uint64(uid), 10)
}

// psStartTime is the wall clock moment the process started.
func psStartTime(p proc.Process, env psEnv) time.Time {
	return env.Boot.Add(time.Duration(p.StartTicks) * time.Second / proc.ClockTicks)
}

func psStatString(p proc.Process) string {
	var b strings.Builder
	b.WriteByte(p.State)
	switch {
	case p.Nice < 0:
		b.WriteByte('<')
	case p.Nice > 0:
		b.WriteByte('N')
	}
	if p.SID == p.PID {
		b.WriteByte('s')
	}
	if p.Threads > 1 {
		b.WriteByte('l')
	}
	if p.Tpgid == p.PID {
		b.WriteByte('+')
	}
	return b.String()
}

// psClock formats seconds as [[dd-]hh:]mm:ss with at least minFields fields.
func psClock(secs, minFields int64) string {
	if secs < 0 {
		secs = 0
	}
	days := secs / 86400
	secs %= 86400
	hours := secs / 3600
	mins := secs % 3600 / 60
	seconds := secs % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d-%02d:%02d:%02d", days, hours, mins, seconds)
	case minFields >= 3 || hours > 0:
		return fmt.Sprintf("%02d:%02d:%02d", hours, mins, seconds)
	}
	return fmt.Sprintf("%02d:%02d", mins, seconds)
}

// psColumnWidth computes the display widths for a table: each column is at
// least its declared width and grows to fit its widest value. When limit is
// positive the total is clamped to it by narrowing columns from the last one
// first, down to a single character, so piped output stays complete while a
// terminal listing truncates instead of wrapping.
func psColumnWidth(cols []psColumn, rows [][]string) []int {
	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = col.width
		if w := psTextWidth(col.title); w > psAbs(widths[i]) {
			widths[i] = psSigned(widths[i], w)
		}
		for _, row := range rows {
			if i >= len(row) {
				continue
			}
			if w := psTextWidth(row[i]); w > psAbs(widths[i]) {
				widths[i] = psSigned(widths[i], w)
			}
		}
	}
	return widths
}

func psAbs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// psSigned returns width n with the justification of base.
func psSigned(base, n int) int {
	if base < 0 {
		return -n
	}
	return n
}

func psTextWidth(s string) int { return len([]rune(s)) }

// psFit narrows widths so their sum plus separators fits limit. Numbers are
// never narrowed below their natural width, so a value is never cut; only the
// text columns shrink, from the last one backwards. A limit that is not
// positive means no narrowing at all.
func psFit(cols []psColumn, widths []int, limit int) []int {
	if limit <= 0 {
		return widths
	}
	total := len(widths) - 1
	for _, w := range widths {
		total += psAbs(w)
	}
	for i := len(widths) - 1; i >= 0 && total > limit; i-- {
		if cols[i].field.numeric {
			continue
		}
		for psAbs(widths[i]) > 1 && total > limit {
			widths[i] = psSigned(widths[i], psAbs(widths[i])-1)
			total--
		}
	}
	return widths
}

// formatPSRows renders the header (unless every title is empty) and rows.
// Numeric columns are right justified, string columns left justified; a
// string value that does not fit is cut and its last character replaced by
// '+' so truncation is visible.
func formatPSRows(cols []psColumn, widths []int, rows [][]string) string {
	var b strings.Builder
	if !psHeaderSuppressed(cols) {
		cells := make([]string, len(cols))
		for i, col := range cols {
			cells[i] = psCell(col.title, widths[i], col.field.numeric, false)
		}
		b.WriteString(strings.TrimRight(strings.Join(cells, " "), " "))
		b.WriteByte('\n')
	}
	for _, row := range rows {
		cells := make([]string, len(row))
		for i := range row {
			numeric := i < len(cols) && cols[i].field.numeric
			cells[i] = psCell(row[i], widths[i], numeric, true)
		}
		b.WriteString(strings.TrimRight(strings.Join(cells, " "), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// psHeaderSuppressed reports whether every column title is empty, which is how
// "-o field=" suppresses the header line.
func psHeaderSuppressed(cols []psColumn) bool {
	for _, col := range cols {
		if col.title != "" {
			return false
		}
	}
	return true
}

// psCell pads one value to width. A value that must stay exact (a number) is
// never cut; a text value that does not fit loses its last character to a '+'
// marker, and a header title is cut without one.
func psCell(text string, width int, numeric, marker bool) string {
	runes := []rune(text)
	w := psAbs(width)
	if len(runes) > w {
		switch {
		case numeric:
			w = len(runes) // keep the exact value
		case marker && w > 1:
			return string(runes[:w-1]) + "+"
		default:
			runes = runes[:w]
		}
	}
	pad := strings.Repeat(" ", w-len(runes))
	if width > 0 {
		return pad + string(runes)
	}
	return string(runes) + pad
}
