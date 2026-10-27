package native

import "testing"

func TestTsort_Basic(t *testing.T) {
	r := runStdin(t, "tsort", "a b\nb c\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\nb\nc\n")
}

func TestTsort_Diamond(t *testing.T) {
	r := runStdin(t, "tsort", "a b\na c\nb d\nc d\n")
	wantSuccess(t, r)
	wantOutContains(t, r, "a")
	wantOutContains(t, r, "d")
}

func TestTsort_Single(t *testing.T) {
	r := runStdin(t, "tsort", "a a\n")
	wantSuccess(t, r)
	wantOut(t, r, "a\n")
}

func TestTsort_Empty(t *testing.T) {
	r := runStdin(t, "tsort", "")
	wantSuccess(t, r)
	wantOut(t, r, "")
}

func TestTsort_Cycle(t *testing.T) {
	r := runStdin(t, "tsort", "a b\nb a\n")
	wantFail(t, r)
}

func TestTsort_Version(t *testing.T) {
	r := run(t, "tsort", "-V")
	wantSuccess(t, r)
}
