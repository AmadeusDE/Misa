package builtins

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const dateSpec = "u(utc)(universal)(uct)(uni)(u)d(date):r(reference):R(rfc-email)(rfc-822)(rfc-2822)I(iso-8601):?s(set):f(file):(debug)(resolution)"

var dateSpecParsed *Spec

var tzAbbrevOffsets = map[string]int{
	"UTC": 0, "GMT": 0, "Z": 0, "UT": 0,
	"EST": -5 * 3600, "EDT": -4 * 3600,
	"CST": -6 * 3600, "CDT": -5 * 3600,
	"MST": -7 * 3600, "MDT": -6 * 3600,
	"PST": -8 * 3600, "PDT": -7 * 3600,
	"KST": 9 * 3600, "JST": 9 * 3600,
	"CET": 1 * 3600, "CEST": 2 * 3600,
	"EET": 2 * 3600, "EEST": 3 * 3600,
	"BST": 1 * 3600, "HST": -10 * 3600,
	"AKST": -9 * 3600, "AKDT": -8 * 3600,
	"AEST": 10 * 3600, "AEDT": 11 * 3600,
	"ACST": 95 * 1800, "ACDT": 105 * 1800,
	"AWST": 8 * 3600,
	"MEST": 2 * 3600, "MET": 1 * 3600,
	"MEZ": 1 * 3600, "MESZ": 2 * 3600, "IST": 19800,
}

var militaryTZOffsets = map[byte]int{
	'A': 1 * 3600, 'B': 2 * 3600, 'C': 3 * 3600, 'D': 4 * 3600,
	'E': 5 * 3600, 'F': 6 * 3600, 'G': 7 * 3600, 'H': 8 * 3600,
	'I': 9 * 3600, 'K': 10 * 3600, 'L': 11 * 3600, 'M': 12 * 3600,
	'N': -1 * 3600, 'O': -2 * 3600, 'P': -3 * 3600, 'Q': -4 * 3600,
	'R': -5 * 3600, 'S': -6 * 3600, 'T': -7 * 3600, 'U': -8 * 3600,
	'V': -9 * 3600, 'W': -10 * 3600, 'X': -11 * 3600, 'Y': -12 * 3600,
	'Z': 0,
}

func init() {
	var err error
	dateSpecParsed, err = ParseSpec(dateSpec)
	if err != nil {
		panic("builtins: bad date spec: " + err.Error())
	}
	Register(Command{Name: "date", Run: runDate})
}

func runDate(ctx *Context) int {
	flags, err := dateSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "date:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, dateSpecParsed) {
		return 0
	}

	// -u applies to interpretation of unzoned input as well as output.
	if flags.Bool("u") {
		local := *ctx
		local.Env = append([]string{"TZ=UTC"}, ctx.Environ()...)
		ctx = &local
	}

	if flags.Bool("resolution") {
		if _, hasD := flags.String("d"); hasD {
			fmt.Fprintln(ctx.Stderr, "date: the options to specify dates for printing are mutually exclusive")
			fmt.Fprintln(ctx.Stderr, "Try 'date --help' for more information.")
			return 1
		}
		if _, hasR := flags.String("r"); hasR {
			fmt.Fprintln(ctx.Stderr, "date: the options to specify dates for printing are mutually exclusive")
			fmt.Fprintln(ctx.Stderr, "Try 'date --help' for more information.")
			return 1
		}
		if _, hasS := flags.String("s"); hasS {
			fmt.Fprintln(ctx.Stderr, "date: the options to specify dates for printing are mutually exclusive")
			fmt.Fprintln(ctx.Stderr, "Try 'date --help' for more information.")
			return 1
		}
	}

	if filePath, ok := flags.String("f"); ok {
		var r io.Reader
		if filePath == "-" {
			r = ctx.Stdin
		} else {
			rp := resolvePath(ctx, filePath)
			fi, err := os.Stat(rp)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "date: %s: No such file or directory\n", filePath)
				return 1
			}
			if fi.IsDir() {
				fmt.Fprintf(ctx.Stderr, "date: expected file, got directory '%s'\n", filePath)
				return 1
			}
			f, err := os.Open(rp)
			if err != nil {
				fmt.Fprintf(ctx.Stderr, "date: cannot open '%s': %v\n", filePath, err)
				return 1
			}
			defer f.Close()
			r = f
		}
		scanner := bufio.NewScanner(r)
		exitCode := 0
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")
			if line == "" {
				continue
			}
			t, ok := parseDateStr(ctx, line)
			if !ok {
				fmt.Fprintf(ctx.Stderr, "date: invalid date '%s'\n", line)
				exitCode = 1
				continue
			}
			if flags.Bool("u") {
				t = t.UTC()
			} else {
				t = t.In(dateLocation(ctx, t))
			}
			formatAndPrintDate(ctx, flags, t)
		}
		return exitCode
	}

	now := time.Now()

	if flags.Bool("resolution") {
		now = time.Unix(0, 0).UTC()
	} else if refFile, ok := flags.String("r"); ok {
		if _, hasDate := flags.String("d"); hasDate {
			fmt.Fprintln(ctx.Stderr, "date: the options to specify dates for printing are mutually exclusive")
			fmt.Fprintln(ctx.Stderr, "Try 'date --help' for more information.")
			return 1
		}
		if refFile == "" {
			fmt.Fprintln(ctx.Stderr, "date: '': No such file or directory")
			return 1
		}
		rp := resolvePath(ctx, refFile)
		fi, err := os.Stat(rp)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "date: %v\n", err)
			return 1
		}
		now = fi.ModTime()
	} else if dateStr, ok := flags.String("d"); ok {
		if t, ok := parseDateStr(ctx, dateStr); ok {
			now = t
		} else {
			fmt.Fprintf(ctx.Stderr, "date: invalid date '%s'\n", dateStr)
			return 1
		}
	} else if dateStr, ok := flags.String("s"); ok {
		if t, ok := parseDateStr(ctx, dateStr); ok {
			now = t
		} else {
			fmt.Fprintf(ctx.Stderr, "date: invalid date '%s'\n", dateStr)
			return 1
		}
	} else {
		for _, op := range flags.Operands {
			if op == "" {
				fmt.Fprintf(ctx.Stderr, "date: invalid date ''\n")
				return 1
			}
		}
		if len(flags.Operands) > 0 && !strings.HasPrefix(flags.Operands[0], "+") {
			if len(flags.Operands) > 1 && !strings.HasPrefix(flags.Operands[1], "+") {
				fmt.Fprintf(ctx.Stderr, "date: extra operand '%s'\n", flags.Operands[1])
				return 1
			}
			if flags.Operands[0] == "-" {
				fmt.Fprintf(ctx.Stderr, "date: invalid date '-'\n")
				return 1
			}
			if t, ok := parseDateStr(ctx, flags.Operands[0]); ok {
				now = t
			} else {
				fmt.Fprintf(ctx.Stderr, "date: invalid date '%s'\n", flags.Operands[0])
				return 1
			}
		}
	}

	if flags.Bool("u") || flags.Bool("resolution") {
		now = now.UTC()
	} else {
		now = now.In(dateLocation(ctx, now))
	}

	return formatAndPrintDate(ctx, flags, now)
}

func formatAndPrintDate(ctx *Context, flags *ParseResult, now time.Time) int {
	if flags.Bool("R") {
		fmt.Fprintln(ctx.Stdout, now.Format(time.RFC1123Z))
		return 0
	}
	if flags.Bool("I") {
		timespec, _ := flags.String("I")
		layout, ok := iso8601Layout(timespec)
		if !ok {
			fmt.Fprintf(ctx.Stderr, "date: invalid argument '%s' for '--iso-8601'\n", timespec)
			return 1
		}
		out := now.Format(layout)
		if timespec == "ns" {
			out = strings.Replace(out, ".", ",", 1)
		}
		fmt.Fprintln(ctx.Stdout, out)
		return 0
	}

	for _, op := range flags.Operands {
		if strings.HasPrefix(op, "+") {
			formatted, ok := strftimeFormat(now, op[1:])
			if !ok {
				fmt.Fprintln(ctx.Stderr, formatted)
				return 1
			}
			fmt.Fprintln(ctx.Stdout, formatted)
			return 0
		}
	}

	fmt.Fprintln(ctx.Stdout, now.Format("Mon Jan _2 15:04:05 MST 2006"))
	return 0
}

func iso8601Layout(timespec string) (string, bool) {
	switch timespec {
	case "", "date":
		return "2006-01-02", true
	case "hours":
		return "2006-01-02T15-07:00", true
	case "minutes":
		return "2006-01-02T15:04-07:00", true
	case "seconds":
		return "2006-01-02T15:04:05-07:00", true
	case "ns":
		return "2006-01-02T15:04:05.000000000-07:00", true
	default:
		return "", false
	}
}

func parseRelativeOffset(s string) (base string, relY, relM, relD, relSec int, ok bool) {
	lower := strings.ToLower(s)
	idx := -1
	units := []string{"year", "month", "day", "hour", "minute", "second"}
	for _, u := range units {
		if i := strings.LastIndex(lower, u); i > 0 {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return s, 0, 0, 0, 0, false
	}

	start := idx - 1
	for start >= 0 && (lower[start] == 's' || lower[start] == ' ' || lower[start] == '\t') {
		start--
	}
	numEnd := start + 1
	for start >= 0 && ((lower[start] >= '0' && lower[start] <= '9') || lower[start] == '+' || lower[start] == '-') {
		start--
	}
	numStart := start + 1
	if numStart >= numEnd {
		return s, 0, 0, 0, 0, false
	}

	valStr := lower[numStart:numEnd]
	num, err := strconv.Atoi(valStr)
	if err != nil {
		return s, 0, 0, 0, 0, false
	}

	if strings.Contains(lower[idx:], "ago") {
		num = -num
	}

	unitStr := lower[idx:]
	if strings.HasPrefix(unitStr, "year") {
		relY = num
	} else if strings.HasPrefix(unitStr, "month") {
		relM = num
	} else if strings.HasPrefix(unitStr, "day") {
		relD = num
	} else if strings.HasPrefix(unitStr, "hour") {
		relSec = num * 3600
	} else if strings.HasPrefix(unitStr, "minute") {
		relSec = num * 60
	} else if strings.HasPrefix(unitStr, "second") {
		relSec = num
	}

	baseStr := strings.TrimRight(s[:numStart], " +-\t")
	return baseStr, relY, relM, relD, relSec, true
}

func isAllAlpha(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return len(s) > 0
}

func daysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if (year%4 == 0 && year%100 != 0) || (year%400 == 0) {
			return 29
		}
		return 28
	default:
		return 0
	}
}

func parseDateStr(ctx *Context, s string) (time.Time, bool) {
	var b strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r' {
			b.WriteByte(' ')
		} else {
			b.WriteRune(r)
		}
	}
	s = strings.TrimSpace(b.String())

	if s == "" || s == "-" {
		loc := dateLocation(ctx, time.Now())
		today := time.Now().In(loc)
		return time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc), true
	}

	// Single letter or letter+digits military timezone e.g. "m", "m9", "a5", "Z"
	if len(s) >= 1 && len(s) <= 3 {
		ch := strings.ToUpper(s)[0]
		if ch == 'J' {
			return time.Now().In(dateLocation(ctx, time.Now())), true
		}
		if offset, ok := militaryTZOffsets[ch]; ok {
			hour := 0
			valid := true
			if len(s) > 1 {
				h, err := strconv.Atoi(s[1:])
				if err == nil && h >= 0 && h <= 23 {
					hour = h
				} else {
					valid = false
				}
			}
			if valid {
				loc := time.FixedZone(string(ch), offset)
				today := time.Now().In(loc)
				return time.Date(today.Year(), today.Month(), today.Day(), hour, 0, 0, 0, loc), true
			}
		}
	}

	// Check relative adjustment like "2003-08-31 12:00:00 +0 7 months ago"
	baseStr, relY, relM, relD, relSec, hasRel := parseRelativeOffset(s)
	if hasRel {
		tBase, ok := parseDateStr(ctx, baseStr)
		if !ok {
			return time.Time{}, false
		}
		tFinal := tBase.AddDate(relY, relM, relD).Add(time.Duration(relSec) * time.Second)
		return tFinal, true
	}

	if strings.HasPrefix(s, "@") {
		if f, err := strconv.ParseFloat(s[1:], 64); err == nil {
			sec := int64(math.Floor(f))
			nsec := int64((f - math.Floor(f)) * 1e9)
			return time.Unix(sec, nsec).In(dateLocation(ctx, time.Unix(sec, nsec))), true
		}
	}

	// Handle AM/PM attached without space e.g. "1:00AM" -> "1:00 AM"
	sNorm := s
	sNorm = strings.ReplaceAll(sNorm, "p.m.", "pm")
	sNorm = strings.ReplaceAll(sNorm, "P.M.", "PM")
	sNorm = strings.ReplaceAll(sNorm, "a.m.", "am")
	sNorm = strings.ReplaceAll(sNorm, "A.M.", "AM")

	for _, ampm := range []string{"AM", "PM", "am", "pm"} {
		if idx := strings.Index(sNorm, ampm); idx > 0 && sNorm[idx-1] >= '0' && sNorm[idx-1] <= '9' {
			sNorm = sNorm[:idx] + " " + sNorm[idx:]
		}
	}

	normalized := strings.Join(strings.Fields(sNorm), " ")

	loc := dateLocation(ctx, time.Now())

	if len(normalized) >= 10 {
		idx := strings.IndexByte(normalized, '-')
		if idx >= 4 {
			if yr, err := strconv.Atoi(normalized[:idx]); err == nil && (yr >= 10000 || yr < 0) {
				rest := normalized[idx+1:]
				if len(rest) >= 5 && rest[2] == '-' {
					if mo, e1 := strconv.Atoi(rest[:2]); e1 == nil && mo >= 1 && mo <= 12 {
						if da, e2 := strconv.Atoi(rest[3:5]); e2 == nil && da >= 1 && da <= daysInMonth(yr, mo) {
							h, m, sec := 0, 0, 0
							rem := strings.TrimSpace(rest[5:])
							if len(rem) >= 5 && rem[2] == ':' {
								h, _ = strconv.Atoi(rem[:2])
								m, _ = strconv.Atoi(rem[3:5])
								if len(rem) >= 8 && rem[5] == ':' {
									sec, _ = strconv.Atoi(rem[6:8])
									rem = strings.TrimSpace(rem[8:])
								} else {
									rem = strings.TrimSpace(rem[5:])
								}
							}
							parseLoc := loc
							if rem != "" {
								if strings.HasPrefix(rem, "+") || strings.HasPrefix(rem, "-") {
									if tzT, ok := parseDateWithTZ("2000-01-01T00:00:00" + rem); ok {
										parseLoc = tzT.Location()
									}
								}
							}
							return time.Date(yr, time.Month(mo), da, h, m, sec, 0, parseLoc), true
						}
					}
				}
			}
		}
	}

	// Check if ending with a known timezone abbreviation (e.g. "KST", "EST")
	parts := strings.Fields(normalized)
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if isAllAlpha(lastPart) && len(lastPart) >= 3 && len(lastPart) <= 4 {
			if offset, ok := tzAbbrevOffsets[strings.ToUpper(lastPart)]; ok {
				loc = time.FixedZone(lastPart, offset)
				normalized = strings.Join(parts[:len(parts)-1], " ")

				// Check if remaining string STILL ends with another timezone or offset (e.g. "EST EST" or "+0000 EST")
				remParts := strings.Fields(normalized)
				if len(remParts) > 0 {
					prevLast := remParts[len(remParts)-1]
					if isAllAlpha(prevLast) {
						if _, dupTZ := tzAbbrevOffsets[strings.ToUpper(prevLast)]; dupTZ {
							return time.Time{}, false
						}
					} else if strings.HasPrefix(prevLast, "+") || strings.HasPrefix(prevLast, "-") {
						return time.Time{}, false
					}
				}
			} else if _, err := time.LoadLocation(lastPart); err == nil {
				// valid loc
			} else {
				return time.Time{}, false
			}
		}
	}

	for _, layout := range []string{
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 -0",
		"2006-01-02 15:04:05 +0",
		"2006-01-02 15:04 -0700",
		"2006-01-02 15:04 +0700",
		"2006-01-02 15:04 -0",
		"2006-01-02 15:04 +0",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02 03:04:05 PM",
		"2006-01-02 3:04:05 PM",
		"2006-01-02 03:04 PM",
		"2006-01-02 3:04 PM",
		"2006-01-02 03:04:05 pm",
		"2006-01-02 3:04:05 pm",
		"2006-01-02 03:04 pm",
		"2006-01-02 3:04 pm",
		"2006-1-2 15:4:5 MST",
		"2006-1-2 15:4:5 -0700",
		"2006-1-2 15:4:5Z",
		"2006-1-2 15:4:5",
		"2006-1-2 15:4",
		"2006-1-2 03:04 PM",
		"2006-1-2 3:04 PM",
		"2006-1-2 3:04:05 PM",
		"2006-1-2",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006.01.02-15:04:05",
		"2006.01.02-15:04",
		"2006.1.2-15:4:5",
		"2006.1.2-15:4",
		"1.2-15:4:5",
		"1.2-15:4",
		"2006-01-02",
		"01/02/2006",
		"1/2/2006",
		"01/02/06",
		"1/2/06",
		"15:04:05 -0700",
		"15:04 -0700",
		"15:04:05",
		"15:04",
		"15:4:5",
		"15:4",
		"03:04 PM",
		"3:04 PM",
		"03:04:05 PM",
		"3:04:05 PM",
		"03:04 pm",
		"3:04 pm",
		"03:04:05 pm",
		"3:04:05 pm",
		"Jan 2 2006 15:04:05",
		"Jan 2 2006 15:04",
		"Jan 2 2006 3:04:05 PM",
		"Jan 2 2006 3:04 PM",
		"Jan 2 2006",
		"Jan 2 15:04:05 2006",
		"Mon Jan 2 15:04:05 MST 2006",
		"Mon 2 Jan 2006 15:04:05",
		"Mon 02 Jan 2006 15:04:05",
		"2 Jan 2006 15:04:05",
		"02 Jan 2006 15:04:05",
		"010215042006.05",
		"010215042006",
		"200601021504.05",
		"200601021504",
		"0102150406.05",
		"0102150406",
		"01021504",
	} {
		if t, err := time.ParseInLocation(layout, normalized, loc); err == nil {
			if t.Year() == 0 {
				today := time.Now().In(loc)
				t = time.Date(today.Year(), today.Month(), today.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
			}
			return t, true
		}
	}
	return parseDateWithTZ(normalized)
}

func parseDateWithTZ(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	dtPart := s
	tzPart := ""
	if idx := strings.IndexByte(s, 'T'); idx > 0 {
		rest := s[idx+1:]
		timeEnd := 0
		for timeEnd < len(rest) && (rest[timeEnd] >= '0' && rest[timeEnd] <= '9' || rest[timeEnd] == ':') {
			timeEnd++
		}
		dtPart = s[:idx+1+timeEnd]
		tzPart = strings.TrimSpace(rest[timeEnd:])
	}

	t, err := time.Parse("2006-01-02T15:04:05", dtPart)
	if err != nil {
		return time.Time{}, false
	}

	if tzPart == "" || tzPart == "Z" {
		return t, true
	}

	tzPart = strings.ReplaceAll(tzPart, " ", "")

	sign := 1
	if tzPart[0] == '+' {
		tzPart = tzPart[1:]
	} else if tzPart[0] == '-' {
		sign = -1
		tzPart = tzPart[1:]
	}

	hours, mins := 0, 0
	if colIdx := strings.IndexByte(tzPart, ':'); colIdx >= 0 {
		h, err1 := strconv.Atoi(tzPart[:colIdx])
		m, err2 := strconv.Atoi(tzPart[colIdx+1:])
		if err1 != nil || err2 != nil {
			return time.Time{}, false
		}
		hours, mins = h, m
	} else {
		switch len(tzPart) {
		case 1, 2:
			h, err := strconv.Atoi(tzPart)
			if err != nil {
				return time.Time{}, false
			}
			hours = h
		case 3:
			h, err := strconv.Atoi(tzPart[:1])
			m, err2 := strconv.Atoi(tzPart[1:])
			if err != nil || err2 != nil {
				return time.Time{}, false
			}
			hours, mins = h, m
		case 4:
			h, err := strconv.Atoi(tzPart[:2])
			m, err2 := strconv.Atoi(tzPart[2:])
			if err != nil || err2 != nil {
				return time.Time{}, false
			}
			hours, mins = h, m
		default:
			return time.Time{}, false
		}
	}

	offset := sign * (hours*3600 + mins*60)
	loc := time.FixedZone("", offset)
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
	return t, true
}

func strftimeFormat(t time.Time, format string) (string, bool) {
	var out strings.Builder
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			out.WriteByte(format[i])
			i++
			continue
		}
		startIdx := i
		i++ // skip '%'
		if i >= len(format) {
			out.WriteByte('%')
			break
		}

		noPad := false
		spacePad := false
		zeroPad := false
		upperCase := false
		changeCase := false
		forceSign := false
		colonCount := 0

		for i < len(format) {
			ch := format[i]
			if ch == '-' {
				noPad = true
				i++
			} else if ch == '_' {
				spacePad = true
				i++
			} else if ch == '0' {
				zeroPad = true
				i++
			} else if ch == '^' {
				upperCase = true
				i++
			} else if ch == '#' {
				changeCase = true
				i++
			} else if ch == '+' {
				forceSign = true
				i++
			} else if ch == ':' {
				colonCount++
				i++
			} else {
				break
			}
		}

		width := -1
		if i < len(format) && format[i] >= '0' && format[i] <= '9' {
			wStart := i
			for i < len(format) && format[i] >= '0' && format[i] <= '9' {
				i++
			}
			wStr := format[wStart:i]
			w, err := strconv.Atoi(wStr)
			if err != nil || w > 2147483647 || len(wStr) > 9 {
				targetSpec := 's'
				if i < len(format) {
					targetSpec = rune(format[i])
				}
				return fmt.Sprintf("date: pad width '%s' is too large for specifier '%%%c'", wStr, targetSpec), false
			}
			width = w
		}

		if i < len(format) && (format[i] == 'E' || format[i] == 'O') {
			i++
		}

		if i >= len(format) {
			out.WriteString(format[startIdx:])
			break
		}

		spec := format[i]
		i++

		if colonCount > 3 {
			out.WriteString(format[startIdx:i])
			continue
		}

		var rawVal string
		recognized := true
		isNumeric := true

		switch spec {
		case 'Y':
			rawVal = fmt.Sprintf("%04d", t.Year())
		case 'y':
			rawVal = fmt.Sprintf("%02d", t.Year()%100)
		case 'm':
			rawVal = fmt.Sprintf("%02d", t.Month())
		case 'd':
			rawVal = fmt.Sprintf("%02d", t.Day())
		case 'e':
			rawVal = fmt.Sprintf("%2d", t.Day())
		case 'H':
			rawVal = fmt.Sprintf("%02d", t.Hour())
		case 'k':
			rawVal = fmt.Sprintf("%2d", t.Hour())
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			rawVal = fmt.Sprintf("%02d", h)
		case 'l':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			rawVal = fmt.Sprintf("%2d", h)
		case 'M':
			rawVal = fmt.Sprintf("%02d", t.Minute())
		case 'S':
			rawVal = fmt.Sprintf("%02d", t.Second())
		case 'N':
			nsStr := fmt.Sprintf("%09d", t.Nanosecond())
			if width > 0 && width < len(nsStr) {
				rawVal = nsStr[:width]
				width = -1
			} else {
				rawVal = nsStr
			}
		case 'A':
			rawVal = t.Weekday().String()
			isNumeric = false
		case 'a':
			rawVal = t.Weekday().String()[:3]
			isNumeric = false
		case 'B':
			rawVal = t.Month().String()
			isNumeric = false
		case 'b', 'h':
			rawVal = t.Month().String()[:3]
			isNumeric = false
		case 'Z':
			rawVal = t.Format("MST")
			isNumeric = false
		case 'z':
			_, offset := t.Zone()
			sign := "+"
			if offset < 0 {
				sign = "-"
				offset = -offset
			}
			h := offset / 3600
			m := (offset % 3600) / 60
			sSec := offset % 60

			if colonCount == 1 {
				rawVal = fmt.Sprintf("%s%02d:%02d", sign, h, m)
			} else if colonCount == 2 {
				rawVal = fmt.Sprintf("%s%02d:%02d:%02d", sign, h, m, sSec)
			} else if colonCount >= 3 {
				if sSec != 0 {
					rawVal = fmt.Sprintf("%s%02d:%02d:%02d", sign, h, m, sSec)
				} else if m != 0 {
					rawVal = fmt.Sprintf("%s%02d:%02d", sign, h, m)
				} else {
					rawVal = fmt.Sprintf("%s%02d", sign, h)
				}
			} else {
				rawVal = fmt.Sprintf("%s%02d%02d", sign, h, m)
			}
			isNumeric = false
		case 's':
			rawVal = strconv.FormatInt(t.Unix(), 10)
		case 'p':
			if t.Hour() < 12 {
				rawVal = "AM"
			} else {
				rawVal = "PM"
			}
			if changeCase {
				rawVal = "am"
				if t.Hour() >= 12 {
					rawVal = "pm"
				}
			}
			isNumeric = false
		case 'P':
			if t.Hour() < 12 {
				rawVal = "am"
			} else {
				rawVal = "pm"
			}
			if changeCase {
				rawVal = "am"
				if t.Hour() >= 12 {
					rawVal = "pm"
				}
			}
			isNumeric = false
		case 'n':
			rawVal = "\n"
			isNumeric = false
		case 't':
			rawVal = "\t"
			isNumeric = false
		case '%':
			rawVal = "%"
			isNumeric = false
		case 'T':
			rawVal = fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
			isNumeric = false
		case 'R':
			rawVal = fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
			isNumeric = false
		case 'F':
			rawVal = fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
			isNumeric = false
		case 'D':
			rawVal = fmt.Sprintf("%02d/%02d/%02d", t.Month(), t.Day(), t.Year()%100)
			isNumeric = false
		case 'r':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			ampm := "AM"
			if t.Hour() >= 12 {
				ampm = "PM"
			}
			fmt.Fprintf(&out, "%02d:%02d:%02d %s", h, t.Minute(), t.Second(), ampm)
			isNumeric = false
		case 'j':
			rawVal = fmt.Sprintf("%03d", t.YearDay())
		case 'u':
			d := int(t.Weekday())
			if d == 0 {
				d = 7
			}
			rawVal = fmt.Sprintf("%d", d)
		case 'w':
			rawVal = fmt.Sprintf("%d", t.Weekday())
		case 'C':
			rawVal = fmt.Sprintf("%02d", t.Year()/100)
		case 'c':
			rawVal = t.Format("Mon Jan _2 15:04:05 2006")
			isNumeric = false
		case 'x':
			rawVal = fmt.Sprintf("%02d/%02d/%02d", t.Month(), t.Day(), t.Year()%100)
			isNumeric = false
		case 'X':
			rawVal = fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
			isNumeric = false
		default:
			recognized = false
		}

		if !recognized {
			out.WriteString(format[startIdx:i])
			continue
		}

		if forceSign && isNumeric && !strings.HasPrefix(rawVal, "-") {
			rawVal = "+" + rawVal
		}

		if upperCase {
			rawVal = strings.ToUpper(rawVal)
		} else if changeCase && spec != 'p' && spec != 'P' {
			var b strings.Builder
			for _, r := range rawVal {
				if r >= 'a' && r <= 'z' {
					b.WriteRune(r - 32)
				} else if r >= 'A' && r <= 'Z' {
					b.WriteRune(r + 32)
				} else {
					b.WriteRune(r)
				}
			}
			rawVal = b.String()
		}

		if noPad {
			if len(rawVal) > 1 && (rawVal[0] == '0' || rawVal[0] == ' ') {
				rawVal = strings.TrimLeft(rawVal, "0 ")
				if rawVal == "" {
					rawVal = "0"
				}
			}
		} else if spacePad {
			if len(rawVal) > 1 && rawVal[0] == '0' {
				rawVal = " " + rawVal[1:]
			}
		} else if zeroPad {
			if len(rawVal) > 1 && rawVal[0] == ' ' {
				rawVal = "0" + rawVal[1:]
			}
		}

		if width > 0 && len(rawVal) > width {
			for len(rawVal) > width && (rawVal[0] == '0' || rawVal[0] == ' ') {
				rawVal = rawVal[1:]
			}
		}

		if width > 0 && len(rawVal) < width && !noPad {
			if width > 1000 {
				width = 1000
			}
			padChar := " "
			if zeroPad || (isNumeric && !spacePad) {
				padChar = "0"
			}
			prefix := ""
			body := rawVal
			if (strings.HasPrefix(rawVal, "+") || strings.HasPrefix(rawVal, "-")) && padChar == "0" {
				prefix = rawVal[:1]
				body = rawVal[1:]
			}
			rawVal = prefix + strings.Repeat(padChar, width-len(rawVal)) + body
		}

		out.WriteString(rawVal)
	}
	return out.String(), true
}
