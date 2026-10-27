package term

import "testing"

func TestSnapshotCopiesScreen(t *testing.T) {
	tm := New(2, 4)
	tm.ProcessOutput([]byte("ab\r\n\x1b[1m世"))
	var s Snapshot
	tm.Snapshot(&s)
	if s.Rows != 2 || s.Cols != 4 || len(s.Cells) != 8 {
		t.Fatalf("size %dx%d, %d cells", s.Rows, s.Cols, len(s.Cells))
	}
	if s.Cells[0].Ch != 'a' || s.Cells[1].Ch != 'b' {
		t.Errorf("row 0 = %q%q", s.Cells[0].Ch, s.Cells[1].Ch)
	}
	wide := s.Cells[4]
	if wide.Ch != '世' || !wide.Wide || !wide.Bold || !s.Cells[5].Continuation() {
		t.Errorf("wide cell = %+v, next continuation %v", wide, s.Cells[5].Continuation())
	}
	if s.CursorRow != 1 || s.CursorCol != 2 || !s.CursorVisible {
		t.Errorf("cursor %d,%d visible %v", s.CursorRow, s.CursorCol, s.CursorVisible)
	}
	tm.ProcessOutput([]byte("x"))
	if s.Cells[6].Ch == 'x' {
		t.Error("snapshot aliases the live screen")
	}
}

func TestOSCHookGetsUnansweredStrings(t *testing.T) {
	tm := New(2, 4)
	var got []string
	tm.SetOSCHook(func(osc string) { got = append(got, osc) })
	tm.ProcessOutput([]byte("\x1b]2;hello\a\x1b]52;c;aGk=\x1b\\"))
	if len(got) != 2 || got[0] != "2;hello" || got[1] != "52;c;aGk=" {
		t.Fatalf("hook got %q", got)
	}
}
