// Package pwhash implements the crypt(3) password hash formats Misa supports,
// in pure Go: MD5-crypt ($1$), SHA-256-crypt ($5$) and SHA-512-crypt ($6$).
//
// Written from the published algorithm descriptions: Ulrich Drepper's
// "Unix crypt using SHA-256 and SHA-512" (public domain) and the publicly
// documented md5-crypt scheme. No code was taken from glibc or any other
// implementation. The mkpasswd command-line conventions these formats are
// used from follow toybox toys/other/mkpasswd.c, commit
// 6a0c0ef5eeb2c43e4feef9781398c1718b8bddcb (0BSD).
package pwhash

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// Canonical method names, as used by NormalizeMethod, CryptMethod and the
// mkpasswd builtin's -m flag.
const (
	MethodMD5    = "md5"
	MethodSHA256 = "sha256"
	MethodSHA512 = "sha512"
)

var (
	// ErrUnsupportedMethod is returned for an unknown method name or an
	// unknown "$id$" prefix in a crypt(3) setting.
	ErrUnsupportedMethod = errors.New("unsupported hash method")
	// ErrBadRounds is returned when a "$5$"/"$6$" setting carries a
	// "rounds=" prefix that is not followed by decimal digits.
	ErrBadRounds = errors.New("bad rounds value")
)

// cryptAlphabet is the crypt base64 alphabet: index 0 is '.', 1 is '/', 2 is
// '0' and 63 is 'z'.
const cryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Salt lengths, in bytes, before truncation.
const (
	md5SaltLen  = 8
	sha2SaltLen = 16
)

// NormalizeMethod maps "md5", "sha256"/"sha-256", "sha512"/"sha-512"
// (case-insensitive) to a canonical method name; ok is false otherwise.
func NormalizeMethod(name string) (method string, ok bool) {
	switch strings.ToLower(name) {
	case "md5":
		return MethodMD5, true
	case "sha256", "sha-256":
		return MethodSHA256, true
	case "sha512", "sha-512":
		return MethodSHA512, true
	}
	return "", false
}

// MaxSaltLen returns the maximum salt length in bytes: 8 for md5, 16 for the
// SHA methods. It returns 0 for an unknown method.
func MaxSaltLen(method string) int {
	switch method {
	case MethodMD5:
		return md5SaltLen
	case MethodSHA256, MethodSHA512:
		return sha2SaltLen
	}
	return 0
}

// ValidateSalt checks raw salt text (for the SHA methods it may carry a
// leading "rounds=N$" prefix) for the crypt charset [./0-9A-Za-z], the length
// limits and non-emptiness. It returns a descriptive error.
//
// A rounds value outside 1000..999999999 is accepted: Crypt clamps it.
func ValidateSalt(method, salt string) error {
	method, ok := NormalizeMethod(method)
	if !ok {
		return ErrUnsupportedMethod
	}
	if method != MethodMD5 {
		var err error
		if salt, err = trimRounds(salt); err != nil {
			return err
		}
	}
	if salt == "" {
		return fmt.Errorf("pwhash: empty salt for %s", method)
	}
	if max := MaxSaltLen(method); len(salt) > max {
		return fmt.Errorf("pwhash: salt %q is %d bytes, %s allows at most %d", salt, len(salt), method, max)
	}
	for i := 0; i < len(salt); i++ {
		if !isCryptChar(salt[i]) {
			return fmt.Errorf("pwhash: salt %q contains %q, only ./0-9A-Za-z are allowed", salt, salt[i])
		}
	}
	return nil
}

// RandomSalt returns a random salt of MaxSaltLen(method) characters from the
// crypt alphabet, read from r (crypto/rand.Reader in production).
func RandomSalt(method string, r io.Reader) (string, error) {
	n := MaxSaltLen(method)
	if n == 0 {
		return "", ErrUnsupportedMethod
	}
	if r == nil {
		return "", fmt.Errorf("pwhash: nil reader for random salt")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("pwhash: random salt: %w", err)
	}
	// 256 is a multiple of the alphabet size, so the modulo below is uniform.
	for i, b := range buf {
		buf[i] = cryptAlphabet[int(b)%len(cryptAlphabet)]
	}
	return string(buf), nil
}

// Crypt hashes password using a full crypt(3) setting such as "$1$abcdefgh",
// "$5$rounds=10000$saltstring" or "$6$saltstring". Unknown "$id$" prefixes
// return ErrUnsupportedMethod.
func Crypt(password, setting string) (string, error) {
	if !strings.HasPrefix(setting, "$") {
		return "", ErrUnsupportedMethod
	}
	id, rest, ok := strings.Cut(setting[1:], "$")
	if !ok {
		return "", ErrUnsupportedMethod
	}
	switch id {
	case "1":
		return md5crypt(password, rest), nil
	case "5":
		return sha2crypt(sha256Kind, password, rest)
	case "6":
		return sha2crypt(sha512Kind, password, rest)
	}
	return "", ErrUnsupportedMethod
}

// CryptMethod hashes password with the named method and its raw salt text.
// For the SHA methods the salt may start with a "rounds=N$" prefix.
func CryptMethod(method, password, salt string) (string, error) {
	method, ok := NormalizeMethod(method)
	if !ok {
		return "", ErrUnsupportedMethod
	}
	switch method {
	case MethodMD5:
		return md5crypt(password, salt), nil
	case MethodSHA256:
		return sha2crypt(sha256Kind, password, salt)
	}
	return sha2crypt(sha512Kind, password, salt)
}

// MD5Crypt hashes password with an md5-crypt setting, which must start with
// "$1$". The salt is the setting text up to the next '$' (or its end),
// truncated to 8 bytes.
func MD5Crypt(password, setting string) (string, error) {
	return cryptWith(md5Magic, password, setting, func() (string, error) {
		return md5crypt(password, setting[len(md5Magic):]), nil
	})
}

// SHA256Crypt hashes password with a SHA-256-crypt setting, which must start
// with "$5$".
func SHA256Crypt(password, setting string) (string, error) {
	return cryptWith(sha256Kind.prefix, password, setting, func() (string, error) {
		return sha2crypt(sha256Kind, password, setting[len(sha256Kind.prefix):])
	})
}

// SHA512Crypt hashes password with a SHA-512-crypt setting, which must start
// with "$6$".
func SHA512Crypt(password, setting string) (string, error) {
	return cryptWith(sha512Kind.prefix, password, setting, func() (string, error) {
		return sha2crypt(sha512Kind, password, setting[len(sha512Kind.prefix):])
	})
}

// cryptWith rejects a setting that does not carry the expected id prefix and
// otherwise runs crypt on the setting body.
func cryptWith(prefix, password, setting string, crypt func() (string, error)) (string, error) {
	if !strings.HasPrefix(setting, prefix) {
		return "", fmt.Errorf("pwhash: setting %q does not start with %q", setting, prefix)
	}
	return crypt()
}

// encGroup encodes three bytes of a digest as n crypt base64 characters.
// A negative index means the constant zero byte rather than a digest byte: the
// final groups of both formats encode one digest byte and pad with zeros.
type encGroup struct {
	b2, b1, b0 int // digest indexes, most significant byte first
	n          int // characters written
}

// at returns digest[i], or zero when i is negative.
func at(digest []byte, i int) byte {
	if i < 0 {
		return 0
	}
	return digest[i]
}

// encodeCrypt encodes digest with the published byte-permutation order: each
// group forms the 24-bit word b2<<16 | b1<<8 | b0 that is emitted least
// significant six bits first.
func encodeCrypt(digest []byte, order []encGroup, alphabet string) string {
	var b strings.Builder
	b.Grow(2 * len(digest))
	for _, g := range order {
		w := uint32(at(digest, g.b2))<<16 | uint32(at(digest, g.b1))<<8 | uint32(at(digest, g.b0))
		for i := 0; i < g.n; i++ {
			b.WriteByte(alphabet[w&0x3f])
			w >>= 6
		}
	}
	return b.String()
}

// repeatTo repeats digest until it is n bytes long (truncating the last copy).
func repeatTo(digest []byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = digest[i%len(digest)]
	}
	return out
}

// isCryptChar reports whether c belongs to the crypt salt charset.
func isCryptChar(c byte) bool {
	switch {
	case c == '.' || c == '/':
		return true
	case c >= '0' && c <= '9':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	}
	return false
}
