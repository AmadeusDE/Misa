package builtins

import "testing"

func TestSplitVolume(t *testing.T) {
	cases := []struct {
		in, vol, rest string
		windows       bool
	}{
		{`C:\Users\x`, "C:", "/Users/x", true},
		{`c:/a/b`, "c:", "/a/b", true},
		{`C:`, "C:", "", true},
		{`\\host\share\dir\f`, "//host/share", "/dir/f", true},
		{`\\?\C:\a`, "C:", "/a", true},
		{`\\?\UNC\host\share\a`, "//host/share", "/a", true},
		{`\??\D:\a`, "D:", "/a", true},
		{`\rooted\p`, "", "/rooted/p", true},
		{`rel\p`, "", "rel/p", true},
		{`C:\a`, "", `C:\a`, false},
		{`/a/b`, "", "/a/b", false},
	}
	for _, c := range cases {
		vol, rest := splitVolume(c.in, c.windows)
		if vol != c.vol || rest != c.rest {
			t.Errorf("splitVolume(%q,%v) = %q,%q; want %q,%q", c.in, c.windows, vol, rest, c.vol, c.rest)
		}
	}
}

func TestCanonRoot(t *testing.T) {
	if got := canonRoot("C:", true); got != `C:\` {
		t.Errorf("drive root = %q", got)
	}
	if got := canonRoot("//h/s", true); got != `\\h\s\` {
		t.Errorf("unc root = %q", got)
	}
	if got := canonRoot("", false); got != "/" {
		t.Errorf("unix root = %q", got)
	}
}

func TestPathSeparators(t *testing.T) {
	if pathSeparators(true) != `/\` || pathSeparators(false) != "/" {
		t.Fatal("separator sets")
	}
}
