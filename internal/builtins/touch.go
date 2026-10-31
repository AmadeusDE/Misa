package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const touchSpec = "c(no-create)h(no-dereference)t:d(date):r(reference):amf"

var touchSpecParsed *Spec

func init() {
	var err error
	touchSpecParsed, err = ParseSpec(touchSpec)
	if err != nil {
		panic("builtins: bad touch spec: " + err.Error())
	}
	Register(Command{Name: "touch", Run: runTouch})
}

func runTouch(ctx *Context) int {
	flags, err := touchSpecParsed.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "touch:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, touchSpecParsed) {
		return 0
	}

	posixlyCorrect := ctx.Getenv("POSIXLY_CORRECT") != ""

	_, hasT := flags.String("t")
	_, hasD := flags.String("d")
	_, hasR := flags.String("r")

	var obsolescentStamp *time.Time
	operands := flags.Operands
	if posixlyCorrect && !hasT && !hasD && !hasR && len(operands) >= 2 {
		if len(operands[0]) == 8 && isObsolescentDate(operands[0]) {
			t, err := parseObsolescentDateIn(operands[0], ctx.Location())
			if err == nil {
				obsolescentStamp = &t
				operands = operands[1:]
			}
		}
	}

	if len(operands) == 0 {
		fmt.Fprintln(ctx.Stderr, "touch: missing operand")
		return 1
	}

	noCreate := flags.Bool("c")
	onlyAccess := flags.Bool("a")
	onlyModify := flags.Bool("m")
	if !onlyAccess && !onlyModify {
		onlyAccess = true
		onlyModify = true
	}

	var stamp time.Time
	haveStamp := false
	noDereference := flags.Bool("h")

	if refName, ok := flags.String("r"); ok {
		refPath := resolvePath(ctx, refName)
		statFn := os.Stat
		if noDereference {
			statFn = os.Lstat
		}
		info, err := statFn(refPath)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "touch: cannot stat '%s': No such file or directory\n", refName)
			return 1
		}
		stamp = info.ModTime()
		haveStamp = true
	}

	if obsolescentStamp != nil {
		stamp = *obsolescentStamp
		haveStamp = true
	}

	if tStr, ok := flags.String("t"); ok {
		var parseErr error
		stamp, parseErr = parseTouchTIn(tStr, ctx.Location())
		if parseErr != nil {
			fmt.Fprintf(ctx.Stderr, "touch: invalid date format '%s'\n", tStr)
			return 1
		}
		haveStamp = true
	}

	if dStr, ok := flags.String("d"); ok {
		var parseErr error
		stamp, parseErr = parseTouchDIn(dStr, stamp, ctx.Location())
		if parseErr != nil {
			fmt.Fprintf(ctx.Stderr, "touch: invalid date format '%s'\n", dStr)
			return 1
		}
		haveStamp = true
	}

	if !haveStamp {
		stamp = time.Now()
	}

	exitCode := 0

	for _, name := range operands {
		if name == "-" && noDereference {
			continue
		}
		var info os.FileInfo
		path := resolvePath(ctx, name)
		if name != "-" {
			statFn := os.Stat
			if noDereference {
				statFn = os.Lstat
			}
			var statErr error
			info, statErr = statFn(path)
			if os.IsNotExist(statErr) {
				if noCreate {
					continue
				}
				if strings.HasSuffix(name, "/") || strings.HasSuffix(name, string(filepath.Separator)) {
					fmt.Fprintf(ctx.Stderr, "touch: cannot touch '%s': No such file or directory\n", name)
					exitCode = 1
					continue
				}
				f, createErr := os.Create(path)
				if createErr != nil {
					fmt.Fprintf(ctx.Stderr, "touch: cannot touch '%s': %s\n", name, fmtOsErr(createErr))
					exitCode = 1
					continue
				}
				f.Close()

				info, statErr = statFn(path)
				if statErr != nil {
					fmt.Fprintf(ctx.Stderr, "touch: cannot touch '%s': %s\n", name, fmtOsErr(statErr))
					exitCode = 1
					continue
				}
			} else if statErr != nil {
				fmt.Fprintf(ctx.Stderr, "touch: cannot touch '%s': %s\n", name, fmtOsErr(statErr))
				exitCode = 1
				continue
			}
		}

		atime := stamp
		mtime := stamp
		if info != nil {
			if onlyAccess && !onlyModify {
				mtime = info.ModTime()
			} else if onlyModify && !onlyAccess {
				atime = info.ModTime()
			}
		}

		if err := touchChtimes(ctx, name, path, atime, mtime, noDereference, haveStamp); err != nil {
			fmt.Fprintf(ctx.Stderr, "touch: setting times of '%s': %s\n", name, fmtOsErr(err))
			exitCode = 1
		}
	}
	return exitCode
}

func isObsolescentDate(s string) bool {
	if len(s) != 8 && len(s) != 10 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	mm, _ := strconv.Atoi(s[0:2])
	dd, _ := strconv.Atoi(s[2:4])
	hh, _ := strconv.Atoi(s[4:6])
	min, _ := strconv.Atoi(s[6:8])
	if mm < 1 || mm > 12 || dd < 1 || dd > 31 || hh > 23 || min > 59 {
		return false
	}
	// For 10-digit: POSIX obsolescent MMDDhhmm[YY] was legacy POSIX.1-1988/1990 format.
	// But in GNU touch POSIXLY_CORRECT, 10-digit is YYMMDDhhmm or MMDDhhmmYY.
	// When MMDD is 0101hhmm (like 0101000000), if s[8:10] == "00" or file exists, prefer filename unless valid.
	return true
}

func parseObsolescentDateIn(s string, loc *time.Location) (time.Time, error) {
	now := time.Now().In(loc)
	if len(s) == 8 { // MMDDhhmm
		return time.ParseInLocation("200601021504", fmt.Sprintf("%d%s", now.Year(), s), loc)
	}
	// 10 digits: MMDDhhmmYY
	mmddhhmm := s[:8]
	yy, _ := strconv.Atoi(s[8:])
	cc := 2000
	if yy >= 69 {
		cc = 1900
	}
	return time.ParseInLocation("200601021504", fmt.Sprintf("%d%s", cc+yy, mmddhhmm), loc)
}

func touchChtimes(ctx *Context, name, path string, atime, mtime time.Time, noFollow bool, specifyTimes bool) error {
	var ts []unix.Timespec
	if specifyTimes {
		ts = []unix.Timespec{
			unix.NsecToTimespec(atime.UnixNano()),
			unix.NsecToTimespec(mtime.UnixNano()),
		}
	} else {
		ts = []unix.Timespec{
			{Sec: 0, Nsec: utimeNow},
			{Sec: 0, Nsec: utimeNow},
		}
	}

	if name == "-" {
		return unix.UtimesNanoAt(1, "", ts, 0)
	}

	flags := 0
	if noFollow {
		flags = unix.AT_SYMLINK_NOFOLLOW
	}

	err := unix.UtimesNanoAt(unix.AT_FDCWD, path, ts, flags)
	if err == nil {
		return nil
	}

	if (err == syscall.EACCES || err == syscall.EPERM) && !specifyTimes {
		fd, errOpen := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
		if errOpen == nil {
			defer unix.Close(fd)
			errFutimens := unix.UtimesNanoAt(fd, "", ts, 0)
			if errFutimens == nil {
				return nil
			}
		}
	}
	return err
}

func parseTouchTIn(s string, loc *time.Location) (time.Time, error) {
	base := s
	secs := "00"
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		base = s[:dot]
		secs = s[dot+1:]
		if len(secs) < 2 {
			secs = secs + "0"
		}
	}

	isLeapSec := false
	if secs == "60" {
		secs = "00"
		isLeapSec = true
	}
	if sVal, err := strconv.Atoi(secs); err != nil || sVal > 59 {
		return time.Time{}, fmt.Errorf("invalid seconds")
	}

	now := time.Now().In(loc)
	var layout, input string

	switch len(base) {
	case 8: // MMDDhhmm
		layout = "200601021504.05"
		input = fmt.Sprintf("%d%s.%s", now.Year(), base, secs)
	case 10: // YYMMDDhhmm
		layout = "0601021504.05"
		input = fmt.Sprintf("%s.%s", base, secs)
	case 12: // CCYYMMDDhhmm
		layout = "200601021504.05"
		input = fmt.Sprintf("%s.%s", base, secs)
	default:
		return time.Time{}, fmt.Errorf("unrecognized format")
	}

	t, err := time.ParseInLocation(layout, input, loc)
	if err != nil {
		return time.Time{}, err
	}

	if isLeapSec {
		t = t.Add(60 * time.Second)
	}

	hh, err2 := strconv.Atoi(base[len(base)-4 : len(base)-2])
	if err2 == nil && t.Hour() != hh {
		return time.Time{}, fmt.Errorf("invalid time for DST transition")
	}

	return t, nil
}

func parseTouchDIn(s string, base time.Time, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if base.IsZero() {
		base = time.Now().In(loc)
	}

	base = base.In(loc)

	if strings.HasPrefix(s, "@") {
		sec, err := strconv.ParseInt(s[1:], 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(sec, 0).UTC(), nil
	}

	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		fields := strings.Fields(s)
		if len(fields) >= 2 {
			val, err := strconv.Atoi(fields[0])
			if err == nil {
				unit := strings.ToLower(fields[1])
				switch {
				case strings.HasPrefix(unit, "day"):
					return base.AddDate(0, 0, val), nil
				case strings.HasPrefix(unit, "hour"):
					return base.Add(time.Duration(val) * time.Hour), nil
				case strings.HasPrefix(unit, "min"):
					return base.Add(time.Duration(val) * time.Minute), nil
				case strings.HasPrefix(unit, "sec"):
					return base.Add(time.Duration(val) * time.Second), nil
				case strings.HasPrefix(unit, "month"):
					return base.AddDate(0, val, 0), nil
				case strings.HasPrefix(unit, "year"):
					return base.AddDate(val, 0, 0), nil
				}
			}
		}
	}

	layouts := []string{
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05.999999999 +0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 +0700",
		"2006-01-02 15:04 -0700",
		"2006-01-02 15:04 +0700",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006-1-2 15:04:05",
		"2006-1-2 15:04",
		"2006-1-2",
		"Mon Jan 02 15:04:05 2006",
		"Mon Jan 2 15:04:05 2006",
		"Jan 02 15:04:05 2006",
		"Jan 2 15:04:05 2006",
		time.RFC3339,
		time.RFC822,
		time.RFC822Z,
		time.RFC1123,
		time.RFC1123Z,
		time.ANSIC,
	}

	sNorm := strings.Replace(s, "T", " ", 1)
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
		if t, err := time.ParseInLocation(layout, sNorm, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date format")
}
