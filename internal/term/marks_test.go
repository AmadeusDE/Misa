package term

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCombiningMarksSurviveAsMarks(t *testing.T) {
	tm := New(1, 4)
	tm.ProcessOutput([]byte("é̂x"))
	var s Snapshot
	tm.Snapshot(&s)
	if got := s.Cells[0].Marks(); got != "́̂" {
		t.Fatalf("marks = %q", got)
	}
	if s.Cells[1].Marks() != "" {
		t.Fatal("marks leaked to the next cell")
	}
	if tm.PlainScreen()[0] != "é̂x" {
		t.Fatalf("plain = %q", tm.PlainScreen()[0])
	}
}

func TestEqualMarksCompareEqual(t *testing.T) {
	a, b := New(1, 2), New(1, 2)
	a.ProcessOutput([]byte("à"))
	b.ProcessOutput([]byte("à"))
	var sa, sb Snapshot
	a.Snapshot(&sa)
	b.Snapshot(&sb)
	if sa.Cells[0] != sb.Cells[0] {
		t.Fatal("identical cells differ")
	}
}

func TestOverwritingClearsMarks(t *testing.T) {
	tm := New(1, 2)
	tm.ProcessOutput([]byte("à\rb"))
	var s Snapshot
	tm.Snapshot(&s)
	if s.Cells[0].Marks() != "" {
		t.Fatal("marks kept after overwrite")
	}
}

func TestCellHasNoPointers(t *testing.T) {
	var walk func(reflect.Type) bool
	walk = func(rt reflect.Type) bool {
		switch rt.Kind() {
		case reflect.Pointer, reflect.String, reflect.Slice, reflect.Map, reflect.Interface, reflect.Chan, reflect.Func:
			return true
		case reflect.Struct:
			for i := 0; i < rt.NumField(); i++ {
				if walk(rt.Field(i).Type) {
					return true
				}
			}
		}
		return false
	}
	if walk(reflect.TypeOf(Cell{})) {
		t.Fatal("Cell holds a pointer, so screens and scrollback are scanned by the GC")
	}
}

func TestScrollbackKeepsTheNewestLines(t *testing.T) {
	tm := New(2, 6)
	total := maxScrollback + scrollbackSlack + 50
	for i := 0; i < total; i++ {
		tm.ProcessOutput([]byte(fmt.Sprintf("%d\r\n", i)))
	}
	lines := tm.PlainLines()
	if n := tm.ScrollbackLen(); n < maxScrollback || n >= maxScrollback+scrollbackSlack {
		t.Fatalf("scrollback holds %d rows", n)
	}
	if last := lines[len(lines)-1]; last != fmt.Sprint(total-1) {
		t.Fatalf("newest line = %q", last)
	}
	for i := 1; i < len(lines); i++ {
		var a, b int
		fmt.Sscan(lines[i-1], &a)
		fmt.Sscan(lines[i], &b)
		if b != a+1 {
			t.Fatalf("lines %d,%d not consecutive: %q %q (a recycled row was still in use)", i-1, i, lines[i-1], lines[i])
		}
	}
}

func TestScrollingDoesNotAllocateRows(t *testing.T) {
	tm := New(24, 80)
	line := []byte("0123456789\r\n")
	for i := 0; i < maxScrollback+scrollbackSlack; i++ {
		tm.ProcessOutput(line)
	}
	allocs := testing.AllocsPerRun(200, func() { tm.ProcessOutput(line) })
	if allocs > 1 {
		t.Fatalf("%.1f allocations per scrolled line", allocs)
	}
}
