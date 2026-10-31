// Tests for dd's character-set conversion tables (dd_tables.go), ported from
// uutils coreutils src/uu/dd/src/conversion_tables.rs (MIT) at the commit
// pinned in docs/CREDITS.md.
package builtins

import "testing"

// TestDDTablesContents checks the plain conversion tables entry by entry
// against what the pinned uutils tables actually contain: each of the 256 byte
// values should appear exactly once, so the conversion loses no byte.
//
// Exception: the pinned uutils ASCII_TO_IBM table is *not* a bijection. It maps
// '[' (0x5b) to 0xad and ']' (0x5d) to 0xbd, so 0xad and 0xbd each occur twice,
// and 0x4a/0x6a never occur at all. That is upstream's data (IBM code page 037
// warts included), so it is asserted here rather than "fixed" -- see the port
// notes in the task report.
func TestDDTablesContents(t *testing.T) {
	tables := []struct {
		name string
		tab  *[256]byte
		// wantCount overrides the expected occurrence count per byte value;
		// values absent from the map must occur exactly once.
		wantCount map[byte]int
	}{
		{
			name: "ddASCIIToEBCDIC",
			tab:  &ddASCIIToEBCDIC,
		},
		{
			name: "ddASCIIToIBM",
			tab:  &ddASCIIToIBM,
			wantCount: map[byte]int{
				0xad: 2, // from 0x5b '[' and from 0xd5
				0xbd: 2, // from 0x5d ']' and from 0xd6
				0x4a: 0, // never produced by this table
				0x6a: 0, // never produced by this table
			},
		},
		{
			name: "ddEBCDICToASCII",
			tab:  &ddEBCDICToASCII,
		},
	}
	for _, tc := range tables {
		t.Run(tc.name, func(t *testing.T) {
			var got [256]int
			for _, v := range tc.tab {
				got[v]++
			}
			for v := 0; v < 256; v++ {
				want := 1
				if n, ok := tc.wantCount[byte(v)]; ok {
					want = n
				}
				if got[v] != want {
					t.Errorf("%s: byte %#02x appears %d times, want %d", tc.name, v, got[v], want)
				}
			}
			if len(tc.wantCount) == 0 {
				for i, v := range tc.tab {
					if first := indexOfByte(tc.tab, v, i); first >= 0 {
						t.Errorf("%s: byte %#02x at index %d duplicates index %d", tc.name, v, i, first)
					}
				}
			}
		})
	}
}

// indexOfByte finds an earlier index holding v in tab, so a duplicate report
// names two distinct positions.
func indexOfByte(tab *[256]byte, v byte, before int) int {
	for i := 0; i < before; i++ {
		if tab[i] == v {
			return i
		}
	}
	return -1
}

// TestDDTablesSpotValues pins values from the uutils dd test suite: space is
// 0x40 and 'A' 0xc1 in EBCDIC, so conv=ascii/conv=ebcdic translate them that
// way.
func TestDDTablesSpotValues(t *testing.T) {
	tests := []struct {
		name string
		got  byte
		want int
	}{
		{"ddEBCDICToASCII[0x40]", ddEBCDICToASCII[0x40], ' '},
		{"ddEBCDICToASCII[0xc1]", ddEBCDICToASCII[0xc1], 'A'},
		{"ddEBCDICToASCII[0xc2]", ddEBCDICToASCII[0xc2], 'B'},
		{"ddEBCDICToASCII[0x81]", ddEBCDICToASCII[0x81], 'a'},
		{"ddASCIIToEBCDIC[' ']", ddASCIIToEBCDIC[' '], 0x40},
		{"ddASCIIToEBCDIC['A']", ddASCIIToEBCDIC['A'], 0xc1},
		{"ddASCIIToEBCDIC['B']", ddASCIIToEBCDIC['B'], 0xc2},
		{"ddASCIIToEBCDIC['a']", ddASCIIToEBCDIC['a'], 0x81},
		{"ddASCIIToIBM[' ']", ddASCIIToIBM[' '], 0x40},
		{"ddASCIIToIBM['A']", ddASCIIToIBM['A'], 0xc1},
	}
	for _, tc := range tests {
		if tc.got != byte(tc.want) {
			t.Errorf("%s = %#02x, want %#02x", tc.name, tc.got, byte(tc.want))
		}
	}
}

// TestDDTablesCombinedCase checks the combined conversion+case tables: the
// case folding is visible both when converting EBCDIC in and when converting
// ASCII out.
func TestDDTablesCombinedCase(t *testing.T) {
	if got := ddEBCDICToASCIILowerToUpper[0xc1]; got != 'A' {
		t.Errorf("ddEBCDICToASCIILowerToUpper[0xc1] = %#02x, want %#02x ('A')", got, 'A')
	}
	// EBCDIC 'a' is 0x81; the lower-to-upper table must map it to 'A' too.
	if got := ddEBCDICToASCIILowerToUpper[0x81]; got != 'A' {
		t.Errorf("ddEBCDICToASCIILowerToUpper[0x81] = %#02x, want %#02x ('A')", got, 'A')
	}
	if got := ddEBCDICToASCIILowerToUpper[0xc2]; got != 'B' {
		t.Errorf("ddEBCDICToASCIILowerToUpper[0xc2] = %#02x, want %#02x ('B')", got, 'B')
	}
	// EBCDIC 'A' (0xc1) and 'a' (0x81) both become 'a'.
	if got := ddEBCDICToASCIIUpperToLower[0xc1]; got != 'a' {
		t.Errorf("ddEBCDICToASCIIUpperToLower[0xc1] = %#02x, want %#02x ('a')", got, 'a')
	}
	if got := ddEBCDICToASCIIUpperToLower[0x81]; got != 'a' {
		t.Errorf("ddEBCDICToASCIIUpperToLower[0x81] = %#02x, want %#02x ('a')", got, 'a')
	}
	if a, b := ddEBCDICToASCIIUpperToLower[0xc1], ddEBCDICToASCIIUpperToLower[0x81]; a != b {
		t.Errorf("ddEBCDICToASCIIUpperToLower[0xc1] (%#02x) != [0x81] (%#02x), want both 'a'", a, b)
	}
	// ASCII 'a' and 'A' both convert to EBCDIC 'A' (0xc1) in the
	// lower-to-upper table.
	if a, b := ddASCIIToEBCDICLowerToUpper['a'], ddASCIIToEBCDICLowerToUpper['A']; a != b {
		t.Errorf("ddASCIIToEBCDICLowerToUpper['a'] (%#02x) != ['A'] (%#02x), want both 0xc1", a, b)
	}
	if got := ddASCIIToEBCDICLowerToUpper['a']; got != 0xc1 {
		t.Errorf("ddASCIIToEBCDICLowerToUpper['a'] = %#02x, want 0xc1", got)
	}
	// ASCII 'a' and 'A' both convert to EBCDIC 'a' (0x81) in the
	// upper-to-lower table.
	if a, b := ddASCIIToEBCDICUpperToLower['a'], ddASCIIToEBCDICUpperToLower['A']; a != b {
		t.Errorf("ddASCIIToEBCDICUpperToLower['a'] (%#02x) != ['A'] (%#02x), want both 0x81", a, b)
	}
	if got := ddASCIIToEBCDICUpperToLower['A']; got != 0x81 {
		t.Errorf("ddASCIIToEBCDICUpperToLower['A'] = %#02x, want 0x81", got)
	}
}

// TestDDTablesIBMVersusEBCDIC checks that conv=ibm really is a different table
// from conv=ebcdic: they differ for a handful of ASCII bytes ('^', '~', ...).
func TestDDTablesIBMVersusEBCDIC(t *testing.T) {
	differs := 0
	for i := range ddASCIIToIBM {
		if ddASCIIToIBM[i] != ddASCIIToEBCDIC[i] {
			differs++
		}
	}
	if differs == 0 {
		t.Error("ddASCIIToIBM and ddASCIIToEBCDIC are identical, want at least one entry to differ")
	}
	if got := ddASCIIToIBM['^']; got == ddASCIIToEBCDIC['^'] {
		t.Errorf("ddASCIIToIBM['^'] = %#02x equals ddASCIIToEBCDIC['^'], want them to differ", got)
	}
	if got := ddASCIIToIBM['~']; got == ddASCIIToEBCDIC['~'] {
		t.Errorf("ddASCIIToIBM['~'] = %#02x equals ddASCIIToEBCDIC['~'], want them to differ", got)
	}
}
