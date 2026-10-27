package term

import (
	"math/rand/v2"
	"testing"
)

// fastPathStreams mix plain runs with what the fast path must defer: wide
// characters it overwrites, wrapping at the last column, line-drawing
// charsets, attributes and cursor moves.
var fastPathStreams = []string{
	"hello world\r\nsecond line that is long enough to wrap past the edge\r\n",
	"世界世界\r\x1b[1Cab\r\x1b[3Cz",
	"abcdefghij\x1b[1;10Hxyz",
	"\x1b(0lqqk\x1b(B plain \x0e\x1b)0qq\x0f qq",
	"\x1b[31mred\x1b[1mbold\x1b[m\tTab\bX",
	"\x1b[?7lno autowrap past the edge of the screen\x1b[?7h",
}

func screenWith(data string, chunk int, fast bool) Snapshot {
	asciiFastPath = fast
	defer func() { asciiFastPath = true }()
	tm := New(4, 10)
	for i := 0; i < len(data); i += chunk {
		tm.ProcessOutput([]byte(data[i:min(i+chunk, len(data))]))
	}
	var s Snapshot
	tm.Snapshot(&s)
	return s
}

func sameScreen(a, b Snapshot) bool {
	if a.CursorRow != b.CursorRow || a.CursorCol != b.CursorCol || len(a.Cells) != len(b.Cells) {
		return false
	}
	for i := range a.Cells {
		if a.Cells[i] != b.Cells[i] {
			return false
		}
	}
	return true
}

func TestASCIIFastPathMatchesPutChar(t *testing.T) {
	for _, data := range fastPathStreams {
		for _, chunk := range []int{1, 3, 1 << 10} {
			if !sameScreen(screenWith(data, chunk, true), screenWith(data, chunk, false)) {
				t.Errorf("fast path differs for %q in chunks of %d", data, chunk)
			}
		}
	}
}

func TestASCIIFastPathMatchesPutCharRandom(t *testing.T) {
	pieces := []string{"ab", "xyz ", "世", "é", "\r", "\n", "\x1b[2C", "\x1b[1;9H", "\x1b(0q\x1b(B", "\x1b[7m", "\x1b[m", "\t", "─"}
	rng := rand.New(rand.NewPCG(1, 2))
	for n := 0; n < 300; n++ {
		var data string
		for i := 0; i < 40; i++ {
			data += pieces[rng.IntN(len(pieces))]
		}
		if !sameScreen(screenWith(data, 1<<10, true), screenWith(data, 1<<10, false)) {
			t.Fatalf("fast path differs for %q", data)
		}
	}
}
