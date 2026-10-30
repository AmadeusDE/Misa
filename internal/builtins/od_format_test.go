// Test cases adapted from uutils coreutils od (MIT), files
// src/uu/od/src/prn_int.rs, prn_char.rs, prn_float.rs and output_info.rs
// (commit fee61ff5d4f87e0f912a52e639942b5f7ad58663).
package builtins

import (
	"encoding/binary"
	"math"
	"testing"
)

func odTestField(t *testing.T, kind odFormatKind, size int, buf []byte) string {
	t.Helper()
	f, ok := odNewFormat(kind, size)
	if !ok {
		t.Fatalf("odNewFormat(%v, %d) failed", kind, size)
	}
	return f.odField(buf, 0, binary.LittleEndian)
}

// odLe returns the little-endian encoding of the low size bytes of v.
func odLe(v uint64, size int) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	return b[:size]
}

func TestOdIntFields(t *testing.T) {
	cases := []struct {
		kind odFormatKind
		size int
		val  uint64
		want string
	}{
		{odFmtOct, 1, 0, " 000"},
		{odFmtOct, 1, 0xff, " 377"},
		{odFmtOct, 2, 0, " 000000"},
		{odFmtOct, 2, 0xffff, " 177777"},
		{odFmtOct, 4, 0, " 00000000000"},
		{odFmtOct, 4, 0xffffffff, " 37777777777"},
		{odFmtOct, 8, 0, " 0000000000000000000000"},
		{odFmtOct, 8, 0xffffffffffffffff, " 1777777777777777777777"},
		{odFmtHex, 1, 0, " 00"},
		{odFmtHex, 1, 0xff, " ff"},
		{odFmtHex, 2, 0xffff, " ffff"},
		{odFmtHex, 4, 0xffffffff, " ffffffff"},
		{odFmtHex, 8, 0xffffffffffffffff, " ffffffffffffffff"},
		{odFmtDecU, 1, 0, "   0"},
		{odFmtDecU, 1, 0xff, " 255"},
		{odFmtDecU, 2, 0, "     0"},
		{odFmtDecU, 2, 0xffff, " 65535"},
		{odFmtDecU, 4, 0, "          0"},
		{odFmtDecU, 4, 0xffffffff, " 4294967295"},
		{odFmtDecU, 8, 0, "                    0"},
		{odFmtDecU, 8, 0xffffffffffffffff, " 18446744073709551615"},
		{odFmtDecS, 1, 0, "    0"},
		{odFmtDecS, 1, 0x7f, "  127"},
		{odFmtDecS, 1, 0x80, " -128"},
		{odFmtDecS, 2, 0x8000, " -32768"},
		{odFmtDecS, 4, 0x80000000, " -2147483648"},
		{odFmtDecS, 8, 0x8000000000000000, " -9223372036854775808"},
	}
	for _, tc := range cases {
		got := odTestField(t, tc.kind, tc.size, odLe(tc.val, tc.size))
		if got != tc.want {
			t.Errorf("field(%v, %d, %#x) = %q, want %q", tc.kind, tc.size, tc.val, got, tc.want)
		}
	}
}

func TestOdCharAndAsciiFields(t *testing.T) {
	cases := []struct {
		kind odFormatKind
		in   byte
		want string
	}{
		{odFmtAscii, 0x00, " nul"},
		{odFmtAscii, 0x01, " soh"},
		{odFmtAscii, 0x20, "  sp"},
		{odFmtAscii, 0x41, "   A"},
		{odFmtAscii, 0x7e, "   ~"},
		{odFmtAscii, 0x7f, " del"},
		{odFmtAscii, 0x80, " nul"},
		{odFmtAscii, 0xc1, "   A"},
		{odFmtAscii, 0xff, " del"},
		{odFmtChar, 0x00, "  \\0"},
		{odFmtChar, 0x01, " 001"},
		{odFmtChar, 0x07, "  \\a"},
		{odFmtChar, 0x0a, "  \\n"},
		{odFmtChar, 0x20, "    "},
		{odFmtChar, 0x41, "   A"},
		{odFmtChar, 0x7e, "   ~"},
		{odFmtChar, 0x7f, " 177"},
		{odFmtChar, 0x80, " 200"},
		{odFmtChar, 0x9f, " 237"},
		{odFmtChar, 0xc3, " 303"},
		{odFmtChar, 0xff, " 377"},
	}
	for _, tc := range cases {
		got := odTestField(t, tc.kind, 1, []byte{tc.in})
		if got != tc.want {
			t.Errorf("field(%v, %#x) = %q, want %q", tc.kind, tc.in, got, tc.want)
		}
	}
}

func TestOdFloatFields(t *testing.T) {
	f32 := func(v float32) []byte { return odLe(uint64(math.Float32bits(v)), 4) }
	f64 := func(v float64) []byte { return odLe(math.Float64bits(v), 8) }
	cases := []struct {
		kind odFormatKind
		size int
		buf  []byte
		want string
	}{
		// The want strings are uutils' format_f32/format_f64 output, which od
		// prints with one more space in front.
		{odFmtFloat, 4, f32(1), "              1"},
		{odFmtFloat, 4, f32(10), "             10"},
		{odFmtFloat, 4, f32(1e10), "    10000000000"},
		{odFmtFloat, 4, f32(0.1), "            0.1"},
		{odFmtFloat, 4, f32(1e-4), "         0.0001"},
		{odFmtFloat, 4, f32(1e-5), "           1e-5"},
		{odFmtFloat, 4, f32(-1), "             -1"},
		{odFmtFloat, 4, f32(-1.2345679), "     -1.2345679"},
		{odFmtFloat, 4, f32(3.4028235e38), "  3.4028235e+38"},
		{odFmtFloat, 4, f32(0), "              0"},
		{odFmtFloat, 4, f32(float32(math.Copysign(0, -1))), "             -0"},
		{odFmtFloat, 4, f32(float32(math.NaN())), "            NaN"},
		{odFmtFloat, 4, f32(float32(math.Inf(1))), "            inf"},
		{odFmtFloat, 4, f32(float32(math.Inf(-1))), "           -inf"},
		{odFmtDouble, 8, f64(1), "                       1"},
		{odFmtDouble, 8, f64(12345678912345678), "       12345678912345678"},
		{odFmtDouble, 8, f64(1e15), "        1000000000000000"},
		{odFmtDouble, 8, f64(1e16), "                   1e+16"},
		{odFmtDouble, 8, f64(-2), "                      -2"},
		{odFmtDouble, 8, f64(-2.2250738585072014e-308), "-2.2250738585072014e-308"},
		{odFmtDouble, 8, f64(5e-324), "                  5e-324"},
		{odFmtDouble, 8, f64(4e-320), "                  4e-320"},
		{odFmtDouble, 8, f64(math.Copysign(0, -1)), "                      -0"},
		{odFmtDouble, 8, f64(math.Inf(1)), "                     inf"},
		{odFmtDouble, 8, f64(math.NaN()), "                     NaN"},
	}
	for _, tc := range cases {
		got := odTestField(t, tc.kind, tc.size, tc.buf)
		if got != " "+tc.want {
			t.Errorf("field(%v, %x) = %q, want %q", tc.kind, tc.buf, got, " "+tc.want)
		}
	}
}

func TestOdHalfFields(t *testing.T) {
	cases := []struct {
		bits uint16
		kind odFormatKind
		want string
	}{
		{0x3c00, odFmtHalf, "               1"},
		{0x4900, odFmtHalf, "              10"},
		{0x0000, odFmtHalf, "               0"},
		{0x8000, odFmtHalf, "              -0"},
		{0x7c00, odFmtHalf, "             inf"},
		{0xfc00, odFmtHalf, "            -inf"},
		{0x7e00, odFmtHalf, "             NaN"},
		{0x8400, odFmtHalf, "   -6.1035156e-5"},
		{0x0001, odFmtHalf, "    5.9604645e-8"},
		{0x3f80, odFmtBFloat, "               1"},
		{0x7f80, odFmtBFloat, "             inf"},
		{0xb880, odFmtBFloat, "   -6.1035156e-5"},
	}
	for _, tc := range cases {
		got := odTestField(t, tc.kind, 2, odLe(uint64(tc.bits), 2))
		if got != tc.want {
			t.Errorf("field(%v, %#04x) = %q, want %q", tc.kind, tc.bits, got, tc.want)
		}
	}
}

func TestOdTrimFloat(t *testing.T) {
	cases := []struct{ in, want string }{
		{"      1.0000000", "1"},
		{"     10.000000", "10"},
		{"     -65504.000", "-65504"},
		{"    -0.19995117", "-0.19995117"},
		{"  -2.0004272e-2", "-2.0004272e-2"},
		{"             NaN", "NaN"},
		{"            -inf", "-inf"},
		{"               0", "0"},
	}
	for _, tc := range cases {
		if got := odTrimFloat(tc.in); got != tc.want {
			t.Errorf("odTrimFloat(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
