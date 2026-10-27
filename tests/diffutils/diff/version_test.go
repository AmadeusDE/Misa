package diff

import "testing"

func TestVersionLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"3.10", "3.12", true}, {"3.12", "3.12", false}, {"3.12.1", "3.12", false}, {"3.9", "3.10", true}, {"4.0", "3.12", false}} {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
