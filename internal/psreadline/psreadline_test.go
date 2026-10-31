package psreadline

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseFixtures(t *testing.T) {
	typical := []string{"Get-ChildItem", `write-host "héllo"`, "ls \n  -Force \n  | sort", "Get-Date"}
	tests := []struct {
		file string
		want []string
	}{
		{"utf8_lf.txt", typical},
		{"utf8_bom_crlf.txt", typical},
		{"utf16le_bom.txt", typical},
		{"utf16le_nobom.txt", typical},
		{"utf16be_bom.txt", typical},
		{"edge.txt", []string{"echo done\nwhat``", "last "}},
		{"empty.txt", nil},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			if got := Parse(data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseInline(t *testing.T) {
	tests := []struct {
		name, in string
		want     []string
	}{
		{"lone CR", "a\rb\r", []string{"a", "b"}},
		{"blank lines only", "\n\r\n  \n", nil},
		{"even backticks do not continue", "a``\nb", []string{"a``", "b"}},
		{"three backticks continue", "a```\nb", []string{"a``\nb"}},
		{"invalid utf-8", "a\xffb", []string{"a�b"}},
		{"plain ascii with odd length is not utf-16", "abc", []string{"abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse([]byte(tt.in)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPath(t *testing.T) {
	if Path("") != "" {
		t.Error("empty APPDATA must give no path")
	}
	want := filepath.Join("A", "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
	if got := Path("A"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
