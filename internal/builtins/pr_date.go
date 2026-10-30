// Date formatting for `pr -D`/--date-format, a small strftime subset. The
// default format ("%Y-%m-%d %H:%M") and the POSIX-locale fallback
// ("%b %e %H:%M %Y") match uutils coreutils pr (MIT) at commit
// fee61ff5d4f87e0f912a52e639942b5f7ad58663; the conversion itself is Go's.
package builtins

import (
	"strconv"
	"strings"
	"time"
)

var prMonthNames = [...]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

var prDayNames = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// prStrftime expands the strftime conversions pr users rely on. Unknown
// conversions are copied through unchanged (GNU pr rejects them; Misa keeps them
// so a format it does not know still prints something sensible).
func prStrftime(t time.Time, format string) string {
	format = strings.TrimPrefix(format, "+")
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			b.WriteByte(format[i])
			continue
		}
		spec := format[i+1]
		i++
		b.WriteString(prStrftimeSpec(t, spec, format[i:]))
	}
	return b.String()
}

func prStrftimeSpec(t time.Time, spec byte, rest string) string {
	switch spec {
	case 'Y':
		return strconv.Itoa(t.Year())
	case 'y':
		return two(t.Year() % 100)
	case 'm':
		return two(int(t.Month()))
	case 'd':
		return two(t.Day())
	case 'e':
		if d := t.Day(); d < 10 {
			return " " + strconv.Itoa(d)
		}
		return strconv.Itoa(t.Day())
	case 'H':
		return two(t.Hour())
	case 'I':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		return two(h)
	case 'M':
		return two(t.Minute())
	case 'S':
		return two(t.Second())
	case 's':
		return strconv.FormatInt(t.Unix(), 10)
	case 'j':
		return pad(t.YearDay(), 3)
	case 'b', 'h':
		return prMonthNames[int(t.Month())-1][:3]
	case 'B':
		return prMonthNames[int(t.Month())-1]
	case 'a':
		return prDayNames[int(t.Weekday())][:3]
	case 'A':
		return prDayNames[int(t.Weekday())]
	case 'p':
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	case 'Z':
		return t.Format("MST")
	case 'z':
		return t.Format("-0700")
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case '%':
		return "%"
	default:
		return "%" + string(spec)
	}
}

func two(n int) string { return pad(n, 2) }

func pad(n, width int) string {
	s := strconv.Itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
