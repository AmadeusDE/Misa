// Package toolargs parses tool-call arguments for both the permission gate
// and the tools, so the values a call is judged on are the values it runs with.
package toolargs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Check accepts one JSON object (or null) whose keys stay distinct under the
// case folding encoding/json uses to match struct fields. Duplicate or
// case-variant keys and trailing data are refused: a plain key lookup and a
// struct decode would read them differently.
func Check(raw json.RawMessage) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("arguments are not a JSON object: %v", err)
	}
	if tok == nil {
		return trailing(dec)
	}
	if tok != json.Delim('{') {
		return errors.New("arguments are not a JSON object")
	}
	seen := map[string]string{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key := tok.(string)
		if prev, dup := seen[fold(key)]; dup {
			return fmt.Errorf("duplicate argument %q (also %q)", key, prev)
		}
		seen[fold(key)] = key
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	return trailing(dec)
}

func trailing(dec *json.Decoder) error {
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing data after the arguments")
	}
	return nil
}

// fold mirrors encoding/json's field-name folding.
func fold(s string) string {
	return strings.Map(func(r rune) rune { return unicode.ToUpper(unicode.ToLower(r)) }, s)
}

// Decode checks raw and unmarshals it into dst.
func Decode(raw json.RawMessage, dst any) error {
	if err := Check(raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// String returns the top-level string field key, matched the way Decode
// matches struct fields. It is "" when raw fails Check or the field is
// missing or not a string.
func String(raw json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	if Decode(raw, &m) != nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		for k, kv := range m {
			if fold(k) == fold(key) {
				v = kv
			}
		}
	}
	var s string
	if json.Unmarshal(v, &s) != nil {
		return ""
	}
	return s
}
