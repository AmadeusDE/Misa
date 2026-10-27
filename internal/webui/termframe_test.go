package webui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/AmadeusDE/Misa/internal/term"
)

func screenOf(rows, cols int, out string) *term.Terminal {
	t := term.New(rows, cols)
	t.ProcessOutput([]byte(out))
	return t
}

func snapOf(t *term.Terminal) *term.Snapshot {
	var s term.Snapshot
	t.Snapshot(&s)
	return &s
}

func rowText(runs []Run) string {
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.T)
	}
	return sb.String()
}

func TestEncodeRowMergesStylesAndTrimsBlanks(t *testing.T) {
	s := snapOf(screenOf(2, 20, "ab\x1b[1;31mcd\x1b[0mef"))
	runs := encodeRow(s.Cells[:20], newTermPalette())
	want := []Run{{T: "ab"}, {T: "cd", FG: gruvboxDark[1].hex(), A: attrBold}, {T: "ef"}}
	if !reflect.DeepEqual(runs, want) {
		t.Fatalf("runs = %+v, want %+v", runs, want)
	}
	if got := encodeRow(s.Cells[20:], newTermPalette()); len(got) != 0 {
		t.Fatalf("blank row = %+v", got)
	}
}

func TestEncodeRowWideAndCombining(t *testing.T) {
	s := snapOf(screenOf(1, 10, "a漢éb"))
	runs := encodeRow(s.Cells, newTermPalette())
	want := []Run{{T: "a"}, {T: "漢", A: attrWide}, {T: "éb"}}
	if !reflect.DeepEqual(runs, want) {
		t.Fatalf("runs = %+v, want %+v", runs, want)
	}
}

func TestEncodeRowKeepsStyledBlanks(t *testing.T) {
	s := snapOf(screenOf(1, 6, "x\x1b[44m  \x1b[0m"))
	runs := encodeRow(s.Cells, newTermPalette())
	if len(runs) != 2 || runs[1].T != "  " || runs[1].BG != gruvboxDark[4].hex() {
		t.Fatalf("runs = %+v", runs)
	}
}

func TestPaletteResolution(t *testing.T) {
	p := newTermPalette()
	cases := []struct {
		name, sgr, fg, bg string
	}{
		{"default", "", "", ""},
		{"indexed", "\x1b[32m", gruvboxDark[2].hex(), ""},
		{"bright", "\x1b[92m", gruvboxDark[10].hex(), ""},
		{"256 cube", "\x1b[38;5;196m", "#ff0000", ""},
		{"256 grey", "\x1b[48;5;240m", "", "#585858"},
		{"truecolor", "\x1b[38;2;1;2;3;48;2;250;251;252m", "#010203", "#fafbfc"},
		{"reverse default", "\x1b[7m", gruvboxDark[0].hex(), gruvboxDark[15].hex()},
		{"reverse colour", "\x1b[31;7m", gruvboxDark[0].hex(), gruvboxDark[1].hex()},
	}
	for _, c := range cases {
		s := snapOf(screenOf(1, 4, c.sgr+"x"))
		fg, bg := p.resolve(&s.Cells[0])
		if fg != c.fg || bg != c.bg {
			t.Errorf("%s: fg=%q bg=%q, want %q %q", c.name, fg, bg, c.fg, c.bg)
		}
	}
}

func TestPaletteDimBlendsTowardBackground(t *testing.T) {
	s := snapOf(screenOf(1, 4, "\x1b[2mx"))
	fg, _ := newTermPalette().resolve(&s.Cells[0])
	if fg == "" || fg == gruvboxDark[15].hex() {
		t.Fatalf("dim fg = %q", fg)
	}
}

func TestFrameDiffSendsOnlyChangedRows(t *testing.T) {
	tm := screenOf(3, 10, "one\r\ntwo")
	enc := newFrameEncoder()
	full := enc.full(snapOf(tm), false, "", 1)
	if full.Type != "full" || len(full.Lines) != 3 || rowText(full.Lines[0]) != "one" || full.Seq != 1 {
		t.Fatalf("full = %+v", full)
	}
	if _, changed := enc.diff(snapOf(tm), false, "", 1); changed {
		t.Fatal("identical screen reported a change")
	}
	tm.ProcessOutput([]byte("\x1b[3;1Hxyz"))
	f, changed := enc.diff(snapOf(tm), false, "", 1)
	if !changed || f.Type != "rows" || len(f.Dirty) != 1 || f.Dirty[0].Y != 2 || rowText(f.Dirty[0].Runs) != "xyz" {
		t.Fatalf("diff = %+v", f)
	}
	if f.Cur == nil {
		t.Fatal("cursor moved but was not sent")
	}
}

func TestFrameDiffCursorMouseTitleOnly(t *testing.T) {
	tm := screenOf(2, 5, "")
	enc := newFrameEncoder()
	enc.full(snapOf(tm), false, "", 1)
	tm.ProcessOutput([]byte("\x1b[2;3H"))
	f, changed := enc.diff(snapOf(tm), true, "vim", 1)
	if !changed || len(f.Dirty) != 0 || f.Cur == nil || f.Cur.Row != 1 || f.Cur.Col != 2 ||
		f.Mouse == nil || !*f.Mouse || f.Title == nil || *f.Title != "vim" {
		t.Fatalf("diff = %+v", f)
	}
	if _, changed := enc.diff(snapOf(tm), true, "vim", 1); changed {
		t.Fatal("second diff should be empty")
	}
}

func TestFrameDiffResizeSendsFull(t *testing.T) {
	tm := screenOf(2, 5, "hi")
	enc := newFrameEncoder()
	enc.full(snapOf(tm), false, "", 1)
	tm.Resize(3, 7)
	f, changed := enc.diff(snapOf(tm), false, "", 1)
	if !changed || f.Type != "full" || f.Rows != 3 || f.Cols != 7 {
		t.Fatalf("diff = %+v", f)
	}
}
