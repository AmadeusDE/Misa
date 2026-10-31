package builtins

// Arbitrary-precision decimal arithmetic for bc, value = unscaled / 10^scale.
//
// The rules follow POSIX bc as implemented by Toybox toys/pending/bc.c at
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD); see docs/plans/bc.md for the
// documented differences. No Toybox code was copied: the C implementation uses
// base-100 digits and bytecode, this one uses math/big and an AST.
//
// The decimal representation is exact for every stored digit: there is no
// floating point anywhere, so `0.1 + 0.2` is exactly `.3` and never 0.30000000000000004.

import (
	"errors"
	"math/big"
	"strings"
)

const (
	// bcMaxScale bounds the scale setting, scale(), and fractional digits.
	bcMaxScale = 100000
	// bcMaxDigits bounds the size of a single value in decimal digits.
	bcMaxDigits = 1000000
	// bcMaxExponent bounds the absolute exponent accepted by ^.
	bcMaxExponent = 1000000
	// bcMaxNameLen is the reported BC_NAME_MAX.
	bcMaxNameLen = 128
	// bcMaxVars is the reported number of variables.
	bcMaxVars = 65535
	// bcMinBase and bcMaxIbase bound ibase (Toybox: 2..16).
	bcMinBase  = 2
	bcMaxIbase = 16
	// bcMaxObase bounds obase (Toybox: INT_MAX).
	bcMaxObase = 2147483647
)

var (
	bcErrNegative     = errors.New("negative number")
	bcErrNonInteger   = errors.New("non integer number")
	bcErrOverflow     = errors.New("overflow")
	bcErrDivideByZero = errors.New("divide by zero")
	bcErrTooLarge     = errors.New("number too large")
	errBcInterrupted  = errors.New("interrupted")
)

// bcNumber is an exact decimal value: unscaled / 10^scale.
type bcNumber struct {
	unscaled *big.Int
	scale    int
}

func bcNumInt(v int64) bcNumber {
	return bcNumber{big.NewInt(v), 0}
}

func bcNumFromBigInt(v *big.Int, scale int) bcNumber {
	return bcNumber{new(big.Int).Set(v), scale}
}

func bcNumZero() bcNumber { return bcNumber{new(big.Int), 0} }

// bcNumZeroScale is the zero value a division produces at the given scale.
func bcNumZeroScale(scale int) bcNumber { return bcNumber{new(big.Int), scale} }

func (n bcNumber) isZero() bool { return n.unscaled.Sign() == 0 }
func (n bcNumber) isNeg() bool  { return n.unscaled.Sign() < 0 }

func (n bcNumber) negated() bcNumber {
	return bcNumber{new(big.Int).Neg(n.unscaled), n.scale}
}

func (n bcNumber) abs() bcNumber {
	return bcNumber{new(big.Int).Abs(n.unscaled), n.scale}
}

// bcNumDigitCount reports the number of decimal digits in |n|, 0 for zero.
func bcNumDigitCount(v *big.Int) int {
	if v.Sign() == 0 {
		return 0
	}
	return len(new(big.Int).Abs(v).String())
}

func (n bcNumber) digits() int { return bcNumDigitCount(n.unscaled) }

// bcPow10 returns 10^exp as a new big.Int. Callers bound exp.
func bcPow10(exp int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil)
}

// bcRescale moves a value to exactly the given scale, truncating (toward zero)
// or zero-padding as needed.
func (n bcNumber) rescale(scale int) bcNumber {
	switch {
	case scale == n.scale:
		return n
	case scale > n.scale:
		return bcNumber{new(big.Int).Mul(n.unscaled, bcPow10(scale-n.scale)), scale}
	default:
		return bcNumber{new(big.Int).Quo(n.unscaled, bcPow10(n.scale-scale)), scale}
	}
}

// bcCheckedRescale is rescale with the digit budget applied.
func bcCheckedRescale(n bcNumber, scale int) (bcNumber, error) {
	if scale < 0 || scale > bcMaxScale {
		return bcNumber{}, bcErrOverflow
	}
	if delta := scale - n.scale; delta > 0 && n.digits()+delta > bcMaxDigits {
		return bcNumber{}, bcErrTooLarge
	}
	return n.rescale(scale), nil
}

// bcAlign returns both values at the larger of their scales.
func bcAlign(a, b bcNumber) (bcNumber, bcNumber, error) {
	scale := a.scale
	if b.scale > scale {
		scale = b.scale
	}
	if delta := scale - a.scale; delta > 0 && a.digits()+delta > bcMaxDigits {
		return bcNumber{}, bcNumber{}, bcErrTooLarge
	}
	if delta := scale - b.scale; delta > 0 && b.digits()+delta > bcMaxDigits {
		return bcNumber{}, bcNumber{}, bcErrTooLarge
	}
	return a.rescale(scale), b.rescale(scale), nil
}

func bcAdd(a, b bcNumber) (bcNumber, error) {
	aa, bb, err := bcAlign(a, b)
	if err != nil {
		return bcNumber{}, err
	}
	return bcNumber{new(big.Int).Add(aa.unscaled, bb.unscaled), aa.scale}, nil
}

func bcSub(a, b bcNumber) (bcNumber, error) {
	aa, bb, err := bcAlign(a, b)
	if err != nil {
		return bcNumber{}, err
	}
	return bcNumber{new(big.Int).Sub(aa.unscaled, bb.unscaled), aa.scale}, nil
}

// bcMul multiplies with POSIX scale rules: the result scale is
// min(a.scale+b.scale, max(scale, a.scale, b.scale)).
func bcMul(a, b bcNumber, scale int) (bcNumber, error) {
	target := a.scale + b.scale
	want := scale
	if a.scale > want {
		want = a.scale
	}
	if b.scale > want {
		want = b.scale
	}
	if want < target {
		target = want
	}
	if a.digits()+b.digits() > bcMaxDigits {
		return bcNumber{}, bcErrTooLarge
	}
	prod := bcNumber{new(big.Int).Mul(a.unscaled, b.unscaled), a.scale + b.scale}
	return bcCheckedRescale(prod, target)
}

// bcDiv divides with the POSIX rule that the result always has exactly the
// scale setting as its scale.
func bcDiv(a, b bcNumber, scale int) (bcNumber, error) {
	if b.isZero() {
		return bcNumber{}, bcErrDivideByZero
	}
	if scale < 0 || scale > bcMaxScale {
		return bcNumber{}, bcErrOverflow
	}
	exp := b.scale + scale - a.scale
	num, den := a.unscaled, b.unscaled
	if exp > 0 {
		if exp > bcMaxDigits || a.digits()+exp > bcMaxDigits {
			return bcNumber{}, bcErrTooLarge
		}
		num = new(big.Int).Mul(num, bcPow10(exp))
	} else if exp < 0 {
		if -exp > bcMaxDigits || b.digits()-exp > bcMaxDigits {
			return bcNumber{}, bcErrTooLarge
		}
		den = new(big.Int).Mul(den, bcPow10(-exp))
	}
	return bcNumber{new(big.Int).Quo(num, den), scale}, nil
}

// bcInv is 1/x with POSIX exponent rules for negative powers.
func bcInv(a bcNumber, scale int) (bcNumber, error) {
	return bcDiv(bcNumInt(1), a, scale)
}

// bcPow raises a to the integer power b. The exponent must have scale 0.
func bcPow(a, b bcNumber, scale int) (bcNumber, error) {
	if b.scale != 0 {
		return bcNumber{}, bcErrNonInteger
	}
	if !b.unscaled.IsInt64() {
		return bcNumber{}, bcErrOverflow
	}
	exp := b.unscaled.Int64()
	if exp > bcMaxExponent || exp < -bcMaxExponent {
		return bcNumber{}, bcErrTooLarge
	}
	if exp == 0 {
		return bcNumInt(1), nil
	}
	if a.isZero() {
		if exp < 0 {
			return bcNumber{}, bcErrDivideByZero
		}
		return bcNumZeroScale(scale), nil
	}
	mag := exp
	if mag < 0 {
		mag = -mag
	}
	if int64(a.digits())*mag > bcMaxDigits {
		return bcNumber{}, bcErrTooLarge
	}
	pow := new(big.Int).Exp(new(big.Int).Abs(a.unscaled), big.NewInt(mag), nil)
	result := bcNumber{pow, a.scale * int(mag)}
	if a.isNeg() && mag%2 == 1 {
		result = result.negated()
	}
	if exp < 0 {
		// Invert first, then truncate to the scale setting.
		return bcDiv(bcNumInt(1), result, scale)
	}
	target := result.scale
	want := scale
	if a.scale > want {
		want = a.scale
	}
	if want < target {
		target = want
	}
	return bcCheckedRescale(result, target)
}

// bcSqrt returns the square root truncated to max(scale, a.scale) digits.
func bcSqrt(a bcNumber, scale int) (bcNumber, error) {
	if a.isNeg() {
		return bcNumber{}, bcErrNegative
	}
	if a.isZero() {
		// Toybox checks for zero before the scale is bumped: sqrt(0.00) has
		// the scale setting, not the operand's scale.
		return bcNumZeroScale(scale), nil
	}
	target := scale
	if a.scale > target {
		target = a.scale
	}
	exp := 2*target - a.scale
	if exp < 0 || exp > bcMaxDigits || a.digits()+exp > bcMaxDigits {
		return bcNumber{}, bcErrTooLarge
	}
	root := new(big.Int).Sqrt(new(big.Int).Mul(a.unscaled, bcPow10(exp)))
	return bcNumber{root, target}, nil
}

// bcMod implements the POSIX remainder: a - (a/b)*b with the quotient at the
// scale setting, giving scale max(scale+b.scale, a.scale).
func bcMod(a, b bcNumber, scale int) (bcNumber, error) {
	if b.isZero() {
		return bcNumber{}, bcErrDivideByZero
	}
	q, err := bcDiv(a, b, scale)
	if err != nil {
		return bcNumber{}, err
	}
	mulScale := scale
	if mulScale != 0 {
		mulScale = scale + b.scale
		if a.scale > mulScale {
			mulScale = a.scale
		}
	}
	prod, err := bcMul(q, b, mulScale)
	if err != nil {
		return bcNumber{}, err
	}
	rem, err := bcSub(a, prod)
	if err != nil {
		return bcNumber{}, err
	}
	target := scale + b.scale
	if a.scale > target {
		target = a.scale
	}
	return bcCheckedRescale(rem, target)
}

// bcCompare orders two values numerically; -0 and 0 compare equal.
func bcCompare(a, b bcNumber) int {
	aa, bb, err := bcAlign(a, b)
	if err != nil {
		return a.unscaled.Cmp(b.unscaled)
	}
	return aa.unscaled.Cmp(bb.unscaled)
}

// bcLength is the POSIX length() of a value: its significant decimal digits,
// counted the way Toybox does (a stored trailing zero still counts, zero is 0).
func (n bcNumber) bcLength() bcNumber {
	return bcNumInt(int64(n.digits()))
}

// bcScaleOf is the POSIX scale() of a value.
func (n bcNumber) bcScaleOf() bcNumber {
	return bcNumInt(int64(n.scale))
}

// decimalString renders the value in base 10 with all stored fraction digits
// and no leading zero before the point, which is what bc prints.
func (n bcNumber) decimalString() string {
	if n.isZero() {
		return "0"
	}
	digits := new(big.Int).Abs(n.unscaled).String()
	if n.scale > 0 {
		for len(digits) < n.scale {
			digits = "0" + digits
		}
		if n.scale < len(digits) {
			digits = digits[:len(digits)-n.scale] + "." + digits[len(digits)-n.scale:]
		} else {
			digits = "." + digits
		}
	}
	if n.isNeg() {
		return "-" + digits
	}
	return digits
}

// bcDigitValue is the numeric value of one input digit. Only letters clamp to
// base-1 when they are too large for the base; 0-9 keep their value even above
// ibase, as in Toybox (bc_num_parseChar) and POSIX's "single digit" rule.
func bcDigitValue(c byte, base int) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'Z':
		v := int(c-'A') + 10
		if v >= base {
			return base - 1
		}
		return v
	}
	return -1
}

// bcParseNumber converts a numeric literal using the current ibase. A
// one-character literal is always the value of that digit, whatever ibase says.
func bcParseNumber(lit string, ibase int) (bcNumber, error) {
	if lit == "" {
		return bcNumber{}, errors.New("bad expression")
	}
	if len(lit) == 1 {
		v := bcDigitValue(lit[0], 36)
		if v < 0 {
			return bcNumber{}, errors.New("bad character")
		}
		return bcNumInt(int64(v)), nil
	}
	intPart, fracPart, hasPoint := strings.Cut(lit, ".")
	total, err := bcParseDigits(intPart, ibase)
	if err != nil {
		return bcNumber{}, err
	}
	if !hasPoint {
		return total, nil
	}
	frac, err := bcParseDigits(fracPart, ibase)
	if err != nil {
		return bcNumber{}, err
	}
	scale := len(fracPart)
	if scale == 0 {
		return total, nil
	}
	if scale > bcMaxScale {
		return bcNumber{}, bcErrTooLarge
	}
	den := new(big.Int).Exp(big.NewInt(int64(ibase)), big.NewInt(int64(scale)), nil)
	if bcNumDigitCount(den) > bcMaxDigits {
		return bcNumber{}, bcErrTooLarge
	}
	// The fractional value is digits/base^scale; store it as a decimal with
	// exactly `scale` places, truncating (Toybox: bc_num_div(..., digits)).
	scaled := new(big.Int).Mul(frac.unscaled, bcPow10(scale))
	frac = bcNumber{new(big.Int).Quo(scaled, den), scale}
	sum, err := bcAdd(total, frac)
	if err != nil {
		return bcNumber{}, err
	}
	sum, err = bcCheckedRescale(sum, scale)
	if err != nil {
		return bcNumber{}, err
	}
	if ibase != 10 && sum.isZero() {
		// Toybox's non-decimal parser returns a bare zero when the result is
		// zero, so scale(.0001) is 0 with ibase=16.
		return bcNumZero(), nil
	}
	return sum, nil
}

// bcParseDigits converts a run of digits in the given base. Digits that are
// too large for the base clamp to base-1, like Toybox's bc_num_parseChar; the
// fast path is only used when every digit is valid, so big literals parse in
// one SetString call.
func bcParseDigits(digits string, ibase int) (bcNumber, error) {
	if digits == "" {
		return bcNumZero(), nil
	}
	if len(digits) > bcMaxDigits {
		return bcNumber{}, bcErrTooLarge
	}
	valid := true
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		var raw int
		switch {
		case c >= '0' && c <= '9':
			raw = int(c - '0')
		case c >= 'A' && c <= 'Z':
			raw = int(c-'A') + 10
		default:
			return bcNumber{}, errors.New("bad character")
		}
		if raw >= ibase {
			valid = false
		}
	}
	if valid {
		v, ok := new(big.Int).SetString(digits, ibase)
		if ok {
			if bcNumDigitCount(v) > bcMaxDigits {
				return bcNumber{}, bcErrTooLarge
			}
			return bcNumber{v, 0}, nil
		}
	}
	base := big.NewInt(int64(ibase))
	total := bcNumZero()
	for i := 0; i < len(digits); i++ {
		v := bcDigitValue(digits[i], ibase)
		if v < 0 {
			return bcNumber{}, errors.New("bad character")
		}
		total.unscaled.Mul(total.unscaled, base)
		total.unscaled.Add(total.unscaled, big.NewInt(int64(v)))
		if total.digits() > bcMaxDigits {
			return bcNumber{}, bcErrTooLarge
		}
	}
	return total, nil
}

// intValue truncates toward zero and returns the integer part, for scale,
// ibase and obase assignment and for ^ exponents.
func (n bcNumber) intValue() (int64, error) {
	if n.isNeg() {
		return 0, bcErrNegative
	}
	v := n.unscaled
	if n.scale > 0 {
		v = new(big.Int).Quo(v, bcPow10(n.scale))
	}
	if !v.IsInt64() {
		return 0, bcErrOverflow
	}
	return v.Int64(), nil
}
