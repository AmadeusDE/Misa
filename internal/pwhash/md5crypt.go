package pwhash

import (
	"crypto/md5"
	"strings"
)

// md5Magic is the md5-crypt setting prefix.
const md5Magic = "$1$"

// md5Order is the md5-crypt encoding order: four 3-byte groups emitting four
// characters each, one more 3-byte group, then two characters from byte 11.
var md5Order = []encGroup{
	{0, 6, 12, 4}, {1, 7, 13, 4}, {2, 8, 14, 4}, {3, 9, 15, 4}, {4, 10, 5, 4},
	{lastGroup, lastGroup, 11, 2},
}

// md5crypt hashes password with the md5-crypt scheme. saltText is the part of
// the setting after the "$1$" prefix: the salt ends at the next '$' (a full
// previous hash may be used as a setting) or the end of the text, and is
// truncated to 8 bytes.
func md5crypt(password, saltText string) string {
	salt := saltText
	if i := strings.IndexByte(salt, '$'); i >= 0 {
		salt = salt[:i]
	}
	if len(salt) > md5SaltLen {
		salt = salt[:md5SaltLen]
	}
	pw, saltB := []byte(password), []byte(salt)

	// 1. alt = MD5(P || S || P).
	alt := md5.Sum(append(append(append([]byte{}, pw...), saltB...), pw...))

	// 2. MD5(P || "$1$" || S || alt repeated to len(P) || one byte per bit
	// of len(P): NUL for a set bit, else P[0]).
	h := md5.New()
	h.Write(pw)
	h.Write([]byte(md5Magic))
	h.Write(saltB)
	for pl := len(pw); pl > 0; pl -= 16 {
		h.Write(alt[:min(pl, 16)])
	}
	for i := len(pw); i > 0; i >>= 1 {
		if i&1 != 0 {
			h.Write([]byte{0})
		} else {
			h.Write(pw[:1])
		}
	}
	final := h.Sum(nil)

	// 3. 1000 strengthening rounds.
	for i := 0; i < 1000; i++ {
		h.Reset()
		if i&1 != 0 {
			h.Write(pw)
		} else {
			h.Write(final)
		}
		if i%3 != 0 {
			h.Write(saltB)
		}
		if i%7 != 0 {
			h.Write(pw)
		}
		if i&1 != 0 {
			h.Write(final)
		} else {
			h.Write(pw)
		}
		final = h.Sum(nil)
	}

	// 3. Encode the final digest.
	var b strings.Builder
	b.WriteString(md5Magic)
	b.WriteString(salt)
	b.WriteByte('$')
	b.WriteString(encodeCrypt(final, md5Order, cryptAlphabet))
	return b.String()
}
