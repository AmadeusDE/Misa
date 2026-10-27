package webui

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpPeriod = 30
	totpDigits = 6
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a fresh base32 secret and its otpauth:// URI.
func NewTOTPSecret(issuer, account string) (secret, uri string, err error) {
	raw := make([]byte, 20)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	secret = b32.EncodeToString(raw)
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"},
		"digits": {fmt.Sprint(totpDigits)}, "period": {fmt.Sprint(totpPeriod)}}
	return secret, "otpauth://totp/" + label + "?" + q.Encode(), nil
}

func totpCounter(t time.Time) int64 { return t.Unix() / totpPeriod }

// TOTPCode is the RFC 6238 code (SHA-1, 6 digits) for the time step containing t.
func TOTPCode(secret string, t time.Time) (string, error) {
	return hotp(secret, totpCounter(t))
}

func hotp(secret string, counter int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0xf
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, n%1000000), nil
}

// matchTOTP accepts the current step and one either side, but only a step
// newer than last, so a code can be used once.
func matchTOTP(secret, code string, last int64, now time.Time) (int64, bool) {
	code = strings.ReplaceAll(code, " ", "")
	cur := totpCounter(now)
	for c := cur - 1; c <= cur+1; c++ {
		want, err := hotp(secret, c)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 && c > last {
			return c, true
		}
	}
	return 0, false
}
