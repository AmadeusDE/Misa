package builtins

import (
	"strconv"
	"strings"
	"time"
)

// dateLocation retains date's legacy POSIX rule support; other utilities use
// Context.Location, which deliberately does not parse POSIX TZ strings.
func dateLocation(ctx *Context, now time.Time) *time.Location {
	tz, ok := ctx.LookupEnv("TZ")
	if ok && tz != "" && !strings.HasPrefix(tz, ":") && (!strings.Contains(tz, "/") || strings.Contains(tz, ",")) {
		if _, err := time.LoadLocation(tz); err != nil && strings.ContainsAny(tz, "0123456789") {
			if loc, ok := parsePosixTZ(tz, now); ok {
				return loc
			}
		}
	}
	return ctx.Location()
}
func parsePosixTZ(tz string, now time.Time) (*time.Location, bool) {
	i := 0
	readName := func() string {
		start := i
		if i < len(tz) && tz[i] == '<' {
			i++
			start = i
			for i < len(tz) && tz[i] != '>' {
				i++
			}
			name := tz[start:i]
			if i < len(tz) && tz[i] == '>' {
				i++
			}
			return name
		}
		for i < len(tz) && ((tz[i] >= 'A' && tz[i] <= 'Z') || (tz[i] >= 'a' && tz[i] <= 'z')) {
			i++
		}
		return tz[start:i]
	}

	readOffset := func() (int, bool) {
		if i >= len(tz) {
			return 0, false
		}
		sign := 1
		if tz[i] == '+' {
			sign = -1
			i++
		} else if tz[i] == '-' {
			sign = 1
			i++
		}
		start := i
		for i < len(tz) && tz[i] >= '0' && tz[i] <= '9' {
			i++
		}
		if start == i {
			return 0, false
		}
		h, _ := strconv.Atoi(tz[start:i])
		m, s := 0, 0
		if i < len(tz) && tz[i] == ':' {
			i++
			start = i
			for i < len(tz) && tz[i] >= '0' && tz[i] <= '9' {
				i++
			}
			m, _ = strconv.Atoi(tz[start:i])
			if i < len(tz) && tz[i] == ':' {
				i++
				start = i
				for i < len(tz) && tz[i] >= '0' && tz[i] <= '9' {
					i++
				}
				s, _ = strconv.Atoi(tz[start:i])
			}
		}
		return sign * (h*3600 + m*60 + s), true
	}

	stdName := readName()
	if stdName == "" {
		return nil, false
	}
	stdOffset, hasStdOffset := readOffset()
	if !hasStdOffset {
		stdOffset = 0
	}

	dstName := readName()
	if dstName == "" {
		if stdOffset == 0 && (stdName == "UTC" || stdName == "GMT" || stdName == "Z") {
			return time.UTC, true
		}
		zoneName := stdName
		if stdName == "UTC" || stdName == "GMT" {
			zoneName = "UTC"
		}
		return time.FixedZone(zoneName, stdOffset), true
	}

	dstOffset, hasDstOffset := readOffset()
	if !hasDstOffset {
		dstOffset = stdOffset + 3600
	}

	if i >= len(tz) || tz[i] != ',' {
		return time.FixedZone(stdName, stdOffset), true
	}
	i++

	parseRule := func() (month, week, day, timeSec int, ok bool) {
		if i >= len(tz) || tz[i] != 'M' {
			return 0, 0, 0, 0, false
		}
		i++
		start := i
		for i < len(tz) && tz[i] >= '0' && tz[i] <= '9' {
			i++
		}
		month, _ = strconv.Atoi(tz[start:i])
		if i >= len(tz) || tz[i] != '.' {
			return 0, 0, 0, 0, false
		}
		i++
		start = i
		for i < len(tz) && tz[i] >= '0' && tz[i] <= '9' {
			i++
		}
		week, _ = strconv.Atoi(tz[start:i])
		if i >= len(tz) || tz[i] != '.' {
			return 0, 0, 0, 0, false
		}
		i++
		start = i
		for i < len(tz) && tz[i] >= '0' && tz[i] <= '9' {
			i++
		}
		day, _ = strconv.Atoi(tz[start:i])
		timeSec = 2 * 3600
		if i < len(tz) && tz[i] == '/' {
			i++
			tOffset, ok := readOffset()
			if ok {
				if tOffset < 0 {
					timeSec = -tOffset
				} else {
					timeSec = tOffset
				}
			}
		}
		return month, week, day, timeSec, true
	}

	sm, sw, sd, stime, ok1 := parseRule()
	if !ok1 || i >= len(tz) || tz[i] != ',' {
		return time.FixedZone(stdName, stdOffset), true
	}
	i++
	em, ew, ed, etime, ok2 := parseRule()
	if !ok2 {
		return time.FixedZone(stdName, stdOffset), true
	}

	utcNow := now.UTC()
	Y := utcNow.Year()

	getRuleTime := func(m, w, d, tSec, offset int) time.Time {
		t := time.Date(Y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		for int(t.Weekday()) != d {
			t = t.AddDate(0, 0, 1)
		}
		t = t.AddDate(0, 0, (w-1)*7)
		if t.Month() != time.Month(m) {
			t = t.AddDate(0, 0, -7)
		}
		return t.Add(time.Duration(tSec-offset) * time.Second)
	}

	dstStart := getRuleTime(sm, sw, sd, stime, stdOffset)
	dstEnd := getRuleTime(em, ew, ed, etime, dstOffset)

	isDST := false
	if !dstStart.After(dstEnd) {
		isDST = !utcNow.Before(dstStart) && utcNow.Before(dstEnd)
	} else {
		isDST = !utcNow.Before(dstStart) || utcNow.Before(dstEnd)
	}

	if isDST {
		return time.FixedZone(dstName, dstOffset), true
	}
	return time.FixedZone(stdName, stdOffset), true
}
