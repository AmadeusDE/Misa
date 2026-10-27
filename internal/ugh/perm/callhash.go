package perm

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"strconv"
)

// CanonicalArgs rewrites tool-argument JSON into one canonical form: object
// keys sorted, no insignificant whitespace, numbers in their shortest form.
// Empty input canonicalizes as null.
func CanonicalArgs(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte("null"), nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, errTrailingJSON
	}
	var buf bytes.Buffer
	writeCanonical(&buf, v)
	return buf.Bytes(), nil
}

type jsonError string

func (e jsonError) Error() string { return string(e) }

const errTrailingJSON = jsonError("trailing data after JSON arguments")

func writeCanonical(buf *bytes.Buffer, v any) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeString(buf, k)
			buf.WriteByte(':')
			writeCanonical(buf, x[k])
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonical(buf, e)
		}
		buf.WriteByte(']')
	case json.Number:
		buf.WriteString(canonicalNumber(x))
	case string:
		writeString(buf, x)
	default:
		b, _ := json.Marshal(x)
		buf.Write(b)
	}
}

func writeString(buf *bytes.Buffer, s string) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	buf.Write(bytes.TrimRight(b.Bytes(), "\n"))
}

func canonicalNumber(n json.Number) string {
	r, ok := new(big.Rat).SetString(n.String())
	if !ok {
		return n.String()
	}
	if r.IsInt() {
		return r.Num().String()
	}
	f, _ := r.Float64()
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// CallHash is the hex sha256 of tool + canonicalArgs: the identity of the
// exact call a human approved.
func CallHash(c Call) (string, error) {
	args, err := CanonicalArgs(c.Args)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(c.Tool), args...))
	return hex.EncodeToString(sum[:]), nil
}

// VerifyApproved reports whether c is exactly the call whose hash was stored
// at approval time. Run a deferred call only when this is true.
func VerifyApproved(storedHash string, c Call) bool {
	h, err := CallHash(c)
	if err != nil || storedHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(h), []byte(storedHash)) == 1
}
