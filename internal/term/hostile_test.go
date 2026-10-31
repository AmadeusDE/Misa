package term

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// Program output is untrusted: no byte sequence may panic the emulator or make
// it hold memory without bound.

func TestHugeCSINumbersDoNotPanic(t *testing.T) {
	numbers := []string{"9223372036854775808", "18446744073709551616", "99999999999999999999999999", "2147483648"}
	finals := "@ABCDEFGHIJKLMPSTXZ`abdefghlmnqrstu"
	for _, n := range numbers {
		for _, f := range finals {
			for _, seq := range []string{
				"\x1b[" + n + string(f),
				"\x1b[" + n + ";" + n + string(f),
				"\x1b[?" + n + string(f),
				"\x1b[>" + n + string(f),
				"\x1b[1:" + n + string(f),
				"\x1b[5;10r\x1b[" + n + string(f),
			} {
				tm := New(6, 20)
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("%q panicked: %v", seq, r)
						}
					}()
					tm.ProcessOutput([]byte("abc\r\n" + seq + "\x1b[r\x1b[Hok"))
				}()
				if got := tm.PlainScreen()[0]; !strings.HasPrefix(got, "ok") {
					t.Errorf("%q: terminal unusable afterwards, row 0 = %q", seq, got)
				}
			}
		}
	}
}

func TestCSIParameterListsAreBounded(t *testing.T) {
	tm := New(4, 10)
	tm.mu.Lock()
	tm.feedSync([]byte("\x1b[" + strings.Repeat("1;", 1<<20)))
	params := len(tm.params)
	tm.feedSync([]byte("1" + strings.Repeat(":2", 1<<20)))
	subs := len(tm.subParams[len(tm.subParams)-1])
	tm.mu.Unlock()
	if params > maxCSIParams || subs > maxCSISubParams {
		t.Errorf("params = %d, sub-params = %d; want at most %d and %d", params, subs, maxCSIParams, maxCSISubParams)
	}
}

func TestCombiningMarksPerCellAreBounded(t *testing.T) {
	tm := New(2, 10)
	tm.ProcessOutput([]byte("a" + strings.Repeat("́", 10000)))
	tm.mu.Lock()
	marks := marksOf(tm.cells[0][0].marks)
	tm.mu.Unlock()
	if n := utf8.RuneCountInString(marks); n == 0 || n > maxMarksPerCell {
		t.Errorf("cell holds %d marks, want 1..%d", n, maxMarksPerCell)
	}
}

func TestQueuedRepliesAreBounded(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	tm := New(4, 10)
	tm.conn = writeConn{w} // nobody reads r: the pipe fills and the writer blocks
	query := []byte(strings.Repeat("\x1b[6n", 1000))
	for i := 0; i < 2000; i++ {
		tm.ProcessOutput(query)
	}
	tm.input.mu.Lock()
	pending := tm.input.pendingReplies
	tm.input.mu.Unlock()
	if pending > maxPendingReplies {
		t.Errorf("%d reply bytes queued, want at most %d", pending, maxPendingReplies)
	}
}
func TestStyleCacheIsBounded(t *testing.T) {
	tm := New(1, 4)
	for i := range 3 * maxStyles {
		tm.ProcessOutput(fmt.Appendf(nil, "\x1b[38;2;%d;%d;%dmx\r", i&255, i>>8&255, i>>16&255))
		tm.Render()
	}
	n := 0
	styleCache.Range(func(any, any) bool { n++; return true })
	if n > maxStyles+1 {
		t.Errorf("style cache holds %d entries, want at most %d", n, maxStyles)
	}
}
