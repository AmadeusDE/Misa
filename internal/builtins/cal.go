// Behaviour adapted from toybox cal.c (0BSD) and uutils util-linux cal (MIT).
package builtins

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

func init() { Register(Command{Name: "cal", Run: calRun}) }

type calOptions struct {
	monday, julian, weeks, year, color bool
	highlight                          time.Time
}

func calRun(ctx *Context) int {
	spec, err := ParseSpec("y(year)3(three)Y(twelve)n(months):m(monday)j(julian)w(week)(color):?")
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "cal:", err)
		return 1
	}
	flags, err := spec.ParseArgs(ctx.Args)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "cal:", err)
		return 1
	}
	if HandleHelpVersion(ctx, flags, spec) {
		return 0
	}
	now := time.Now()
	date, yearMode, err := calDate(flags.Operands, now)
	if err != nil {
		fmt.Fprintln(ctx.Stderr, "cal:", err)
		return 1
	}
	o := calOptions{monday: flags.Bool("m"), julian: flags.Bool("j"), weeks: flags.Bool("w"), year: yearMode || flags.Bool("y"), highlight: now}
	if len(flags.Operands) == 3 {
		o.highlight = date
	}
	color, hasColor := flags.String("color")
	if !hasColor || color == "" {
		color = "auto"
	}
	switch color {
	case "always":
		o.color = true
	case "auto":
		if f, ok := ctx.Stdout.(*os.File); ok {
			o.color = term.IsTerminal(f.Fd())
		}
	case "never":
	default:
		fmt.Fprintf(ctx.Stderr, "cal: invalid color mode %q\n", color)
		return 1
	}
	months := 1
	n, hasMonths := flags.String("n")
	if flags.Bool("y") && (flags.Bool("Y") || hasMonths) || flags.Bool("Y") && hasMonths {
		fmt.Fprintln(ctx.Stderr, "cal: --year, --twelve and --months are mutually exclusive")
		return 1
	}
	if hasMonths {
		count, err := strconv.ParseUint(n, 10, 32)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "cal: invalid month count %q\n", n)
			return 1
		}
		months = int(max(count, 1))
	}
	date = time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
	switch {
	case o.year:
		months = 12
		date = time.Date(date.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		_, width := o.widths()
		fmt.Fprintln(ctx.Stdout, calCenter(strconv.Itoa(date.Year()), 3*width+6))
		fmt.Fprintln(ctx.Stdout)
	case flags.Bool("Y"):
		months = 12
	case flags.Bool("3"):
		months = 3
		date = date.AddDate(0, -1, 0)
	}
	for offset := 0; offset < months; offset += 3 {
		var calendars [][8]string
		for column := 0; column < min(3, months-offset); column++ {
			calendars = append(calendars, calMonth(date.AddDate(0, offset+column, 0), o))
		}
		for row := 0; row < 8; row++ {
			var line []string
			for _, calendar := range calendars {
				line = append(line, calendar[row])
			}
			if _, err := fmt.Fprintln(ctx.Stdout, strings.Join(line, "   ")); err != nil {
				fmt.Fprintln(ctx.Stderr, "cal:", err)
				return 1
			}
		}
	}
	return 0
}

func calDate(args []string, now time.Time) (time.Time, bool, error) {
	year, month, day := now.Year(), int(now.Month()), 1
	yearMode := false
	var err error
	switch len(args) {
	case 0:
		return now, false, nil
	case 1:
		if year, err = strconv.Atoi(args[0]); err == nil {
			yearMode, month = true, 1
		} else {
			year = now.Year()
			month = calParseMonth(args[0])
			err = nil
		}
	case 2, 3:
		year, err = strconv.Atoi(args[len(args)-1])
		month = calParseMonth(args[len(args)-2])
		if len(args) == 3 && err == nil {
			day, err = strconv.Atoi(args[0])
		}
	default:
		return time.Time{}, false, fmt.Errorf("too many arguments")
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if err != nil || year < 1 || year > 9999 || month < 1 || month > 12 || day < 1 || date.Day() != day || int(date.Month()) != month {
		return time.Time{}, false, fmt.Errorf("invalid date")
	}
	return date, yearMode, nil
}

func calParseMonth(s string) int {
	if month, err := strconv.Atoi(s); err == nil {
		return month
	}
	for month := time.January; month <= time.December; month++ {
		if strings.EqualFold(s, month.String()) || strings.EqualFold(s, month.String()[:3]) {
			return int(month)
		}
	}
	return 0
}

func (o calOptions) widths() (int, int) {
	cell := 2
	if o.julian {
		cell = 3
	}
	width := 7*(cell+1) - 1
	if o.weeks {
		width += 3
	}
	return cell, width
}

func calCenter(text string, width int) string {
	padding := max(0, width-len(text))
	return strings.Repeat(" ", padding/2) + text + strings.Repeat(" ", padding-padding/2)
}

func calMonth(date time.Time, o calOptions) [8]string {
	cell, width := o.widths()
	var lines [8]string
	title := date.Month().String()
	if !o.year {
		title += " " + strconv.Itoa(date.Year())
	}
	lines[0] = calCenter(title, width)
	start := time.Sunday
	if o.monday {
		start = time.Monday
	}
	var weekdays []string
	for i := 0; i < 7; i++ {
		weekdays = append(weekdays, time.Weekday((int(start) + i) % 7).String()[:cell])
	}
	lines[1] = strings.Join(weekdays, " ")
	if o.weeks {
		lines[1] = "   " + lines[1]
	}
	first := (int(date.Weekday()) - int(start) + 7) % 7
	days := date.AddDate(0, 1, -1).Day()
	for row := 0; row < 6; row++ {
		var cells []string
		for column := 0; column < 7; column++ {
			day := row*7 + column - first + 1
			text := strings.Repeat(" ", cell)
			if day >= 1 && day <= days {
				d := date.AddDate(0, 0, day-1)
				number := day
				if o.julian {
					number = d.YearDay()
				}
				text = fmt.Sprintf("%*d", cell, number)
				if o.color && d.Year() == o.highlight.Year() && d.YearDay() == o.highlight.YearDay() {
					text = strings.TrimSuffix(lipgloss.NewStyle().Reverse(true).Render(text), ansi.ResetStyle) + ansi.NewStyle(0).String()
				}
			}
			cells = append(cells, text)
		}
		lines[row+2] = strings.Join(cells, " ")
		if o.weeks {
			prefix := "   "
			day := max(1, row*7-first+1)
			if day <= days {
				d := date.AddDate(0, 0, day-1)
				var week int
				if o.monday {
					_, week = d.ISOWeek()
				} else {
					jan1 := time.Date(d.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
					week = (d.YearDay()-1+int(jan1.Weekday()))/7 + 1
				}
				prefix = fmt.Sprintf("%2d ", week)
			}
			lines[row+2] = prefix + lines[row+2]
		}
	}
	return lines
}
