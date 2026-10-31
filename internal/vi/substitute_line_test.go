package vi

import (
	"testing"

	"github.com/AmadeusDE/Misa/internal/vi/vipat"
)

func TestDropLastRune(t *testing.T) {
	if got := dropLastRune("s/é"); got != "s/" {
		t.Errorf("dropLastRune = %q", got)
	}
	if got := dropLastRune(""); got != "" {
		t.Errorf("dropLastRune(\"\") = %q", got)
	}
}

func TestCaseInsensitiveReplaceKeepsUTF8(t *testing.T) {
	for _, c := range []struct {
		line, pat, want string
		fold            bool
		n               int
	}{
		{"İİ Foo foo", "foo", "İİ bar bar", true, 2},
		{"ab", "x*", "-a-b", false, 2},
		{"éé", "x*", "-é-é", false, 2},
		{"abc", "b*", "-a-c", false, 2},
		{"abc", "c*", "-a-b-", false, 3},
		{"aXa", "x", "abca", true, 1},
	} {
		re, err := vipat.Compile(c.pat, c.fold)
		if err != nil {
			t.Fatal(err)
		}
		repl := "bar"
		if c.pat != "foo" {
			repl = "-"
		}
		if c.line == "aXa" {
			repl = "bc"
		}
		got, n, _, err := substituteLine(re, c.line, repl, true)
		if err != nil || got != c.want || n != c.n {
			t.Errorf("substituteLine(%q, %q) = %q, %d, %v want %q, %d", c.line, c.pat, got, n, err, c.want, c.n)
		}
	}
}
