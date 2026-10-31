package uutils

import "testing"

func TestUnknownSetupIsNotApproximated(t *testing.T) {
	var cases []testCase
	collectCases(`let (at, mut ucmd) = at_and_ucmd!();
at.unrecognized_setup("file");
ucmd.args(&["file"]).succeeds();`, 1, nil, "test_unknown", &cases)
	if len(cases) != 1 || cases[0].skipReason == "" {
		t.Fatalf("unknown setup must prevent execution: %+v", cases)
	}
}
