package help

import "testing"

func TestPageNamesStayInThePageDirectories(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "../../../etc/passwd", "a/b", "x*", "x?", "[a]", "a\x00b"} {
		if validPageName(bad) {
			t.Errorf("validPageName(%q) = true", bad)
		}
		if _, ok := FindManPage(bad); ok {
			t.Errorf("FindManPage(%q) found a page", bad)
		}
	}
	for _, good := range []string{"ls", "git-commit", "7z", "c++"} {
		if !validPageName(good) {
			t.Errorf("validPageName(%q) = false", good)
		}
	}
}
