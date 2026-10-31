package posixre

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// randomERE builds patterns from constructs both engines support, so the
// backtracking matcher can be checked against Go's regexp with Longest.
func randomERE(r *rand.Rand, depth int) string {
	if depth <= 0 {
		return []string{"a", "b", ".", "[ab]", "[^a]", "^", "$", `\b`}[r.IntN(8)]
	}
	switch r.IntN(7) {
	case 0:
		return randomERE(r, depth-1) + randomERE(r, depth-1)
	case 1:
		return randomERE(r, depth-1) + "|" + randomERE(r, depth-1)
	case 2:
		return "(" + randomERE(r, depth-1) + ")"
	case 3:
		return "(" + randomERE(r, depth-1) + ")" + []string{"*", "+", "?", "{2}", "{1,3}", "{0,2}", "{2,}"}[r.IntN(7)]
	case 4:
		return randomERE(r, depth-1) + "*"
	}
	return randomERE(r, 0)
}

func randomInput(r *rand.Rand) string {
	var b strings.Builder
	for n := r.IntN(8); n > 0; n-- {
		b.WriteByte("ab \n"[r.IntN(4)])
	}
	return b.String()
}

func TestEnginesAgree(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 20000; i++ {
		pattern := randomERE(r, 1+r.IntN(4))
		opt := Options{Syntax: Extended, GNU: true, IgnoreCase: r.IntN(4) == 0}
		re, err := Compile(pattern, opt)
		if err != nil || re.fast == nil {
			continue
		}
		input := []byte(randomInput(r))
		pos := 0
		if re.fastAnywhere && len(input) > 0 {
			pos = r.IntN(len(input) + 1)
		}
		want, _ := re.Find(input, pos)
		got, err := re.newMachine(input, pos).find(pos, re.prefix)
		if err != nil {
			t.Fatalf("%q on %q: %v", pattern, input, err)
		}
		if !reflect.DeepEqual(got, want) && !(got == nil && want == nil) {
			t.Fatalf("pattern %q input %q pos %d: engine %v, regexp %v", pattern, input, pos, got, want)
		}
	}
}

func FuzzEnginesAgree(f *testing.F) {
	f.Add("(a|ab)(c|bcd)(d*)", "abcd")
	f.Add("(a*)+$", "aab")
	f.Add(`\b(a|b)*`, "ab ba")
	f.Fuzz(func(t *testing.T, pattern, input string) {
		re, err := Compile(pattern, Options{Syntax: Extended, GNU: true})
		if err != nil || re.fast == nil || len(input) > 64 {
			return
		}
		want, _ := re.Find([]byte(input), 0)
		got, err := re.newMachine([]byte(input), 0).find(0, re.prefix)
		if err == nil && !reflect.DeepEqual(got, want) && !(got == nil && want == nil) {
			t.Fatalf("pattern %q input %q: engine %v, regexp %v", pattern, input, got, want)
		}
	})
}
