package vi

import (
	"unicode"
	"unicode/utf8"
)

// foldIndex finds sub in s ignoring case and returns the match's byte span
// in s itself. Indexing a lowered copy is wrong: lowering can change a
// character's byte length.
func foldIndex(s, sub string) (start, end int) {
	for i := 0; i <= len(s); {
		if j, ok := foldPrefix(s[i:], sub); ok {
			return i, i + j
		}
		if i == len(s) {
			break
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		i += n
	}
	return -1, -1
}

// foldLastIndex is foldIndex for the last match.
func foldLastIndex(s, sub string) (start, end int) {
	for i := len(s); i >= 0; {
		if j, ok := foldPrefix(s[i:], sub); ok {
			return i, i + j
		}
		if i == 0 {
			break
		}
		_, n := utf8.DecodeLastRuneInString(s[:i])
		i -= n
	}
	return -1, -1
}

// foldPrefix reports whether s starts with sub ignoring case, and how many
// bytes of s the match covers.
func foldPrefix(s, sub string) (int, bool) {
	i := 0
	for _, want := range sub {
		if i >= len(s) {
			return 0, false
		}
		got, n := utf8.DecodeRuneInString(s[i:])
		if !foldEqual(got, want) {
			return 0, false
		}
		i += n
	}
	return i, true
}

func foldEqual(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

// dropLastRune removes the last character, not the last byte.
func dropLastRune(s string) string {
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}
