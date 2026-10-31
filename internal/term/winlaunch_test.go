package term

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestWindowsCommandLine(t *testing.T) {
	tests := []struct {
		argv []string
		want string
	}{
		{nil, ""},
		{[]string{"hello", "world"}, "hello world"},
		{[]string{`C:\Program Files\app.exe`, `/c`, `script name.cmd`}, `"C:\Program Files\app.exe" /c "script name.cmd"`},
		{[]string{"", "a b", "a\tb", "日本語"}, `"" "a b" "a	b" 日本語`},
		{[]string{`a"b`, `C:\some dir\`, `a\\"b`}, `"a\"b" "C:\some dir\\" "a\\\\\"b"`},
		{[]string{`plain\`, `\\`, `"`, "line\nfeed"}, `plain\ \\ "\"" line` + "\n" + `feed`},
	}
	for _, tt := range tests {
		got, err := windowsCommandLine(tt.argv)
		if err != nil || got != tt.want {
			t.Errorf("%q: got %q, %v; want %q", tt.argv, got, err, tt.want)
		}
	}
	if _, err := windowsCommandLine([]string{"bad\x00arg"}); err == nil {
		t.Fatal("NUL accepted")
	}
}

func TestWindowsEnvBlock(t *testing.T) {
	for _, tt := range []struct {
		env  []string
		want string
	}{
		{nil, "\x00\x00"},
		{[]string{"z=1", "Path=old", "A=あ", "PATH=new", "=C:=C:\\work"}, "=C:=C:\\work\x00A=あ\x00PATH=new\x00z=1\x00\x00"},
		{[]string{"x=one=two"}, "x=one=two\x00\x00"},
		{[]string{"\ue000=bmp", "\U00010000=supplementary", "ä=old", "Ä=new"}, "Ä=new\x00\U00010000=supplementary\x00\ue000=bmp\x00\x00"},
	} {
		got, err := windowsEnvBlock(tt.env)
		if err != nil || !reflect.DeepEqual(got, utf16.Encode([]rune(tt.want))) {
			t.Errorf("%q: got %q, %v; want %q", tt.env, string(utf16.Decode(got)), err, tt.want)
		}
	}
	if _, err := windowsEnvBlock([]string{"X=a\x00b"}); err == nil {
		t.Fatal("NUL accepted")
	}
}

func TestResolveWindowsExecutable(t *testing.T) {
	files := map[string]bool{}
	for _, p := range []string{`C:\work\bin\tool.EXE`, `C:\system\tool.com`, `C:\work\local.exe`, `C:\work\script.cmd`, `\\server\share\bin\tool.exe`, `C:\Windows\System32\cmd.exe`} {
		files[strings.ToLower(p)] = true
	}
	exists := func(p string) bool { return files[strings.ToLower(p)] }
	tests := []struct {
		name, dir   string
		env         []string
		want        string
		errContains string
	}{
		{"tool", `C:\work`, []string{`PATH=bin;C:\system`}, `C:\work\bin\tool.exe`, ""},
		{"tool", `C:\work`, []string{`path=C:\system`, `PATHEXT=.BAT;.COM;.EXE`}, `C:\system\tool.COM`, ""},
		{"tool", `C:\work`, []string{`PATH=\\server\share\bin`}, `\\server\share\bin\tool.exe`, ""},
		{"tool", `\\server\share\work`, []string{`PATH=..\bin`}, `\\server\share\bin\tool.exe`, ""},
		{`local`, `C:\work`, nil, "", ""}, // no implicit current-directory search
		{`.\local`, `C:\work`, nil, `C:\work\local.exe`, ""},
		{`C:\work\local.exe`, `C:\work`, nil, `C:\work\local.exe`, ""},
		{`\work\local.exe`, `C:\other`, nil, `C:\work\local.exe`, ""},
		{`script.cmd`, `C:\work`, nil, "", "I01"},
		{`script.BAT`, `C:\work`, nil, "", "I01"},
		{`C:\Windows\System32\cmd.exe`, `C:\work`, nil, `C:\Windows\System32\cmd.exe`, ""},
		{"bad\x00name", `C:\work`, nil, "", "NUL"},
		{"tool", `C:\work`, []string{`PATH=bin`, `Path=C:\system`, `PATHEXT=.com`}, `C:\system\tool.com`, ""},
		{"tool", `C:\work`, []string{`PATH="C:\system"`, `PATHEXT=.com`}, `C:\system\tool.com`, ""},
		{"tool", `C:\work`, []string{`PATH="bin"`}, `C:\work\bin\tool.exe`, ""},
		{"tool", `C:\work`, []string{`PATH="";`}, "", "not found"},
		{"tool", `C:\work`, []string{`PATH=bin`, `PATHEXT=.cmd;.bat`}, "", "not found"},
		{"tool", `C:\work`, []string{`PATH=;`, `PATHEXT=.exe`}, "", "not found"},
		{`C:local.exe`, `C:\work`, nil, "", "drive-relative"},
		{"tool", `C:\work`, []string{`PATH=C:bin;C:\system`, `PATHEXT=.com`}, `C:\system\tool.com`, ""},
		{"tool", `C:\work`, []string{`PATH=C:bin`}, "", "skipped"},
		{"tool", `C:\work`, []string{`PATH=\\broken;D:bin`}, "", "skipped"},
		{`local.txt`, `C:\work`, nil, "", "unsupported"},
		{`\\server\share\bin\tool.exe`, `C:\work`, nil, `\\server\share\bin\tool.exe`, ""},
		{`\\server\share\..\bin\tool.exe`, `C:\work`, nil, `\\server\share\bin\tool.exe`, ""},
		{`C:\..\work\local.exe`, `C:\work`, nil, `C:\work\local.exe`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name+strings.Join(tt.env, ";"), func(t *testing.T) {
			got, err := resolveWindowsExecutable(tt.name, tt.dir, tt.env, exists)
			if tt.want == "" {
				if err == nil || (tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains)) {
					t.Fatalf("got %q, %v; expected error containing %q", got, err, tt.errContains)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	_, err := resolveWindowsExecutable("tool", `C:\work`, []string{`PATH=C:bin;D:other`}, exists)
	if err == nil || !strings.Contains(err.Error(), `C:bin`) || !strings.Contains(err.Error(), `D:other`) {
		t.Fatalf("not-found error must identify skipped PATH entries: %v", err)
	}
}
