package help

import "testing"

func TestFindManPageInRejectsPathTricks(t *testing.T) {
	for _, c := range []struct{ name, section string }{
		{"../etc/passwd", "1"}, {"ls", "../.."}, {"ls", "1/../../x"}, {"", "1"},
	} {
		if p, ok := FindManPageIn(c.name, c.section); ok {
			t.Errorf("FindManPageIn(%q, %q) = %q, want none", c.name, c.section, p)
		}
	}
}
