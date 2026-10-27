package term

import (
	"os"
	"testing"
)

// FuzzProcessOutput feeds arbitrary program output and checks that where it
// is split never changes what is drawn, and that around a resize the
// terminal neither panics, leaves the cursor off the screen nor grows its
// scrollback past the limit. Run longer with
// go test -fuzz FuzzProcessOutput ./internal/term/
func FuzzProcessOutput(f *testing.F) {
	for _, s := range []string{
		"hello\r\nworld",
		"\x1b[2J\x1b[H\x1b[38;2;1;2;3mx\x1b[m",
		"\x1b[?1049h\x1b[10;20r\x1b[5S\x1b[3T\x1b[?1049l",
		"\x1b[?2026hframe\x1b[?2026l",
		"\x1b]0;title\x07\x1b]52;c;aGk=\x1b\\",
		"\x1b[6n\x1b[c\x1b[>c\x1b]11;?\x07",
		"é̂世界‍\U0001F600\t\x1b[4h\x1b[2@\x1b[3P\x1b[2L\x1b[4M",
		"\x1b(0lqk\x1b(B\x1b#8\x1b[99999;99999H\x1b[99999X",
		"\x1bP+q544e\x1b\\\x1b[?25l\x1b7\x1b8\x1bc",
	} {
		f.Add([]byte(s), uint16(len(s)/2), uint8(3), uint8(7))
	}
	if data, err := os.ReadFile("testdata/btop-100x30.vt"); err == nil {
		f.Add(data, uint16(len(data)/2), uint8(30), uint8(100))
	}
	f.Fuzz(func(t *testing.T, data []byte, split uint16, rows, cols uint8) {
		cut := min(int(split), len(data))
		whole, pieces := New(10, 20), New(10, 20)
		defer whole.Close()
		defer pieces.Close()
		whole.ProcessOutput(data)
		whole.Finish()
		pieces.ProcessOutput(data[:cut])
		pieces.ProcessOutput(data[cut:])
		pieces.Finish()
		if a, b := whole.RenderFull(true, nil), pieces.RenderFull(true, nil); a != b {
			t.Fatalf("output split at %d renders differently:\n%q\n%q", cut, a, b)
		}

		r, c := int(rows%40)+1, int(cols%120)+1
		tm := New(10, 20)
		defer tm.Close()
		tm.ProcessOutput(data[:cut])
		tm.Resize(r, c)
		tm.ProcessOutput(data[cut:])
		tm.Finish()
		if row, col := tm.CursorRow(), tm.CursorCol(); row < 0 || row >= r || col < 0 || col > c {
			t.Fatalf("cursor at %d,%d on a %dx%d screen", row, col, r, c)
		}
		if n := len(tm.scrollback); n > maxScrollback+scrollbackSlack {
			t.Fatalf("scrollback holds %d rows", n)
		}
		tm.RenderFull(true, nil)
		var s Snapshot
		tm.Snapshot(&s)
	})
}
