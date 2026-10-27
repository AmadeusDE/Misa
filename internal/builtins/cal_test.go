// Behaviour adapted from toybox cal.c (0BSD) and uutils util-linux cal (MIT).
package builtins

import (
	"regexp"
	"strings"
	"testing"
)

func TestCalGrids(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "isoPreviousYear", args: []string{"-w", "-m", "1", "2021"}, want: "     January 2021      \n   Mo Tu We Th Fr Sa Su\n53              1  2  3\n 1  4  5  6  7  8  9 10\n 2 11 12 13 14 15 16 17\n 3 18 19 20 21 22 23 24\n 4 25 26 27 28 29 30 31\n                       \n"},
		{name: "isoFirstWeek", args: []string{"-w", "-m", "1", "2015"}, want: "     January 2015      \n   Mo Tu We Th Fr Sa Su\n 1           1  2  3  4\n 2  5  6  7  8  9 10 11\n 3 12 13 14 15 16 17 18\n 4 19 20 21 22 23 24 25\n 5 26 27 28 29 30 31   \n                       \n"},
		{name: "usWeeks", args: []string{"-w", "1", "2021"}, want: "     January 2021      \n   Su Mo Tu We Th Fr Sa\n 1                 1  2\n 2  3  4  5  6  7  8  9\n 3 10 11 12 13 14 15 16\n 4 17 18 19 20 21 22 23\n 5 24 25 26 27 28 29 30\n 6 31                  \n"},
		{name: "julianLeapYear", args: []string{"-j", "12", "2000"}, want: "       December 2000       \nSun Mon Tue Wed Thu Fri Sat\n                    336 337\n338 339 340 341 342 343 344\n345 346 347 348 349 350 351\n352 353 354 355 356 357 358\n359 360 361 362 363 364 365\n366                        \n"},
		{name: "julianISOWeeks", args: []string{"-j", "-w", "-m", "2", "2024"}, want: "        February 2024         \n   Mon Tue Wed Thu Fri Sat Sun\n 5              32  33  34  35\n 6  36  37  38  39  40  41  42\n 7  43  44  45  46  47  48  49\n 8  50  51  52  53  54  55  56\n 9  57  58  59  60            \n                              \n"},
		{name: "colorNever", args: []string{"--color=never", "15", "3", "2024"}, want: "     March 2024     \nSu Mo Tu We Th Fr Sa\n                1  2\n 3  4  5  6  7  8  9\n10 11 12 13 14 15 16\n17 18 19 20 21 22 23\n24 25 26 27 28 29 30\n31                  \n"},
		{name: "colorAlways", args: []string{"--color=always", "15", "3", "2024"}, want: "     March 2024     \nSu Mo Tu We Th Fr Sa\n                1  2\n 3  4  5  6  7  8  9\n10 11 12 13 14 \u001b[7m15\u001b[0m 16\n17 18 19 20 21 22 23\n24 25 26 27 28 29 30\n31                  \n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltin(t, "cal", tt.args...)
			assertExitCode(t, r, 0)
			assertStdout(t, r, tt.want)
		})
	}
}

func TestCalModes(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		titles []string
	}{
		{"year", []string{"2024"}, []string{" 2024 ", "January", "February", "March", "October", "November", "December"}},
		{"yearOption", []string{"-y", "3", "2024"}, []string{"January", "February", "March", "October", "November", "December"}},
		{"previousYear", []string{"-3", "1", "2024"}, []string{"December 2023", "January 2024", "February 2024"}},
		{"nextYear", []string{"-3", "12", "2023"}, []string{"November 2023", "December 2023", "January 2024"}},
		{"twelve", []string{"-Y", "15", "3", "2024"}, []string{"March 2024", "February 2025"}},
		{"zeroMonths", []string{"-n", "0", "12", "2023"}, []string{"December 2023", "Su Mo Tu We Th Fr Sa"}},
		{"fourMonths", []string{"-n", "4", "aug", "2024"}, []string{"August 2024", "September 2024", "October 2024", "November 2024"}},
		{"monthName", []string{"aug"}, []string{"August"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBuiltin(t, "cal", tt.args...)
			assertExitCode(t, r, 0)
			last := 0
			for _, title := range tt.titles {
				index := strings.Index(r.stdout[last:], title)
				if index < 0 {
					t.Fatalf("missing or misordered title %q in %q", title, r.stdout)
				}
				last += index + len(title)
			}
			if tt.name == "year" || tt.name == "yearOption" {
				for _, pattern := range []string{`January +February +March`, `October +November +December`} {
					if !regexp.MustCompile(pattern).MatchString(r.stdout) {
						t.Errorf("output does not match %q: %q", pattern, r.stdout)
					}
				}
			}
			if tt.name == "previousYear" || tt.name == "nextYear" {
				line := strings.SplitN(r.stdout, "\n", 2)[0]
				for _, title := range tt.titles {
					if !strings.Contains(line, title) {
						t.Errorf("title %q not on first line %q", title, line)
					}
				}
			}
		})
	}
}

func TestCalInvalid(t *testing.T) {
	for _, args := range [][]string{
		{"--definitely-invalid"}, {"31", "2", "2000"}, {"13", "2000"},
		{"29", "2", "1900"}, {"bad"}, {"1", "2", "3", "4"},
		{"-n", "-1"}, {"-n", "abc"}, {"--color=bad"}, {"-y", "-Y"}, {"-y", "-n", "2"}, {"-Y", "-n", "2"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			r := runBuiltin(t, "cal", args...)
			assertExitCode(t, r, 1)
			if r.stderr == "" {
				t.Error("missing diagnostic")
			}
			if r.stdout != "" {
				t.Errorf("unexpected stdout %q", r.stdout)
			}
		})
	}
}
