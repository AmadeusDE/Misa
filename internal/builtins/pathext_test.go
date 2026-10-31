package builtins

import (
	"reflect"
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
