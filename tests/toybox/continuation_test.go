package toybox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContinuationInsideQuotedString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexdump.test")
	text := "testcmd \\\n  \"case\" \\\n  \"-C input\" \\\n  \"\\\nfirst\\n\\\nsecond\\n\" \\\n  \"\\\nabc\\\ndef\" \\\n  \"\"\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	suite, err := parseTestFile(path)
	if err != nil || len(suite.cases) != 1 {
		t.Fatalf("parse: %v, cases: %+v", err, suite.cases)
	}
	tc := suite.cases[0]
	if tc.expected != "first\nsecond\n" || tc.infile != "abcdef" {
		t.Fatalf("expected %q input %q", tc.expected, tc.infile)
	}
}
