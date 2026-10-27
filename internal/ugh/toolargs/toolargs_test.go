package toolargs

import (
	"encoding/json"
	"testing"
)

func TestCheck(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, ` {"path":"a","limit":3} `, `{"a":{"path":1,"PATH":2}}`} {
		if err := Check(json.RawMessage(raw)); err != nil {
			t.Errorf("Check(%s) = %v, want ok", raw, err)
		}
	}
	for _, raw := range []string{``, `[]`, `"x"`, `{"path":"a"}{"path":"b"}`, `{"path":"a"} x`,
		`{"path":"a","path":"b"}`, `{"path":"ok","PATH":"/secret"}`, `{"path":"ok","path":"/secret"}`,
		`{"Kind":"a","Kind":"b"}`, `{"path":"a"`} {
		if err := Check(json.RawMessage(raw)); err == nil {
			t.Errorf("Check(%s) = nil, want an error", raw)
		}
	}
}

func TestStringAgreesWithDecode(t *testing.T) {
	for _, raw := range []string{`{"path":"a"}`, `{"PATH":"a"}`, `{"Path":"a","other":1}`} {
		var dst struct{ Path string }
		if err := Decode(json.RawMessage(raw), &dst); err != nil {
			t.Fatal(err)
		}
		if got := String(json.RawMessage(raw), "path"); got != dst.Path || got != "a" {
			t.Errorf("%s: String = %q, Decode = %q", raw, got, dst.Path)
		}
	}
	for _, raw := range []string{`{"path":"ok","PATH":"/secret"}`, `{"path":3}`, `{"path":"a"} {}`} {
		if got := String(json.RawMessage(raw), "path"); got != "" {
			t.Errorf("String(%s) = %q, want empty", raw, got)
		}
	}
}
