package builtins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInvocationLocation(t *testing.T) {
	cases := []struct {
		env    []string
		offset int
	}{{[]string{"TZ="}, 0}, {[]string{"TZ=invalid/zone"}, 0}, {[]string{"TZ=Etc/GMT+5"}, -18000}, {[]string{"TZ=:Etc/GMT-3"}, 10800}, {[]string{"TZ=EST5EDT"}, -18000}}
	for _, tc := range cases {
		_, offset := time.Date(2024, 1, 1, 0, 0, 0, 0, (&Context{Env: tc.env}).Location()).Zone()
		if offset != tc.offset {
			t.Errorf("%v: %d != %d", tc.env, offset, tc.offset)
		}
	}
	if (&Context{Env: []string{}}).Location() != time.Local {
		t.Error("unset TZ must use Local")
	}
	if b, err := os.ReadFile("/usr/share/zoneinfo/Etc/GMT+5"); err == nil {
		p := filepath.Join(t.TempDir(), "zone")
		if err := os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
		_, off := time.Date(2024, 1, 1, 0, 0, 0, 0, (&Context{Env: []string{"TZ=:" + p}}).Location()).Zone()
		if off != -18000 {
			t.Errorf("file offset %d", off)
		}
	}
}

func TestInvocationTimezoneCommands(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file")
	if err := os.WriteFile(p, nil, 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, tc := range []struct{ tz, want string }{{"UTC", "2024-01-02 03:04 +0000"}, {"Etc/GMT+5", "2024-01-01 22:04 -0500"}} {
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, cmd := range []struct {
				name string
				args []string
			}{{"date", []string{"-r", p, "+%Y-%m-%d %H:%M %z"}}, {"stat", []string{"-c", "%y", p}}, {"ls", []string{"-l", "--time-style=+%Y-%m-%d %H:%M %z", p}}} {
				var out, er bytes.Buffer
				rc := Run(context.Background(), cmd.name, &Context{Env: []string{"TZ=" + tc.tz}, Args: cmd.args, Stdout: &out, Stderr: &er})
				got := out.String()
				if cmd.name == "stat" {
					got = strings.Replace(got, ":05.000000000", "", 1)
				}
				if rc != 0 || !strings.Contains(got, tc.want) {
					t.Errorf("%s %s: rc=%d out=%q err=%q", cmd.name, tc.tz, rc, out.String(), er.String())
				}
			}
		}()
	}
	wg.Wait()
}

func TestInvocationTouchTimezone(t *testing.T) {
	dir := t.TempDir()
	var out, er bytes.Buffer
	c := &Context{Dir: dir, Env: []string{"TZ=Etc/GMT+5"}, Args: []string{"-t", "202401020304.05", "f"}, Stdout: &out, Stderr: &er}
	if Run(context.Background(), "touch", c) != 0 {
		t.Fatal(er.String())
	}
	fi, err := os.Stat(filepath.Join(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2024, 1, 2, 8, 4, 5, 0, time.UTC); !fi.ModTime().Equal(want) {
		t.Errorf("got %v want %v", fi.ModTime(), want)
	}
}

func TestInvocationLsSize(t *testing.T) {
	w, h := resolveSize(&Context{Env: []string{"COLUMNS=41", "LINES=17"}})
	if w != 41 || h != 17 {
		t.Errorf("%d %d", w, h)
	}
}

func TestInvocationDateResolutionUTC(t *testing.T) {
	var out, er bytes.Buffer
	rc := Run(context.Background(), "date", &Context{Env: []string{"TZ=Etc/GMT+6"}, Args: []string{"--resolution", "-Iseconds"}, Stdout: &out, Stderr: &er})
	if rc != 0 || out.String() != "1970-01-01T00:00:00+00:00\n" {
		t.Errorf("rc=%d out=%q err=%q", rc, out.String(), er.String())
	}
}

func TestInvocationDateUTCParsing(t *testing.T) {
	var out, er bytes.Buffer
	rc := Run(context.Background(), "date", &Context{Env: []string{"TZ=Etc/GMT+5"}, Args: []string{"-u", "-d", "2024-01-02 03:04:05", "+%H:%M %z"}, Stdout: &out, Stderr: &er})
	if rc != 0 || out.String() != "03:04 +0000\n" {
		t.Errorf("rc=%d out=%q err=%q", rc, out.String(), er.String())
	}
}

func TestInvocationTouchPosixEnv(t *testing.T) {
	dir := t.TempDir()
	var out, er bytes.Buffer
	c := &Context{Dir: dir, Env: []string{"TZ=UTC", "POSIXLY_CORRECT=1"}, Args: []string{"01020304", "f"}, Stdout: &out, Stderr: &er}
	if Run(context.Background(), "touch", c) != 0 {
		t.Fatal(er.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "01020304")); !os.IsNotExist(err) {
		t.Errorf("timestamp treated as filename: %v", err)
	}
	fi, err := os.Stat(filepath.Join(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}
	mt := fi.ModTime().UTC()
	if mt.Month() != 1 || mt.Day() != 2 || mt.Hour() != 3 || mt.Minute() != 4 {
		t.Errorf("mtime %v", mt)
	}
}

func TestInvocationStatDefaultTimezone(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, nil, 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	var out, er bytes.Buffer
	rc := Run(context.Background(), "stat", &Context{Env: []string{"TZ=Etc/GMT+5"}, Args: []string{p}, Stdout: &out, Stderr: &er})
	if rc != 0 || !strings.Contains(out.String(), "Modify: 2024-01-01 22:04:05.000000000 -0500") {
		t.Errorf("rc=%d out=%q err=%q", rc, out.String(), er.String())
	}
}

func TestInvocationLsColorEnv(t *testing.T) {
	if terminalSupportsColor(&Context{Env: []string{"TERM=dumb"}}) {
		t.Error("dumb supports color")
	}
	if !terminalSupportsColor(&Context{Env: []string{"TERM=dumb", "COLORTERM=truecolor"}}) {
		t.Error("COLORTERM ignored")
	}
}

func TestInvocationLsColorsIsolation(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.xyz")
	if err := os.WriteFile(p, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, code := range []string{"31", "32"} {
		code := code
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				var out, er bytes.Buffer
				rc := Run(context.Background(), "ls", &Context{Env: []string{"TERM=xterm", "LS_COLORS=*.xyz=" + code}, Args: []string{"--color=always", p}, Stdout: &out, Stderr: &er})
				if rc != 0 || !strings.Contains(out.String(), "\x1b["+code+"m") {
					t.Errorf("code %s rc %d out %q err %q", code, rc, out.String(), er.String())
				}
			}
		}()
	}
	wg.Wait()
}

func TestDateLegacyPosixTZ(t *testing.T) {
	var out, er bytes.Buffer
	rc := Run(context.Background(), "date", &Context{Env: []string{"TZ=ABC5DEF,M3.2.0,M11.1.0"}, Args: []string{"-d", "@1704067200", "+%z"}, Stdout: &out, Stderr: &er})
	if rc != 0 || out.String() != "+0500\n" {
		t.Errorf("%d %q %q", rc, out.String(), er.String())
	}
}

func TestInvocationCalTimezone(t *testing.T) {
	env := []string{"TZ=Etc/GMT+12"}
	loc := (&Context{Env: env}).Location()
	before := time.Now().In(loc)
	var out, er bytes.Buffer
	rc := Run(context.Background(), "cal", &Context{Env: env, Stdout: &out, Stderr: &er})
	after := time.Now().In(loc)
	if before.Year() != after.Year() || before.Month() != after.Month() {
		t.Skip("month changed during test")
	}
	if rc != 0 || !strings.Contains(out.String(), before.Format("January 2006")) {
		t.Errorf("rc=%d out=%q err=%q", rc, out.String(), er.String())
	}
}
