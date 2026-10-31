package builtins

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func mustNum(t *testing.T, lit string, ibase int) bcNumber {
	t.Helper()
	neg := strings.HasPrefix(lit, "-")
	if neg {
		lit = lit[1:]
	}
	n, err := bcParseNumber(lit, ibase)
	if err != nil {
		t.Fatalf("parse %q: %v", lit, err)
	}
	if neg {
		n = n.negated()
	}
	return n
}

func TestBcParseNumberDecimal(t *testing.T) {
	tests := []struct {
		lit   string
		want  string
		scale int
	}{
		{"0", "0", 0},
		{"7", "7", 0},
		{"123", "123", 0},
		{"1.5", "1.5", 1},
		{".5", ".5", 1},
		{"0.50", ".50", 2},
		{"0000.250", ".250", 3},
	}
	for _, tc := range tests {
		n := mustNum(t, tc.lit, 10)
		if got := n.decimalString(); got != tc.want {
			t.Errorf("parse %q = %q, want %q", tc.lit, got, tc.want)
		}
		if n.scale != tc.scale {
			t.Errorf("parse %q scale = %d, want %d", tc.lit, n.scale, tc.scale)
		}
	}
}

func TestBcParseNumberBases(t *testing.T) {
	tests := []struct {
		lit   string
		ibase int
		want  string
	}{
		{"FF", 16, "255"},
		{"10", 16, "16"},
		{"11", 2, "3"},
		{"A", 2, "10"},  // one character is always its digit value
		{"11A", 2, "7"}, // out-of-range digits clamp to base-1
		{"1.8", 16, "1.5"},
		{".8", 16, ".5"},
		{".A", 16, ".6"}, // 10/16 truncated to one decimal place
		{"1.0", 16, "1.0"},
	}
	for _, tc := range tests {
		n := mustNum(t, tc.lit, tc.ibase)
		if got := n.decimalString(); got != tc.want {
			t.Errorf("parse %q base %d = %q, want %q", tc.lit, tc.ibase, got, tc.want)
		}
	}
}

func TestBcParseNumberTooLarge(t *testing.T) {
	lit := strings.Repeat("9", bcMaxDigits+1)
	if _, err := bcParseNumber(lit, 10); !errors.Is(err, bcErrTooLarge) {
		t.Fatalf("err = %v, want %v", err, bcErrTooLarge)
	}
}

func TestBcArithmetic(t *testing.T) {
	// The scale rules are POSIX/Toybox:
	//   + and - : max(sa, sb)
	//   *       : min(sa+sb, max(setting, sa, sb))
	//   /       : setting
	//   %       : max(setting+sb, sa)
	//   ^       : min(sa*e, max(setting, sa)) for e > 0, setting for e < 0
	//   sqrt    : max(setting, sa)
	tests := []struct {
		a, op, b string
		setting  int
		want     string
	}{
		{"1.5", "+", "2.5", 3, "4.0"},
		{"1.50", "+", "2.5", 3, "4.00"},
		{"0.1", "+", "0.2", 0, ".3"},
		{"1.5", "-", "2.50", 0, "-1.00"},
		{"1.5", "*", "1.5", 3, "2.25"},
		{"1.234", "*", "1.234", 2, "1.522"},
		{"1.234", "*", "1.234", 8, "1.522756"},
		{"1.234", "*", "1.234", 0, "1.522"},
		{"1", "/", "3", 5, ".33333"},
		{"10", "/", "3", 3, "3.333"},
		{"-10", "/", "3", 3, "-3.333"},
		{"7", "%", "3", 0, "1"},
		{"7.5", "%", "2", 0, "1.5"},
		{"-7", "%", "3", 0, "-1"},
		{"7.5", "%", "2", 2, "0"},
		{"2", "^", "10", 0, "1024"},
		{"2", "^", "0", 0, "1"},
		{"0.5", "^", "3", 0, ".1"},
		{"1.5", "^", "2", 5, "2.25"},
		{"2", "^", "-2", 4, ".2500"},
	}
	for _, tc := range tests {
		a := mustNum(t, tc.a, 10)
		b := mustNum(t, tc.b, 10)
		var got bcNumber
		var err error
		switch tc.op {
		case "+":
			got, err = bcAdd(a, b)
		case "-":
			got, err = bcSub(a, b)
		case "*":
			got, err = bcMul(a, b, tc.setting)
		case "/":
			got, err = bcDiv(a, b, tc.setting)
		case "%":
			got, err = bcMod(a, b, tc.setting)
		case "^":
			got, err = bcPow(a, b, tc.setting)
		}
		if err != nil {
			t.Errorf("%s %s %s (scale %d): %v", tc.a, tc.op, tc.b, tc.setting, err)
			continue
		}
		if got.decimalString() != tc.want {
			t.Errorf("%s %s %s (scale %d) = %s, want %s", tc.a, tc.op, tc.b, tc.setting, got.decimalString(), tc.want)
		}
	}
}

func TestBcSqrt(t *testing.T) {
	tests := []struct {
		a       string
		setting int
		want    string
	}{
		{"0", 3, "0"},
		{"1", 3, "1.000"},
		{"2", 0, "1"},
		{"2", 5, "1.41421"},
		{"16", 2, "4.00"},
		{"1.44", 0, "1.20"},
		{"0.09", 0, ".30"},
		{"0.0001", 0, ".0100"},
		{"123456789", 10, "11111.1110605555"},
	}
	for _, tc := range tests {
		a := mustNum(t, tc.a, 10)
		got, err := bcSqrt(a, tc.setting)
		if err != nil {
			t.Fatalf("sqrt(%s): %v", tc.a, err)
		}
		if got.decimalString() != tc.want {
			t.Errorf("sqrt(%s) scale %d = %s, want %s", tc.a, tc.setting, got.decimalString(), tc.want)
		}
	}
}

func TestBcArithmeticErrors(t *testing.T) {
	one := mustNum(t, "1", 10)
	zero := mustNum(t, "0", 10)
	if _, err := bcDiv(one, zero, 2); !errors.Is(err, bcErrDivideByZero) {
		t.Errorf("1/0 err = %v", err)
	}
	if _, err := bcMod(one, zero, 2); !errors.Is(err, bcErrDivideByZero) {
		t.Errorf("1%%0 err = %v", err)
	}
	half := mustNum(t, "1.5", 10)
	if _, err := bcPow(one, half, 2); !errors.Is(err, bcErrNonInteger) {
		t.Errorf("1^1.5 err = %v", err)
	}
	neg := mustNum(t, "-1", 10)
	if _, err := bcSqrt(neg, 2); !errors.Is(err, bcErrNegative) {
		t.Errorf("sqrt(-1) err = %v", err)
	}
	if _, err := bcPow(one, mustNum(t, "10000001", 10), 0); !errors.Is(err, bcErrTooLarge) {
		t.Errorf("huge exponent err = %v", err)
	}
	if _, err := bcCheckedRescale(bcNumInt(1), bcMaxScale+1); !errors.Is(err, bcErrOverflow) {
		t.Errorf("huge rescale err = %v", err)
	}
}

func TestBcCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1", "1", 0},
		{"1.0", "1", 0},
		{"0", "-0.0", 0},
		{"1.5", "1.50", 0},
		{"1.5", "1.51", -1},
		{"-1.5", "-1.51", 1},
		{"2", "10", -1},
		{".5", "1", -1},
	}
	for _, tc := range tests {
		got := bcCompare(mustNum(t, tc.a, 10), mustNum(t, tc.b, 10))
		if got != tc.want {
			t.Errorf("compare(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestBcLengthAndScale(t *testing.T) {
	tests := []struct {
		lit      string
		length   int64
		numScale int64
	}{
		{"0", 0, 0},
		{"123", 3, 0},
		{"1.5", 2, 1},
		{"1.50", 3, 2},
		{".05", 1, 2},
		{".50", 2, 2},
		{"100.00", 5, 2},
	}
	for _, tc := range tests {
		n := mustNum(t, tc.lit, 10)
		if got := n.bcLength().unscaled.Int64(); got != tc.length {
			t.Errorf("length(%s) = %d, want %d", tc.lit, got, tc.length)
		}
		if got := n.bcScaleOf().unscaled.Int64(); got != tc.numScale {
			t.Errorf("scale(%s) = %d, want %d", tc.lit, got, tc.numScale)
		}
	}
}

func TestBcPrintNumberBase10(t *testing.T) {
	tests := []struct {
		lit  string
		want string
	}{
		{"0", "0"},
		{"0.000", "0"},
		{"123", "123"},
		{"1.5", "1.5"},
		{".05", ".05"},
		{"-1.50", "-1.50"},
	}
	for _, tc := range tests {
		var buf bytes.Buffer
		out := newBcOutput(&buf, 69)
		out.printNumber(mustNum(t, tc.lit, 10), 10)
		if got := buf.String(); got != tc.want {
			t.Errorf("print %s = %q, want %q", tc.lit, got, tc.want)
		}
	}
}

func TestBcPrintNumberOtherBases(t *testing.T) {
	tests := []struct {
		lit   string
		obase int
		want  string
	}{
		{"0", 2, "0"},
		{"10", 2, "1010"},
		{"255", 16, "FF"},
		{"-255", 16, "-FF"},
		{"255.5", 16, "FF.8"},
		{".5", 16, ".8"},
		{".1", 2, ".0001"},
		{"12345", 100, " 01 23 45"},
		{"100", 100, " 01 00"},
		{".5", 100, ".50"},
		{"16", 16, "10"},
	}
	for _, tc := range tests {
		var buf bytes.Buffer
		out := newBcOutput(&buf, 1000)
		out.printNumber(mustNum(t, tc.lit, 10), tc.obase)
		if got := buf.String(); got != tc.want {
			t.Errorf("print %s base %d = %q, want %q", tc.lit, tc.obase, got, tc.want)
		}
	}
}

func TestBcOutputWrapsLongNumbers(t *testing.T) {
	var buf bytes.Buffer
	out := newBcOutput(&buf, 10)
	out.printNumber(mustNum(t, "123456789012345", 10), 10)
	got := buf.String()
	if got != "123456789\\\n012345" {
		t.Fatalf("got %q", got)
	}
}
