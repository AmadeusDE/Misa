package wayland

import (
	"bytes"
	"testing"
)

func TestEncodeLayout(t *testing.T) {
	b, fds := encode(3, 2, []any{uint32(7), int32(-1), "ab", FD(9)})
	want := []byte{
		3, 0, 0, 0, 2, 0, 24, 0,
		7, 0, 0, 0,
		0xff, 0xff, 0xff, 0xff,
		3, 0, 0, 0, 'a', 'b', 0, 0,
	}
	if !bytes.Equal(b, want) {
		t.Fatalf("encode = %v, want %v", b, want)
	}
	if len(fds) != 1 || fds[0] != 9 {
		t.Fatalf("fds = %v", fds)
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	b, _ := encode(5, 1, []any{uint32(42), "hello", []byte{1, 2, 3}, Fixed(256 * 3)})
	b = append(b, 0xde, 0xad)
	ev, n, ok := splitMessage(b)
	if !ok || n != len(b)-2 {
		t.Fatalf("split ok=%v n=%d", ok, n)
	}
	if ev.Obj != 5 || ev.Op != 1 {
		t.Fatalf("header = %d/%d", ev.Obj, ev.Op)
	}
	if v := ev.Uint(); v != 42 {
		t.Errorf("uint = %d", v)
	}
	if s := ev.Text(); s != "hello" {
		t.Errorf("string = %q", s)
	}
	if a := ev.Array(); !bytes.Equal(a, []byte{1, 2, 3}) {
		t.Errorf("array = %v", a)
	}
	if f := ev.Fixed().Float(); f != 3 {
		t.Errorf("fixed = %v", f)
	}
	if ev.Err != nil {
		t.Errorf("err = %v", ev.Err)
	}
}

func TestSplitNeedsWholeMessage(t *testing.T) {
	b, _ := encode(1, 0, []any{uint32(1)})
	if _, _, ok := splitMessage(b[:len(b)-1]); ok {
		t.Fatal("split a truncated message")
	}
}

func TestTruncatedArgumentSetsErr(t *testing.T) {
	ev := Event{data: []byte{1, 0}}
	ev.Uint()
	if ev.Err == nil {
		t.Fatal("no error for short data")
	}
}
