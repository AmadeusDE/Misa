package builtins

import (
	"os"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/lscore"
)

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func windowsLikeMeta() *fsMeta {
	return &fsMeta{
		Dev: 7, Ino: 42, Nlink: 2, Mode: 0100666,
		Atime:   fsTimespec{Sec: 100, Nsec: 5},
		Mtime:   fsTimespec{Sec: 200, Nsec: 6},
		Missing: lscore.FieldOwner | lscore.FieldGroup | lscore.FieldBlocks | lscore.FieldChangeTime,
	}
}

func TestStatFormatPrintsQuestionMarkForMissingFields(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f", "abc")
	info := mustStat(t, path)
	cases := map[string]string{
		"%u": "?", "%g": "?", "%U": "?", "%G": "?", "%b": "?", "%o": "?",
		"%z": "?", "%Z": "?", "%i": "42", "%h": "2", "%d": "7", "%X": "100", "%s": "3",
	}
	for format, want := range cases {
		got, err := statFormat(format, "f", info, windowsLikeMeta())
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", format, got, want)
		}
	}
}

func TestStatFormatKeepsPresentFieldsAtNanosecondPrecision(t *testing.T) {
	info := mustStat(t, writeFile(t, t.TempDir(), "f", ""))
	got, err := statFormat("%.9Y", "f", info, windowsLikeMeta())
	if err != nil || got != "200.000000006" {
		t.Errorf("%%.9Y = %q, %v", got, err)
	}
}

func TestStatDefaultViewMarksMissingFields(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f", "abc")
	var out strings.Builder
	statPrintDefault(&Context{Stdout: &out}, "f", mustStat(t, path), windowsLikeMeta())
	for _, want := range []string{"Blocks: ?", "Uid: (    ?/ ?)", "Gid: (    ?/ ?)", "Change: ?", "Links: 2"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("default view lacks %q:\n%s", want, out.String())
		}
	}
}

func TestPosixModeBuildsTypeBits(t *testing.T) {
	info := mustStat(t, t.TempDir())
	if got := posixMode(info.Mode()); got&0170000 != 0040000 {
		t.Errorf("posixMode(dir) = %o", got)
	}
}
