package uutils

import "testing"

func TestFunctionDiscoveryGaps(t *testing.T) {
	src := `fn test_empty() { complicated(); }
fn test_parsed() { new_ucmd!().succeeds(); }
fn test_unsupported() { new_ucmd!().custom(); }`
	gaps := functionGaps(src, []testCase{{name: "test_parsed", line: 2}})
	if len(gaps) != 2 || gaps[0].name != "test_empty" || gaps[1].name != "test_unsupported" {
		t.Fatalf("gaps: %+v", gaps)
	}
}

func TestNativeReferencesSpecific(t *testing.T) {
	if nativeForGap("id", functionGap{name: "test_loop", body: "for x in y {}"}) != "" {
		t.Fatal("unrelated native reference")
	}
	if nativeForGap("chmod", functionGap{name: "test_mode", body: "for x in y {}"}) == "" {
		t.Fatal("missing chmod loop reference")
	}
	if nativeForGap("cp", functionGap{name: "test_symlink", body: ""}) == "" {
		t.Fatal("missing symlink reference")
	}
}
