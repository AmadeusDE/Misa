package vi

import "testing"

func TestComplexMotionFindPure(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"axaxax"}
	e.topLine = 3
	m, ok := e.complexMotion("tx", 2, true)
	if !ok || m != (motionResult{0, 2, rangeInclusive}) || e.col != 0 || e.topLine != 3 {
		t.Fatalf("2tx: %+v %v cursor=%d", m, ok, e.col)
	}
	e.col = 2
	m, ok = e.complexMotion(";", 1, false)
	if !ok || m.col != 4 {
		t.Fatalf("repeat till: %+v %v", m, ok)
	}
	e.col = 5
	m, ok = e.complexMotion("Fx", 1, false)
	if !ok || m.col != 3 || m.kind != rangeExclusive {
		t.Fatalf("Fx: %+v %v", m, ok)
	}
	m, ok = e.complexMotion(",", 1, false)
	if ok || e.lastFCmd != 'F' {
		t.Fatalf("comma should preserve direction: %+v %v %c", m, ok, e.lastFCmd)
	}
}

func TestComplexMotionBracketMarksPercent(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{" a (x", "(y))", "   z"}
	e.marks['a'] = pos{2, 99}
	m, ok := e.complexMotion("%", 1, false)
	if !ok || m != (motionResult{1, 3, rangeInclusive}) || e.col != 0 {
		t.Fatalf("bracket: %+v %v", m, ok)
	}
	m, ok = e.complexMotion("%", 50, true)
	if !ok || m.row != 1 || m.kind != rangeLinewise {
		t.Fatalf("percent: %+v %v", m, ok)
	}
	if _, ok = e.complexMotion("%", 101, true); ok {
		t.Fatal("101% succeeded")
	}
	m, ok = e.complexMotion("`a", 1, false)
	if !ok || m != (motionResult{2, 3, rangeExclusive}) {
		t.Fatalf("mark: %+v %v", m, ok)
	}
	m, ok = e.complexMotion("'a", 1, false)
	if !ok || m != (motionResult{2, 3, rangeLinewise}) {
		t.Fatalf("line mark: %+v %v", m, ok)
	}
}

func TestSearchMotionOverlapAndCycle(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"ababa", ""}
	e.searchPat = "aba"
	e.col = 3
	m, ok := e.searchMotion(1, true)
	if !ok || m != (motionResult{0, 2, rangeExclusive}) || e.col != 3 {
		t.Fatalf("backward overlap: %+v %v", m, ok)
	}
	m, ok = e.searchMotion(1000000000, false)
	if !ok || m.col != 2 || e.col != 3 {
		t.Fatalf("cycle: %+v %v", m, ok)
	}
}

func TestComplexMotionFailureIsPure(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"  (", ""}
	e.col = 0
	e.topLine = 4
	e.jumpList = []pos{{0, 0}}
	if _, ok := e.complexMotion("%", 1, false); ok {
		t.Fatal("unmatched bracket succeeded")
	}
	if e.col != 0 || e.row != 0 || e.topLine != 4 || len(e.jumpList) != 1 {
		t.Fatal("failed bracket mutated editor")
	}
	if _, ok := e.complexMotion("'z", 1, false); ok || e.statusMsg == "" {
		t.Fatal("unset mark must fail with status")
	}
}

func TestSearchMotionUnicodeAndEmptyLines(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"", "ÅåÅ", ""}
	e.searchPat = "åå\\c"
	m, ok := e.searchMotion(1, false)
	if !ok || m.row != 1 || m.col != 0 {
		t.Fatalf("fold search: %+v %v", m, ok)
	}
	e.row, e.col = 1, 2
	m, ok = e.searchMotion(1, true)
	if !ok || m.row != 1 || m.col != 1 {
		t.Fatalf("fold overlap: %+v %v", m, ok)
	}
}

func TestComplexMotionUppercaseMarkAndFailureStatus(t *testing.T) {
	e := New("", t.TempDir())
	e.lines = []string{"abc", "  z"}
	e.marks['A'] = pos{1, 2}
	for _, tc := range []struct {
		key  string
		want motionResult
	}{{"`A", motionResult{1, 2, rangeExclusive}}, {"'A", motionResult{1, 2, rangeLinewise}}} {
		got, ok := e.complexMotion(tc.key, 1, false)
		if !ok || got != tc.want {
			t.Errorf("%s = %+v, %v", tc.key, got, ok)
		}
	}
	for _, tc := range []struct {
		key   string
		count int
		given bool
	}{{"fz", 1, false}, {"%", 1, false}, {"%", 101, true}, {";", 1, false}} {
		e.statusMsg = ""
		if _, ok := e.complexMotion(tc.key, tc.count, tc.given); ok || e.statusMsg == "" {
			t.Errorf("%s failure status = %q", tc.key, e.statusMsg)
		}
	}
	e.statusMsg = ""
	if _, ok := e.complexMotion("unknown", 1, false); ok || e.statusMsg != "" {
		t.Fatal("unknown motion was not silent")
	}
}
