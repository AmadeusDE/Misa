// Package awk implements the core of the AWK language: a lexer, parser and
// interpreter for pattern-action programs with fields and records,
// POSIX numeric-string coercion, printf formatting and the string and math
// builtins that need no arrays. Regular expressions are POSIX EREs from
// internal/posixre.
//
// Arrays, user-defined functions, getline, output redirection, pipes and
// system calls are not implemented yet; programs using them are rejected
// with an error that names the feature.
//
// Text is handled as UTF-8: length, substr, index, match and field
// splitting with an empty FS count characters, and every invalid byte
// counts as one character that stands for itself. Config.Bytes switches
// those functions to bytes.
package awk

import (
	"math"
	"strconv"
	"strings"
)

type kind uint8

const (
	kUninit kind = iota
	kNum
	kStr
	kStrnum
)

// Value is an AWK scalar. Uninitialised values are both "" and 0; strnum
// values are input-derived strings that look like numbers and so compare
// numerically.
type Value struct {
	k kind
	n float64
	s string
}

func num(n float64) Value { return Value{k: kNum, n: n} }
func str(s string) Value  { return Value{k: kStr, s: s} }

func boolVal(b bool) Value {
	if b {
		return num(1)
	}
	return num(0)
}

// input builds the value of a string that came from input: a strnum when
// the whole string looks like a number.
func input(s string) Value {
	if n, ok := looksNumeric(s); ok {
		return Value{k: kStrnum, n: n, s: s}
	}
	return Value{k: kStr, s: s}
}

func (v Value) isNumeric() bool { return v.k != kStr }

func (v Value) float() float64 {
	switch v.k {
	case kStr:
		return strToNum(v.s)
	case kUninit:
		return 0
	}
	return v.n
}

func (v Value) truth() bool {
	switch v.k {
	case kStr:
		return v.s != ""
	case kUninit:
		return false
	}
	return v.n != 0
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

// scanNumber returns the length of the decimal number at the start of s
// (no leading blanks), or 0.
func scanNumber(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	return i
}

func parseDecimal(s string) float64 {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil && !math.IsInf(n, 0) {
		return 0
	}
	return n
}

// strToNum converts the longest numeric prefix of s, after leading blanks.
func strToNum(s string) float64 {
	i := 0
	for i < len(s) && (isBlank(s[i]) || s[i] == '\r' || s[i] == '\f' || s[i] == '\v') {
		i++
	}
	end := scanNumber(s[i:])
	if end == 0 {
		return 0
	}
	return parseDecimal(s[i : i+end])
}

// looksNumeric reports whether all of s, ignoring surrounding blanks, is a
// decimal number.
func looksNumeric(s string) (float64, bool) {
	i, j := 0, len(s)
	for i < j && isBlank(s[i]) {
		i++
	}
	for j > i && isBlank(s[j-1]) {
		j--
	}
	t := s[i:j]
	if end := scanNumber(t); end == 0 || end != len(t) {
		return 0, false
	}
	return parseDecimal(t), true
}

// numToString renders n: integral values as integers, others with format.
func numToString(n float64, format string) string {
	if n == math.Trunc(n) && math.Abs(n) < 1<<63 {
		return strconv.FormatInt(int64(n), 10)
	}
	if math.IsNaN(n) {
		if math.Signbit(n) {
			return "-nan"
		}
		return "nan"
	}
	if math.IsInf(n, 0) {
		if n < 0 {
			return "-inf"
		}
		return "inf"
	}
	return sprintf(format, []Value{num(n)}, false, "%.6g")
}

func compareValues(a, b Value, convfmt string) int {
	if a.isNumeric() && b.isNumeric() {
		x, y := a.float(), b.float()
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	return strings.Compare(a.text(convfmt), b.text(convfmt))
}

func (v Value) text(convfmt string) string {
	if v.k == kNum {
		return numToString(v.n, convfmt)
	}
	return v.s
}
