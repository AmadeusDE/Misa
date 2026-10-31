// uptime — tell how long the system has been running.
//
// The default line (leading time, up duration, load averages), the -s boot time
// format ("%F %T") and the option letters follow toybox toys/other/uptime.c
// (0BSD, commit 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb). The pretty form of -p
// (zero units left out, no load average) follows uutils coreutils uptime (MIT,
// commit fee61ff5d4f87e0f912a52e639942b5f7ad58663). Both references print a
// logged-in user count from utmp; Misa has no utmp reader, so that field is
// left out rather than guessed. Uptime comes from internal/proc (Linux/Android);
// other platforms are reported as unsupported. See docs/CREDITS.md.

package builtins

import (
	"fmt"
	"strings"
	"time"

	"github.com/AmadeusDE/Misa/internal/proc"
)

const uptimeSpec = "p(pretty)s(since)"

var uptimeSpecParsed *Spec

func init() {
	var err error
	uptimeSpecParsed, err = ParseSpec(uptimeSpec)
	if err != nil {
		panic("builtins: bad uptime spec: " + err.Error())
	}
	Register(Command{Name: "uptime", Run: runUptime})
}

// Seams for tests: the procfs root and the clock.
var (
	uptimeProcRoot = ""
	uptimeNow      = time.Now
)

func runUptime(ctx *Context) int {
	flags, err := uptimeSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintf(ctx.Stderr, "uptime: %v\nTry 'uptime --help' for more information.\n", err)
		return 1
	}
	if flags.HelpRequested {
		fmt.Fprint(ctx.Stdout, uptimeUsage())
		return 0
	}
	if flags.VersionRequested {
		fmt.Fprintf(ctx.Stdout, "%s (Misa) 0.1.0\n", ctx.CmdName)
		return 0
	}
	if len(flags.Operands) > 0 {
		fmt.Fprintf(ctx.Stderr, "uptime: extra operand '%s'\nTry 'uptime --help' for more information.\n", flags.Operands[0])
		return 1
	}

	reader := proc.New(uptimeProcRoot)
	now := uptimeNow()
	up, err := reader.Uptime()
	if err != nil {
		printProcError(ctx, "uptime", "the system uptime", err)
		return 1
	}

	if flags.Bool("s") {
		fmt.Fprintln(ctx.Stdout, uptimeBootTime(reader, now, up).Format("2006-01-02 15:04:05"))
		return 0
	}
	if flags.Bool("p") {
		fmt.Fprintf(ctx.Stdout, "up %s\n", uptimePretty(up))
		return 0
	}
	fmt.Fprintln(ctx.Stdout, uptimeLine(reader, now, up))
	return 0
}

func uptimeUsage() string {
	return "Usage: uptime [-p|-s]\n\n" +
		"Tell the current time, how long the system has been running, and the\n" +
		"system load averages for the past 1, 5 and 15 minutes.\n\n" +
		"  -p, --pretty  print the uptime in a human readable form and nothing else\n" +
		"  -s, --since   print the date and time the system booted\n\n" +
		"There is no logged-in user count: Misa has no utmp reader.\n" +
		"Exit status: 0 printed, 1 bad usage or no readable /proc/uptime.\n"
}

// uptimeBootTime reports when the system booted. The kernel's btime is the
// authoritative value; without it (or without /proc/stat) the boot time is the
// current time minus the uptime, which is what the references do.
func uptimeBootTime(reader *proc.Reader, now time.Time, up time.Duration) time.Time {
	if st, err := reader.Stat(); err == nil && st.BootTime > 0 {
		return time.Unix(st.BootTime, 0)
	}
	return now.Add(-up)
}

// uptimeLine builds the default one-liner: " 09:00:00 up 2 days,  3:04,  load
// average: 0.10, 0.20, 0.30". Without a readable loadavg the line ends after the
// duration instead of printing an invented average.
func uptimeLine(reader *proc.Reader, now time.Time, up time.Duration) string {
	seconds := int64(up / time.Second)
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	var b strings.Builder
	fmt.Fprintf(&b, " %s up ", now.Format("15:04:05"))
	if days > 0 {
		fmt.Fprintf(&b, "%d day%s, ", days, uptimePlural(days))
	}
	if hours > 0 {
		fmt.Fprintf(&b, "%2d:%02d, ", hours, minutes)
	} else {
		fmt.Fprintf(&b, "%d min, ", minutes)
	}
	if la, err := reader.Loadavg(); err == nil {
		fmt.Fprintf(&b, " load average: %.2f, %.2f, %.2f", la.One, la.Five, la.Fifteen)
	} else {
		return strings.TrimSuffix(b.String(), ", ")
	}
	return b.String()
}

// uptimePretty builds the -p form: "2 days, 3 hours, 4 minutes". Zero units are
// left out, and a machine up for less than a minute still says so.
func uptimePretty(up time.Duration) string {
	seconds := int64(up / time.Second)
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d day%s", days, uptimePlural(days)))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d hour%s", hours, uptimePlural(hours)))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d minute%s", minutes, uptimePlural(minutes)))
	}
	return strings.Join(parts, ", ")
}

func uptimePlural(n int64) string {
	if n == 1 {
		return ""
	}
	return "s"
}
