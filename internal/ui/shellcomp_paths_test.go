package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCompleteFSSeparators(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"sub/deep", "src"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"sub/file.txt", "sub/fix.txt", "top.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name, partial string
		windows       bool
		want          []string
	}{
		{"unix top level", "s", false, []string{"src/", "sub/"}},
		{"unix slash", "sub/f", false, []string{"sub/file.txt", "sub/fix.txt"}},
		{"unix nested", "sub/", false, []string{"sub/deep/", "sub/file.txt", "sub/fix.txt"}},
		{"unix backslash is a filename character", `sub\f`, false, nil},
		{"windows slash", "sub/fi", true, []string{"sub/file.txt", "sub/fix.txt"}},
		{"windows backslash", `sub\fi`, true, []string{`sub\file.txt`, `sub\fix.txt`}},
		{"windows backslash directory", `s`, true, []string{"src/", "sub/"}},
		{"windows backslash then directory", `sub\d`, true, []string{`sub\deep\`}},
		{"windows mixed", `sub/deep\`, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := completeFS(tt.partial, dir, tt.windows)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("completeFS(%q, windows=%v) = %q, want %q", tt.partial, tt.windows, got, tt.want)
			}
		})
	}
}

func TestCompleteFSAbsoluteUnix(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "abc"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := completeFS(filepath.ToSlash(dir)+"/a", t.TempDir(), false)
	if want := []string{filepath.ToSlash(dir) + "/abc"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPSReadLineEntries(t *testing.T) {
	appData := t.TempDir()
	dir := filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "ConsoleHost_history.txt")
	body := "\xef\xbb\xbfGet-Date\r\nls `\r\n  -Force\r\n\r\n"
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := psReadLineEntries(appData)
	want := []histEntry{{cmd: "Get-Date"}, {cmd: "ls -Force"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
	after, _ := os.ReadFile(file)
	if string(after) != body {
		t.Error("PSReadLine history was modified")
	}
	if psReadLineEntries("") != nil || psReadLineEntries(t.TempDir()) != nil {
		t.Error("a missing history file must give no entries")
	}
}

func TestHistoryLocationAndFormat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	want := filepath.Join(home, ".local", "share", "misa", "history")
	if onWindows {
		want = filepath.Join(home, "local", "misa", "history")
	}
	if got := historyPath(); got != want {
		t.Fatalf("historyPath = %q, want %q", got, want)
	}
	saveHistoryEntries([]histEntry{{ts: 5, cmd: "echo hi"}})
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != ": 5:0;echo hi\n" {
		t.Errorf("history file = %q", data)
	}
	if got := loadHistoryEntries(); !reflect.DeepEqual(got, []histEntry{{ts: 5, cmd: "echo hi"}}) {
		t.Errorf("loaded %v", got)
	}
	if strings.Contains(want, "xdg") {
		t.Error("XDG_DATA_HOME must not move the history file")
	}
}
