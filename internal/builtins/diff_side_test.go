// Behaviour tests ported from uutils diffutils (MIT/Apache-2.0).
// Source: src/side_diff.rs (https://github.com/uutils/diffutils).
package builtins

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDiffFormatTabsAndSpaces(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to int
		expanded bool
		want     string
	}{
		{"expanded_false", 0, 5, false, "\t "},
		{"expanded_true", 0, 5, true, "     "},
		{"from_greater_than_to", 6, 5, false, ""},
		{"from_non_zero_position", 2, 7, false, "\t   "},
		{"multiple_full_tabs_needed", 0, 12, false, "\t\t\t"},
		{"uneven_tab_boundary_with_spaces", 3, 10, false, "\t\t  "},
		{"expanded_true_with_offset", 3, 9, true, "      "},
		{"exact_tab_boundary_from_midpoint", 4, 8, false, "\t"},
		{"mixed_tabs_and_spaces_edge_case", 5, 9, false, "\t "},
		{"minimal_gap_with_tab", 7, 8, false, "\t"},
		{"expanded_false_with_tab_at_end", 6, 8, false, "\t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := diffSideConfig{halfWidth: 60, tabSize: 4, expanded: tc.expanded}
			var buf strings.Builder
			diffFormatTabsAndSpaces(tc.from, tc.to, cfg, &buf)
			if buf.String() != tc.want {
				t.Fatalf("%q != %q", buf.String(), tc.want)
			}
		})
	}
}

func TestDiffProcessHalfLine(t *testing.T) {
	utf32le := []byte{0x63, 0x00, 0x00, 0x00, 0x61, 0x00, 0x00, 0x00, 0x66, 0x00, 0x00, 0x00, 0xE9, 0x00, 0x00, 0x00}
	for _, tc := range []struct {
		name                  string
		s, want               string
		maxWidth              int
		isRight, wsGutter     bool
		expanded, invalidUTF8 bool
	}{
		{"empty_line_left_expanded_false", "", "\t\t   ", 10, false, false, false, false},
		{"tabs_unexpanded", "\tabc", "\tabc\t ", 8, false, false, false, false},
		{"utf8_multibyte", "\U0001F609\U0001F609\U0001F609", "\U0001F609\t", 3, false, false, false, false},
		{"newline_handling", "abc\ndef", "abc\t  ", 5, false, false, false, false},
		{"carriage_return", "\rxyz", "\r" + strings.Repeat("\t", 15) + "xyz", 5, true, false, false, false},
		{"exact_width_fit", "abcd", "abcd ", 4, false, false, true, false},
		{"non_utf8_bytes", string([]byte{0x63, 0x61, 0x66, 0xE9}), string([]byte{0x63, 0x61, 0x66, 0xE9, ' ', ' '}), 5, false, false, false, true},
		{"non_utf8_bytes_ignore_padding_bytes", string(utf32le), string(utf32le) + "   ", 6, false, false, false, false},
		{"non_utf8_non_preserve_ascii_bytes_cut", string([]byte{0x63, 0x61, 0x66, 0xA8, 0x80}), string([]byte{0x63, 0x61, 0x66, 0xA8, ' '}), 4, false, false, false, false},
		{"right_line_padding", "xyz", "xyz", 5, true, true, false, false},
		{"mixed_tabs_spaces", "\t  \t", "\t  \t   ", 10, false, false, false, false},
		{"overflow_multibyte", "\u65e5\u672c\u8a9e", "\u65e5\u672c  ", 5, false, false, false, false},
		{"white_space_gutter", "abc", "abc\t  ", 3, false, true, false, false},
		{"expanded_true", "abc", "abc        ", 10, false, false, true, false},
		{"expanded_true_with_gutter", "abc", "abc          ", 10, false, true, true, false},
		{"width0_chars", "abc\x00\v\a\f", "abc\x00\v\a\f\t ", 4, false, false, false, false},
		{"left_empty_white_space_gutter", "", "\t\t\t", 9, false, true, false, false},
		{"s_size_eq_max_width_p1", "abcdefghij", "abcdefghij ", 10, false, false, false, false},
		{"mixed_tabs_and_spaces_inversion", " \t \t ", " \t \t   ", 10, false, false, false, false},
		{"expanded_with_tabs", " \t \t ", "           ", 10, false, false, true, false},
		{"expanded_with_tabs_and_space_gutter", " \t \t ", "             ", 10, false, true, true, false},
		{"zero_width_unicode_chars", "\u200b", "\u200b\t\t   ", 10, false, false, false, false},
		{"multiple_carriage_returns", "\r\r", "\r" + strings.Repeat("\t", 15) + "\r" + strings.Repeat("\t", 15) + "\t\t   ", 10, false, false, false, false},
		{"multiple_carriage_returns_is_right_true", "\r\r", "\r" + strings.Repeat("\t", 15) + "\r" + strings.Repeat("\t", 15), 10, true, false, false, false},
		{"mixed_invalid_utf8_with_valid", "abc\xFF\xFEdef", "abc\xFF\xFEdef   ", 10, false, false, false, true},
		{"max_width_zero", "foo bar", " ", 0, false, false, false, false},
		{"line_only_with_tabs", "\t\t\t", "\t\t   ", 10, false, false, false, false},
		{"tabs_expanded", "\t\t\t", strings.Repeat(" ", 13), 12, false, false, true, false},
		{"mixed_tabs", "a\tb\tc\t", "a\tb\tc  ", 10, false, false, false, false},
		{"mixed_tabs_with_gutter", "a\tb\tc\t", "a\tb\tc\t ", 10, false, true, false, false},
		{"mixed_tabs_expanded", "a\tb\tc\t", "a   b   c  ", 10, false, false, true, false},
		{"mixed_tabs_expanded_with_gutter", "a\tb\tc\t", "a   b   c    ", 10, false, true, true, false},
		{"break_if_invalid_max_width", "a\tb\tc\t", "", 61, false, true, true, false},
		{"new_line", "abc", "abc\t\t   ", 10, false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := diffSideConfig{halfWidth: 30, columnTwoOffset: 60, tabSize: 4, separatorPos: 15, expanded: tc.expanded}
			var buf strings.Builder
			diffProcessHalfLine(tc.s, tc.maxWidth, tc.isRight, tc.wsGutter, cfg, &buf)
			if buf.String() != tc.want {
				t.Fatalf("%q != %q", buf.String(), tc.want)
			}
			if tc.invalidUTF8 && utf8.ValidString(buf.String()) {
				t.Fatal("expected invalid utf8 output")
			}
		})
	}
}

func TestDiffPushOutput(t *testing.T) {
	for _, tc := range []struct {
		name        string
		left, right string
		symbol      byte
		expanded    bool
		want        string
	}{
		{"left_empty_right_not_added", "", "bar", '>', false, "\t\t\t\t\t\t\t      >\tbar\n"},
		{"right_empty_left_not_del", "bar", "", '>', false, "bar\t\t\t\t\t\t\t      >\n"},
		{"both_empty", "", "", ' ', false, "\n"},
		{"output_cut_with_maximization", strings.Repeat("a", 62), strings.Repeat("a", 62), ' ', false, strings.Repeat("a", 61) + "\t" + strings.Repeat("a", 61) + "\n"},
		{"both_lines_non_empty_with_space_symbol_max_tabs", "left", "right", ' ', false, "left\t\t\t\t\t\t\t\tright\n"},
		{"non_space_symbol_with_padding", "data", "", '<', false, "data\t\t\t\t\t\t\t      <\n"},
		{"lines_exceeding_half_width", strings.Repeat("a", 100), strings.Repeat("b", 100), ' ', false, strings.Repeat("a", 61) + "\t" + strings.Repeat("b", 61) + "\n"},
		{"tabs_in_lines_expanded", "\tleft", "\tright", ' ', true, "        left" + strings.Repeat(" ", 52) + "        right" + "\n"},
		{"unicode_characters", "\u00e1\u00e9\u00ed\u00f3\u00fa", "\U0001F600\U0001F603\U0001F604", ' ', false, "\u00e1\u00e9\u00ed\u00f3\u00fa\t\t\t\t\t\t\t\t\U0001F600\U0001F603\U0001F604\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := diffNewSideConfig(130, 8, tc.expanded)
			var buf strings.Builder
			diffPushOutput(tc.left, tc.right, tc.symbol, cfg, &buf)
			if buf.String() != tc.want {
				t.Fatalf("%q != %q", buf.String(), tc.want)
			}
		})
	}
}

// Expected counts match GNU diff 3.12 -y (changed lines pair onto one "|"
// row; a final line without a newline produces no trailing newline).
func TestDiffSideBySide(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		from, to                  string
		opts                      diffOptions
		wantLines, wantLt, wantGt int
		contains                  map[string]int
		checkLtCol                bool
		ltCol                     int
	}{
		{"equal_lines", "equal", "equal", diffOptions{width: 130, tabsize: 8}, 0, 0, 0, map[string]int{"equal": 2}, false, 0},
		{"gutter_marker_column_wide_gutter", "aa\n", "", diffOptions{width: 40, tabsize: 8, expandTabs: true}, 1, 1, 0, map[string]int{"aa": 1}, true, 19},
		{"different_lines", "eq", "ne", diffOptions{width: 130, tabsize: 8}, 0, 0, 0, map[string]int{"eq": 1, "ne": 1}, false, 0},
		{"added_line", "", "new line", diffOptions{width: 130, tabsize: 8}, 0, 0, 1, map[string]int{"new line": 1}, false, 0},
		{"removed_line", "old line", "", diffOptions{width: 130, tabsize: 8}, 0, 1, 0, map[string]int{"old line": 1}, false, 0},
		{"multiple_changes", "line1\nline2\nline3", "line1\nmodified\nline4", diffOptions{width: 130, tabsize: 8}, 2, 0, 0, nil, false, 0},
		{"unicode_and_special_chars", "\u00e1\t\u20ac", "\u20ac\t\U0001F60A", diffOptions{width: 130, tabsize: 8}, 0, 0, 0, map[string]int{"\u00e1": 1, "\u20ac": 2, "\U0001F60A": 1}, false, 0},
		{"mixed_whitespace", "  \tspaces", "\t\t tabs", diffOptions{width: 130, tabsize: 8}, 0, 0, 0, map[string]int{"spaces": 1, "tabs": 1}, false, 0},
		{"empty_files", "", "", diffOptions{width: 130, tabsize: 8}, 0, 0, 0, nil, false, 0},
		{"partially_matching_lines", "match\nchange", "match\nupdated", diffOptions{width: 130, tabsize: 8}, 1, 0, 0, map[string]int{"match": 2}, false, 0},
		{"interleaved_add_remove", "A\nB\nC\nD", "B\nX\nD\nY", diffOptions{width: 130, tabsize: 8}, 4, 1, 1, map[string]int{"A": 1, "X": 1, "Y": 1}, false, 0},
		{"swapped_lines", "1\n2\n3\n4", "4\n3\n2\n1", diffOptions{width: 130, tabsize: 8}, 4, 1, 1, nil, false, 0},
		{"gap_between_changes", "Start\nKeep1\nRemove\nKeep2\nEnd", "Start\nNew1\nKeep1\nKeep2\nNew2\nEnd", diffOptions{width: 130, tabsize: 8}, 6, 1, 2, map[string]int{"Remove": 1, "New1": 1, "New2": 1}, false, 0},
		{"mixed_operations_complex", "Same\nOld1\nSameMid\nOld2\nSameEnd", "Same\nNew1\nSameMid\nNew2\nNew3\nSameEnd", diffOptions{width: 130, tabsize: 8}, 5, 0, 1, nil, false, 0},
		{"insert_remove_middle", "Header\nContent1\nFooter", "Header\nContent2\nFooter", diffOptions{width: 130, tabsize: 8}, 2, 0, 0, map[string]int{"Content1": 1, "Content2": 1}, false, 0},
		{"multiple_adjacent_changes", "A\nB\nC\nD\nE", "A\nX\nY\nD\nZ", diffOptions{width: 130, tabsize: 8}, 4, 0, 0, nil, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := diffLines([]byte(tc.from)), diffLines([]byte(tc.to))
			out := diffSide(a, b, diffMyers(a, b, tc.opts), tc.opts)
			if n := strings.Count(out, "\n"); n != tc.wantLines {
				t.Fatalf("lines %d != %d\n%s", n, tc.wantLines, out)
			}
			if n := strings.Count(out, "<"); n != tc.wantLt {
				t.Fatalf("< count %d != %d\n%s", n, tc.wantLt, out)
			}
			if n := strings.Count(out, ">"); n != tc.wantGt {
				t.Fatalf("> count %d != %d\n%s", n, tc.wantGt, out)
			}
			for s, want := range tc.contains {
				if n := strings.Count(out, s); n != want {
					t.Fatalf("count of %q = %d, want %d\n%s", s, n, want, out)
				}
			}
			if tc.checkLtCol {
				line := strings.SplitN(out, "\n", 2)[0]
				if col := strings.Index(line, "<"); col != tc.ltCol {
					t.Fatalf("gutter marker at column %d, want %d: %q", col, tc.ltCol, line)
				}
			}
		})
	}
}

func TestDiffSideConfig(t *testing.T) {
	for _, tc := range []struct {
		name           string
		width, tabsize int
		expanded       bool
		half, c2o, sep int
	}{
		{"full_width_80_tab_4", 80, 4, false, 37, 40, 38},
		{"full_width_40_tab_8", 40, 8, true, 16, 24, 19},
		{"full_width_30_tab_2", 30, 2, false, 13, 16, 14},
		{"small_width_10_tab_4", 10, 4, false, 2, 8, 4},
		{"minimal_width_3_tab_4", 3, 4, false, 0, 3, 1},
		{"odd_width_7_tab_3", 7, 3, false, 1, 6, 3},
		{"tab_size_larger_than_width", 5, 10, false, 0, 5, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := diffNewSideConfig(tc.width, tc.tabsize, tc.expanded)
			if c.halfWidth != tc.half || c.columnTwoOffset != tc.c2o || c.separatorPos != tc.sep {
				t.Fatalf("%+v", c)
			}
		})
	}
}
