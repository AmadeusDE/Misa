package hostpath

import (
	"os"
	"path/filepath"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDataDir(t *testing.T) {
	tests := []struct {
		name, goos, home string
		env              map[string]string
		want             string
	}{
		{"unix", "linux", "/home/a", nil, filepath.Join("/home/a", ".local", "share", "misa")},
		{"unix ignores XDG_DATA_HOME", "linux", "/home/a", map[string]string{"XDG_DATA_HOME": "/x"}, filepath.Join("/home/a", ".local", "share", "misa")},
		{"unix without home", "linux", "", nil, ""},
		{"windows local app data", "windows", `C:\Users\a`, map[string]string{"LOCALAPPDATA": `D:\L`}, filepath.Join(`D:\L`, "misa")},
		{"windows fallback", "windows", `C:\Users\a`, nil, filepath.Join(`C:\Users\a`, "AppData", "Local", "misa")},
		{"windows without anything", "windows", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dataDir(tt.goos, env(tt.env), tt.home); got != tt.want {
				t.Errorf("dataDir = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostLocations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	t.Setenv("AppData", filepath.Join(home, "cfg"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
	if got, want := ConfigDir(), filepath.Join(home, "cfg", "misa"); got != want {
		t.Errorf("ConfigDir = %q, want %q", got, want)
	}
	if got, want := HistoryFile(), filepath.Join(DataDir(), "history"); got != want {
		t.Errorf("HistoryFile = %q, want %q", got, want)
	}
	if got, want := TldrDir(), filepath.Join(DataDir(), "tldr"); got != want {
		t.Errorf("TldrDir = %q, want %q", got, want)
	}
}

func TestFindFileIgnoresWorkingDirectory(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("AppData", cfg)
	cwd := t.TempDir()
	t.Chdir(cwd)
	os.WriteFile(filepath.Join(cwd, "x.yaml"), nil, 0o644)
	if got := FindFile("x.yaml"); got != "" {
		t.Fatalf("FindFile found %q in the working directory", got)
	}
	want := filepath.Join(cfg, "misa", "x.yaml")
	os.MkdirAll(filepath.Dir(want), 0o755)
	os.WriteFile(want, nil, 0o644)
	if got := FindFile("x.yaml"); got != want {
		t.Fatalf("FindFile = %q, want %q", got, want)
	}
}

func TestUsername(t *testing.T) {
	cur := func(s string) func() string { return func() string { return s } }
	tests := []struct {
		name, goos string
		env        map[string]string
		current    string
		want       string
	}{
		{"unix USER", "linux", map[string]string{"USER": "u", "LOGNAME": "l"}, "c", "u"},
		{"unix LOGNAME", "linux", map[string]string{"LOGNAME": "l"}, "c", "l"},
		{"unix ignores USERNAME", "linux", map[string]string{"USERNAME": "w"}, "c", ""},
		{"windows USERNAME", "windows", map[string]string{"USERNAME": "w", "USER": "u"}, "", "w"},
		{"windows domain stripped", "windows", nil, `PC\amy`, "amy"},
		{"windows bare", "windows", nil, "amy", "amy"},
		{"windows none", "windows", nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := username(tt.goos, env(tt.env), cur(tt.current)); got != tt.want {
				t.Errorf("username = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAccountName(t *testing.T) {
	if accountName("windows", `PC\amy`) != "amy" || accountName("windows", "amy") != "amy" || accountName("linux", `a\b`) != `a\b` {
		t.Error("accountName mismatch")
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		in        string
		win       bool
		dir, base string
	}{
		{"foo", false, "", "foo"},
		{"a/b", false, "a/", "b"},
		{"/", false, "/", ""},
		{`a\b`, false, "", `a\b`},
		{`a\b`, true, `a\`, "b"},
		{`a/b\c`, true, `a/b\`, "c"},
		{`C:\`, true, `C:\`, ""},
		{`C:\Win`, true, `C:\`, "Win"},
		{`C:/Win/Sys`, true, `C:/Win/`, "Sys"},
		{`C:`, true, `C:`, ""},
		{`c:foo`, true, `c:`, "foo"},
		{`C:foo`, false, "", "C:foo"},
		{`\\srv\share\f`, true, `\\srv\share\`, "f"},
		{`\\srv\sh`, true, `\\srv\`, "sh"},
		{"", true, "", ""},
	}
	for _, tt := range tests {
		dir, base := SplitPath(tt.in, tt.win)
		if dir != tt.dir || base != tt.base {
			t.Errorf("SplitPath(%q, %v) = %q, %q; want %q, %q", tt.in, tt.win, dir, base, tt.dir, tt.base)
		}
	}
}

func TestIsAbs(t *testing.T) {
	tests := []struct {
		in   string
		win  bool
		want bool
	}{
		{"/x", false, true}, {"x", false, false}, {`C:\`, false, false},
		{`C:\`, true, true}, {`c:/x`, true, true}, {`C:`, true, false}, {`C:x`, true, false},
		{`\\srv\share`, true, true}, {`//srv/share`, true, true}, {`\x`, true, false}, {`x\y`, true, false},
	}
	for _, tt := range tests {
		if got := IsAbs(tt.in, tt.win); got != tt.want {
			t.Errorf("IsAbs(%q, %v) = %v", tt.in, tt.win, got)
		}
	}
}

func TestSeparator(t *testing.T) {
	if Separator(`a\b`, true) != `\` || Separator("a/b", true) != "/" || Separator(`a\b`, false) != "/" || Separator("", true) != "/" {
		t.Error("Separator mismatch")
	}
}
