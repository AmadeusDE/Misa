package vi

import (
	"strings"
	"testing"
)

func countKeys(e *Editor, keys ...string) {
	for _, k := range keys {
		e.Update(keyFromName(k))
	}
}
func TestCountStatePrefixes(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"a", "b", "c", "d", "e", "f"}
	countKeys(e, "5", "g", "g")
	if e.row != 4 {
		t.Errorf("5gg row=%d", e.row)
	}
	e.lines = []string{"axaxax"}
	e.row = 0
	e.col = 0
	countKeys(e, "2", "f", "x")
	if e.col != 3 {
		t.Errorf("2fx col=%d", e.col)
	}
	e.lines = []string{"abcdef"}
	e.col = 0
	countKeys(e, "3", "r", "x")
	if e.lines[0] != "xxxdef" {
		t.Errorf("3rx=%q", e.lines[0])
	}
}
func TestCountStateEdits(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"one two three four five six seven"}
	countKeys(e, "2", "d", "3", "w")
	if e.lines[0] != "seven" {
		t.Errorf("2d3w=%q", e.lines[0])
	}
	e.lines = []string{"a", "b", "c"}
	e.row = 0
	e.col = 0
	countKeys(e, "\"", "a", "2", "y", "y", "\"", "a", "p")
	if strings.Join(e.lines, ",") != "a,a,b,b,c" {
		t.Errorf("register put=%q", e.lines)
	}
	e.lines = []string{"abc"}
	e.row = 0
	e.col = 0
	e.register = []string{"z"}
	e.regLine = false
	countKeys(e, "3", "p")
	if e.lines[0] != "azzzbc" {
		t.Errorf("3p=%q", e.lines[0])
	}
}
func TestCountStateCancel(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"abcd"}
	countKeys(e, "\"", "a", "2", "d", "esc", "x")
	if e.lines[0] != "bcd" || e.hasReg || e.pendingOp != 0 {
		t.Errorf("escape: %q reg=%v op=%c", e.lines[0], e.hasReg, e.pendingOp)
	}
}

func TestCountStateLargeAndInvalid(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"alpha beta", "gamma", "delta"}
	countKeys(e, "\"", "a", "d", "!", "x")
	if e.lines[0] != "lpha beta" || e.hasReg {
		t.Fatalf("invalid operator leaked state: %q, reg=%v", e.lines[0], e.hasReg)
	}
	countKeys(e, "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "x")
	if e.lines[0] != "" {
		t.Errorf("large x: %q", e.lines[0])
	}
	countKeys(e, "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "b")
	if e.row != 0 {
		t.Errorf("large b row=%d", e.row)
	}
}

func TestCountStateCountedActions(t *testing.T) {
	for _, tc := range []struct {
		keys  []string
		lines []string
		want  string
		mode  Mode
	}{
		{[]string{"2", "D"}, []string{"abc", "def", "ghi"}, "|ghi", ModeNormal},
		{[]string{"2", "C"}, []string{"abc", "def", "ghi"}, "|ghi", ModeInsert},
		{[]string{"2", "S"}, []string{"abc", "def", "ghi"}, "|ghi", ModeInsert},
		{[]string{"2", "s"}, []string{"abc", "def", "ghi"}, "c|def|ghi", ModeInsert},
		{[]string{"3", "J"}, []string{"abc", "def", "ghi", "jkl"}, "abc def ghi|jkl", ModeNormal},
	} {
		e := New("", t.TempDir())
		e.lines = append([]string(nil), tc.lines...)
		countKeys(e, tc.keys...)
		if got := strings.Join(e.lines, "|"); got != tc.want || e.mode != tc.mode {
			t.Errorf("%v: %q mode %v", tc.keys, got, e.mode)
		}
	}
}

func TestCountStatePositionBeyondContentLimit(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = make([]string, 11050)
	countKeys(e, "1", "1", "0", "0", "1", "g", "g")
	if e.row != 11000 {
		t.Fatalf("11001gg row=%d", e.row)
	}
	countKeys(e, "9", "9", "9", "9", "9", "9", "9", "9", "9", "9", "j")
	if e.row != 11049 {
		t.Fatalf("large j row=%d", e.row)
	}
}

func TestCountStateWrappingSearch(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"x x x"}
	e.searchPat = "x"
	e.searchDir = 1
	countKeys(e, "1", "0", "0", "0", "0", "0", "0", "0", "1", "n")
	if e.col != 4 {
		t.Fatalf("large wrapping search col=%d", e.col)
	}
	countKeys(e, "1", "0", "0", "0", "0", "0", "0", "0", "1", "N")
	if e.col != 0 {
		t.Fatalf("large reverse wrapping search col=%d", e.col)
	}
}
