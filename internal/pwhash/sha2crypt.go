package pwhash

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"strconv"
	"strings"
)

// SHA-crypt parameters from the published description.
const (
	defaultRounds = 5000      // rounds when the setting carries no prefix
	minRounds     = 1000      // values below are raised to this
	maxRounds     = 999999999 // values above are lowered to this
	roundsPrefix  = "rounds="
)

// sha2Kind parameterises the SHA-crypt algorithm over SHA-256 and SHA-512.
type sha2Kind struct {
	newHash func() hash.Hash // digest constructor
	prefix  string           // setting id prefix, "$5$" or "$6$"
	order   []encGroup       // published digest encoding order
}

var (
	sha256Kind = &sha2Kind{newHash: sha256.New, prefix: "$5$", order: sha256Order}
	sha512Kind = &sha2Kind{newHash: sha512.New, prefix: "$6$", order: sha512Order}
)

// lastGroup marks a group byte that is the constant zero rather than a digest
// index, as in the final SHA-512-crypt group, which encodes byte 63 alone.
const lastGroup = -1

// sha256Order is the SHA-256-crypt encoding order: ten 3-byte groups emitting
// four characters each, then a final 3-character group.
var sha256Order = []encGroup{
	{0, 10, 20, 4}, {21, 1, 11, 4}, {12, 22, 2, 4}, {3, 13, 23, 4},
	{24, 4, 14, 4}, {15, 25, 5, 4}, {6, 16, 26, 4}, {27, 7, 17, 4},
	{18, 28, 8, 4}, {9, 19, 29, 4}, {lastGroup, 31, 30, 3},
}

// sha512Order is the SHA-512-crypt encoding order: twenty-one 3-byte groups
// emitting four characters each, then a final 2-character group.
var sha512Order = []encGroup{
	{0, 21, 42, 4}, {22, 43, 1, 4}, {44, 2, 23, 4}, {3, 24, 45, 4},
	{25, 46, 4, 4}, {47, 5, 26, 4}, {6, 27, 48, 4}, {28, 49, 7, 4},
	{50, 8, 29, 4}, {9, 30, 51, 4}, {31, 52, 10, 4}, {53, 11, 32, 4},
	{12, 33, 54, 4}, {34, 55, 13, 4}, {56, 14, 35, 4}, {15, 36, 57, 4},
	{37, 58, 16, 4}, {59, 17, 38, 4}, {18, 39, 60, 4}, {40, 61, 19, 4},
	{62, 20, 41, 4}, {lastGroup, lastGroup, 63, 2},
}

// sha2crypt hashes password with the SHA-crypt algorithm of the published
// description. saltText is the part of the setting after the "$5$"/"$6$" id
// prefix: an optional "rounds=N$" prefix followed by the salt, which ends at
// the next '$' (a full previous hash may be used as a setting) or the end of
// the text and is truncated to 16 bytes.
func sha2crypt(k *sha2Kind, password, saltText string) (string, error) {
	rounds, haveRounds, rest, err := splitRounds(saltText)
	if err != nil {
		return "", err
	}
	salt := rest
	if i := strings.IndexByte(salt, '$'); i >= 0 {
		salt = salt[:i]
	}
	if len(salt) > sha2SaltLen {
		salt = salt[:sha2SaltLen]
	}
	pw, saltB := []byte(password), []byte(salt)

	// 1. digestB = H(P || S || P).
	h := k.newHash()
	h.Write(pw)
	h.Write(saltB)
	h.Write(pw)
	digestB := h.Sum(nil)

	// 2. digestA = H(P || S || digestB repeated to len(P) || <digestB or P per bit of len(P)>).
	h.Reset()
	h.Write(pw)
	h.Write(saltB)
	// One copy of digestB per full digest-sized block of P, then the
	// remaining len(P) mod size bytes of it.
	cnt := len(pw)
	for ; cnt > len(digestB); cnt -= len(digestB) {
		h.Write(digestB)
	}
	h.Write(digestB[:cnt])
	for cnt := len(pw); cnt > 0; cnt >>= 1 {
		if cnt&1 != 0 {
			h.Write(digestB)
		} else {
			h.Write(pw)
		}
	}
	digestA := h.Sum(nil)

	// 3. p_bytes = H(P repeated len(P) times), repeated to len(P) bytes.
	h.Reset()
	for i := 0; i < len(pw); i++ {
		h.Write(pw)
	}
	pBytes := repeatTo(h.Sum(nil), len(pw))

	// 4. s_bytes = H(S repeated 16+digestA[0] times), repeated to len(S) bytes.
	h.Reset()
	for i := 0; i < sha2SaltLen+int(digestA[0]); i++ {
		h.Write(saltB)
	}
	sBytes := repeatTo(h.Sum(nil), len(saltB))

	// 5. The rounds loop.
	c := digestA
	for i := 0; i < rounds; i++ {
		h.Reset()
		if i&1 != 0 {
			h.Write(pBytes)
		} else {
			h.Write(c)
		}
		if i%3 != 0 {
			h.Write(sBytes)
		}
		if i%7 != 0 {
			h.Write(pBytes)
		}
		if i&1 != 0 {
			h.Write(c)
		} else {
			h.Write(pBytes)
		}
		c = h.Sum(nil)
	}

	// 6.+7. Encode and echo the setting's own prefix with the clamped rounds.
	var b strings.Builder
	b.WriteString(k.prefix)
	if haveRounds {
		b.WriteString(roundsPrefix)
		b.WriteString(strconv.Itoa(rounds))
		b.WriteByte('$')
	}
	b.WriteString(salt)
	b.WriteByte('$')
	b.WriteString(encodeCrypt(c, k.order, cryptAlphabet))
	return b.String(), nil
}

// splitRounds splits an optional "rounds=N$" prefix off a SHA-crypt setting
// body. It reports whether the prefix was present; when absent the default
// rounds apply. Values are clamped to 1000..999999999.
func splitRounds(saltText string) (rounds int, haveRounds bool, rest string, err error) {
	if !strings.HasPrefix(saltText, roundsPrefix) {
		return defaultRounds, false, saltText, nil
	}
	digits, rest, ok := strings.Cut(saltText[len(roundsPrefix):], "$")
	if !ok {
		return 0, false, "", fmt.Errorf("pwhash: %q prefix is not followed by a $: %w", roundsPrefix, ErrBadRounds)
	}
	rounds, err = parseRounds(digits)
	if err != nil {
		return 0, false, "", err
	}
	return rounds, true, rest, nil
}

// trimRounds removes an optional "rounds=N$" prefix, returning the salt. The
// rounds value is validated but not otherwise used.
func trimRounds(salt string) (string, error) {
	_, _, rest, err := splitRounds(salt)
	if err != nil {
		return "", err
	}
	return rest, nil
}

// parseRounds parses the decimal rounds value of a "rounds=N$" prefix and
// clamps it to 1000..999999999. It returns ErrBadRounds when digits is not one
// or more decimal digits.
func parseRounds(digits string) (int, error) {
	if digits == "" {
		return 0, fmt.Errorf("pwhash: empty rounds value: %w", ErrBadRounds)
	}
	n := 0
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("pwhash: rounds value %q is not a decimal number: %w", digits, ErrBadRounds)
		}
		// Stop accumulating once past the maximum: the extra digits only
		// prove the value is large, and this keeps n from overflowing.
		if n <= maxRounds {
			n = n*10 + int(c-'0')
		}
	}
	if n < minRounds {
		n = minRounds
	}
	if n > maxRounds {
		n = maxRounds
	}
	return n, nil
}
