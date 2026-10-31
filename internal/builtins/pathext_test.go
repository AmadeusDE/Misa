package builtins

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPathextNames(t *testing.T) {
	cases := []struct {
		name, pathext string
		want          []string
	}{
		{"git", ".EXE;.CMD", []string{"git.exe", "git.cmd"}},
		{"git.EXE", ".exe;.cmd", []string{"git.EXE"}},
		{"build.Cmd", ".EXE;.CMD", []string{"build.Cmd"}},
		{"python3.11", ".EXE", []string{"python3.11.exe"}},
		{"notes.txt", ".EXE", []string{"notes.txt.exe"}},
		{"tool", "", []string{"tool.com", "tool.exe", "tool.bat", "tool.cmd"}},
		{"tool", " .EXE ;;.BAT", []string{"tool.exe", "tool.bat"}},
	}
	for _, c := range cases {
		if got := pathextNames(c.name, c.pathext); !reflect.DeepEqual(got, c.want) {
			t.Errorf("pathextNames(%q,%q) = %v; want %v", c.name, c.pathext, got, c.want)
		}
	}
}

func TestRootedOnVolume(t *testing.T) {
	cases := []struct {
		dir, path string
		windows   bool
		ok        bool
	}{
		{`C:\work`, `/no/such`, true, true},
		{`D:\work`, `\x`, true, true},
		{`C:\work`, `E:\x`, true, false},
		{`C:\work`, `\\host\share\x`, true, false},
		{`C:\work`, `rel`, true, false},
		{`/work`, `/x`, false, false},
	}
	for _, c := range cases {
		got, ok := rootedOnVolume(c.dir, c.path, c.windows)
		if ok != c.ok {
			t.Errorf("rootedOnVolume(%q,%q,%v) ok=%v", c.dir, c.path, c.windows, ok)
		}
		if ok && !strings.HasPrefix(got, c.dir[:2]) {
			t.Errorf("rootedOnVolume(%q,%q) = %q, lost the volume", c.dir, c.path, got)
		}
	}
}

func TestIsFilesystemRoot(t *testing.T) {
	if !isFilesystemRoot(string(filepath.Separator)) {
		t.Error("separator alone is a root")
	}
	for _, p := range []string{"", ".", "a", filepath.Join("a", "b")} {
		if isFilesystemRoot(p) {
			t.Errorf("%q is not a root", p)
		}
	}
}

func TestBaseNameRoot(t *testing.T) {
	if baseName("/") != "/" || baseName("/usr/bin/") != "bin" {
		t.Fatal("baseName")
	}
}
