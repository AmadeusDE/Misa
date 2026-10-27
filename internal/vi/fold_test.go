package vi

import "testing"

func TestFoldIndexReturnsSpansInTheOriginal(t *testing.T) {
	cases := []struct {
		s, sub     string
		start, end int
	}{
		{"Hello World", "world", 6, 11},
		{"İİ needle", "NEEDLE", 5, 11}, // İ lowers to two runes in strings.ToLower
		{"ẞtraße", "SS", -1, -1},
		{"KELVIN", "Kelvin", 0, 6},
		{"abc", "", 0, 0},
		{"abc", "x", -1, -1},
	}
	for _, c := range cases {
		if s, e := foldIndex(c.s, c.sub); s != c.start || e != c.end {
			t.Errorf("foldIndex(%q, %q) = %d,%d want %d,%d", c.s, c.sub, s, e, c.start, c.end)
		}
	}
	if s, e := foldLastIndex("aXa xA", "xa"); s != 4 || e != 6 {
		t.Errorf("foldLastIndex = %d,%d", s, e)
	}
}

func TestDropLastRune(t *testing.T) {
	if got := dropLastRune("s/é"); got != "s/" {
		t.Errorf("dropLastRune = %q", got)
	}
	if got := dropLastRune(""); got != "" {
		t.Errorf("dropLastRune(\"\") = %q", got)
	}
}

func TestCaseInsensitiveReplaceKeepsUTF8(t *testing.T) {
	if got, n := replaceAll("İİ Foo foo", "foo", "bar", true); got != "İİ bar bar" || n != 2 {
		t.Errorf("replaceAll = %q, %d", got, n)
	}
	if got, n := replaceAll("ab", "", "-", false); got != "-a-b" || n != 2 {
		t.Errorf("empty pattern = %q, %d", got, n)
	}
	if got, n := replaceAll("éé", "", "-", false); got != "-é-é" || n != 2 {
		t.Errorf("empty pattern over UTF-8 = %q, %d", got, n)
	}
}
