package toybox

import (
	"strings"
	"testing"
)

func TestFindParser(t *testing.T) {
	source := "mkdir dir\ncd dir\ntouch file\ncd ..\ntesting \"cwd\" \\\n  \"cd dir; find\" \".\\n./file\\n\" \"\" \"\"\nln -s file dir/link\ntesting 'env' 'LANG=C find dir' 'dir\\n' '' ''\n"
	cases, err := parseFindCases(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases", len(cases))
	}
	if cases[0].command != "cd dir; find" || cases[0].expected != ".\n./file\n" || strings.Contains(cases[0].setup, "ln -s") {
		t.Fatalf("first: %+v", cases[0])
	}
	if cases[1].command != "LANG=C find dir" || !strings.Contains(cases[1].setup, "ln -s file dir/link") {
		t.Fatalf("second: %+v", cases[1])
	}
}
