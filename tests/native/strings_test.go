package native

import "testing"

func TestStrings_Basic(t *testing.T) {
	input := "abc\x00\x01\x02hello world\x00\x03"
	r := runStdinBytes(t, "strings", []byte(input), "-n", "4")
	wantSuccess(t, r)
	wantOutContains(t, r, "hello world")
}

func TestStrings_DefaultMin(t *testing.T) {
	input := "\x00\x00abcd\x00\x00"
	r := runStdinBytes(t, "strings", []byte(input))
	wantSuccess(t, r)
	wantOutContains(t, r, "abcd")
}

func TestStrings_ShortStringsFiltered(t *testing.T) {
	input := "\x00ab\x00cdef\x00"
	r := runStdinBytes(t, "strings", []byte(input))
	wantSuccess(t, r)
	wantOut(t, r, "cdef\n")
}

func TestStrings_File(t *testing.T) {
	dir := t.TempDir()
	mkFile(t, dir, "f.bin", "\x00\x00hello\x00\x00world\x00\x00")
	r := runIn(t, dir, "strings", "-n", "4", "f.bin")
	wantSuccess(t, r)
	wantOutContains(t, r, "hello")
	wantOutContains(t, r, "world")
}

func TestStrings_CustomLength(t *testing.T) {
	input := "\x00ab\x00abc\x00abcdef\x00"
	r := runStdinBytes(t, "strings", []byte(input), "-n", "5")
	wantSuccess(t, r)
	wantOut(t, r, "abcdef\n")
}

func TestStrings_OffsetOctal(t *testing.T) {
	input := make([]byte, 0, 20)
	for i := 0; i < 6; i++ {
		input = append(input, 0)
	}
	input = append(input, []byte("foobar\n")...)
	r := runStdinBytes(t, "strings", input, "-o")
	wantSuccess(t, r)
	wantOutContains(t, r, "6 foobar")
}

func TestStrings_OffsetDecimal(t *testing.T) {
	input := make([]byte, 0, 20)
	for i := 0; i < 10; i++ {
		input = append(input, 0)
	}
	input = append(input, []byte("foobar\n")...)
	r := runStdinBytes(t, "strings", input, "-t", "d")
	wantSuccess(t, r)
	wantOutContains(t, r, "10 foobar")
}

func TestStrings_OffsetHex(t *testing.T) {
	input := make([]byte, 0, 20)
	for i := 0; i < 10; i++ {
		input = append(input, 0)
	}
	input = append(input, []byte("foobar\n")...)
	r := runStdinBytes(t, "strings", input, "-t", "x")
	wantSuccess(t, r)
	wantOutContains(t, r, "a foobar")
}

func TestStrings_MultipleWithOffset(t *testing.T) {
	input := []byte("\x00foo\x00b\x00bar\n")
	r := runStdinBytes(t, "strings", input, "-n", "3", "-o")
	wantSuccess(t, r)
	wantOutContains(t, r, "1 foo")
	wantOutContains(t, r, "7 bar")
}
