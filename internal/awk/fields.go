package awk

import (
	"strings"
	"unicode/utf8"
)

// record holds $0 and its fields. Splitting and rebuilding are lazy: the
// fields are split with the FS in force when first needed, and $0 is
// rebuilt with OFS after a field changes.
type record struct {
	rec        string
	fields     []string
	haveFields bool
	haveRec    bool
}

func (in *interp) setRecord(s string) {
	in.r = record{rec: s, haveRec: true}
}

func (in *interp) ensureFields() {
	if in.r.haveFields {
		return
	}
	in.r.fields = in.splitFields(in.r.rec)
	in.r.haveFields = true
}

func (in *interp) ensureRecord() {
	if in.r.haveRec {
		return
	}
	in.r.rec = strings.Join(in.r.fields, in.ofs)
	in.r.haveRec = true
}

func (in *interp) nf() int {
	in.ensureFields()
	return len(in.r.fields)
}

func (in *interp) setNF(n int) {
	if n < 0 {
		runtimeErrorf("cannot set NF to a negative value")
	}
	in.ensureFields()
	f := in.r.fields
	switch {
	case n < len(f):
		f = f[:n]
	default:
		for len(f) < n {
			f = append(f, "")
		}
	}
	in.r.fields = f
	in.r.haveRec = false
}

func (in *interp) field(i int) Value {
	if i < 0 {
		runtimeErrorf("trying to access out of range field %d", i)
	}
	if i == 0 {
		in.ensureRecord()
		return input(in.r.rec)
	}
	in.ensureFields()
	if i > len(in.r.fields) {
		return Value{}
	}
	return input(in.r.fields[i-1])
}

func (in *interp) setField(i int, v Value) {
	if i < 0 {
		runtimeErrorf("trying to access out of range field %d", i)
	}
	s := v.text(in.convfmt)
	if i == 0 {
		in.setRecord(s)
		return
	}
	if i > maxFields {
		runtimeErrorf("field index %d is too large", i)
	}
	in.ensureFields()
	for len(in.r.fields) < i {
		in.r.fields = append(in.r.fields, "")
	}
	in.r.fields[i-1] = s
	in.r.haveRec = false
}

const maxFields = 10_000_000

func (in *interp) splitFields(rec string) []string {
	if rec == "" {
		return nil
	}
	switch {
	case in.fs == " ":
		return splitBlanks(rec)
	case in.fs == "":
		return in.splitChars(rec)
	case len(in.fs) == 1:
		return splitByte(rec, in.fs[0], in.rs == "")
	}
	return in.splitRegex(rec, in.fs)
}

func splitBlanks(s string) []string {
	var out []string
	i := 0
	for {
		for i < len(s) && isBlank(s[i]) {
			i++
		}
		if i >= len(s) {
			return out
		}
		j := i
		for j < len(s) && !isBlank(s[j]) {
			j++
		}
		out = append(out, s[i:j])
		i = j
	}
}

func splitByte(s string, sep byte, newlineToo bool) []string {
	if !newlineToo || sep == '\n' {
		return strings.Split(s, string(sep))
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep || s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func (in *interp) splitChars(s string) []string {
	out := make([]string, 0, len(s))
	for i := 0; i < len(s); {
		w := 1
		if !in.cfg.Bytes {
			_, w = utf8.DecodeRuneInString(s[i:])
		}
		out = append(out, s[i:i+w])
		i += w
	}
	return out
}

func (in *interp) splitRegex(s, sep string) []string {
	re := in.regex(sep)
	b := []byte(s)
	var out []string
	start := 0
	for pos := 0; pos <= len(b); {
		m, err := re.Find(b, pos)
		if err != nil {
			runtimeErrorf("regular expression error: %v", err)
		}
		if m == nil {
			break
		}
		if m[1] == m[0] {
			if m[0] >= len(b) {
				break
			}
			_, w := utf8.DecodeRune(b[m[0]:])
			pos = m[0] + w
			continue
		}
		out = append(out, s[start:m[0]])
		start, pos = m[1], m[1]
	}
	return append(out, s[start:])
}
